package espresso

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpmiddleware "github.com/suryakencana007/espresso/v2/middleware/http"
	servicemiddleware "github.com/suryakencana007/espresso/v2/middleware/service"
)

// ============================================
// Error Status-Code Matrix (v2.2 task-04)
// ============================================
//
// This is the consolidated regression net for the v2.2 error-pipeline work:
// for EVERY way an error can originate (extractor, handler, service layer, panic,
// auth, rate-limit) it asserts BOTH the HTTP status code AND the canonical JSON
// envelope shape on a single Router. It locks the contracts established by
// task-01 (fail-fast), task-02 (service-layer error -> status mapping), and
// task-03 (unified envelope for panic/auth/rate-limit, previously text/plain).
//
// The previously scattered single-status assertions (TestWithLayersTyped_WithTimeout,
// the middleware/http error_envelope tests) remain; this table is the one place
// that asserts the combined post-fix behavior.

// matrixEnvelope is the canonical {"error":{...}} wire shape. Decoding the inner
// object into a raw-message map (errorMatrixKeys) lets a row distinguish an
// ABSENT "details" key (panic) from a PRESENT one (validation) — a plain struct
// with omitempty cannot make that distinction.
type matrixEnvelope struct {
	Error struct {
		Code      string         `json:"code"`
		Message   string         `json:"message"`
		Details   map[string]any `json:"details"`
		RequestID string         `json:"request_id"`
	} `json:"error"`
}

// validateAlwaysFails is a service-layer validator that returns structured
// FieldErrors, so the translated *Error carries details.errors (task-02).
type validateAlwaysFails struct{}

func (validateAlwaysFails) Validate(ctx context.Context, req *JSON[CreateUserReq]) error {
	return FieldErrors{{Field: "name", Message: "is required"}}
}

// alwaysFailService always returns a non-espresso error so the circuit breaker
// counts failures and trips to Open.
func alwaysFailService(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
	return JSON[UserRes]{}, errors.New("upstream down")
}

// slowService blocks until its context deadline so a short Timeout layer fires.
func slowService(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
	select {
	case <-time.After(100 * time.Millisecond):
		return JSON[UserRes]{Data: UserRes{Message: "done"}}, nil
	case <-ctx.Done():
		return JSON[UserRes]{}, ctx.Err()
	}
}

// buildErrorMatrixRouter wires one Router with the three HTTP-level middleware
// named in the task (recover, auth, rate-limit) plus request-id, and registers
// one route per error origin. Auth and rate-limit are HTTP middleware that
// reject every request, so they cannot be global Use() middleware (they would
// reject the other rows); instead they wrap only their own route's handler,
// while RequestID + Recover are global so every row gets a request_id and a
// recovered panic.
func buildErrorMatrixRouter(t *testing.T) *Router {
	t.Helper()

	r := Portafilter()
	r.Use(httpmiddleware.RequestIDMiddleware(), httpmiddleware.RecoverMiddleware())

	okHandler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "ok"}}, nil
	}

	// extract_error: non-espresso extractor failure -> writeExtractError -> 400.
	r.Post("/extract-bad", HandlerCtxReqErr(okHandler))

	// extract_espresso_error: extractor returns *espresso.Error (422 carried).
	r.Get("/extract-esp", HandlerCtxReqErr(
		func(ctx context.Context, req *espressoErrExtractor) (Text, error) {
			return Text{Body: "unreachable"}, nil
		}))

	// handler_error: handler returns ErrNotFound -> 404 carried.
	r.Get("/not-found", HandlerCtx(func(ctx context.Context) (Text, error) {
		return Text{}, ErrNotFound("widget not found")
	}))

	// handler_plain_error: handler returns a non-espresso error -> 500 INTERNAL.
	r.Get("/boom", HandlerCtx(func(ctx context.Context) (Text, error) {
		return Text{}, errors.New("kaboom")
	}))

	// service_validation: Validation layer (task-02) -> 400 VALIDATION_ERROR + details.
	r.Post("/validate", WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](
		okHandler,
		Validation[*JSON[CreateUserReq]](validateAlwaysFails{}),
	))

	// service_circuit_breaker_open: pre-trip the breaker so the matrix request
	// is rejected with *servicemiddleware.CircuitBreakerError -> 503 (task-02).
	cbHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](
		alwaysFailService,
		CircuitBreaker(servicemiddleware.CircuitBreakerConfig{
			ServiceName:      "matrix",
			FailureThreshold: 2,
			Timeout:          time.Minute,
			SuccessThreshold: 1,
		}),
	)
	for i := 0; i < 2; i++ {
		warm := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/cb",
			strings.NewReader(`{"name":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		cbHandler(warm, req)
	}
	r.Post("/cb", cbHandler)

	// service_timeout: Timeout layer shorter than the handler sleep -> 503 (task-02).
	r.Post("/slow", WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](
		slowService,
		Timeout(10*time.Millisecond),
	))

	// panic: recovered by RecoverMiddleware -> 500 PANIC, details ABSENT (task-03).
	r.Get("/panic", HandlerCtxNoErr(func(ctx context.Context) Text {
		panic("boom in handler")
	}))

	// auth_failure: APIKey rejection -> 401 UNAUTHORIZED, JSON (task-03). The auth
	// middleware wraps only this route's handler; recover+request-id are global.
	secured := httpmiddleware.APIKeyMiddleware(httpmiddleware.APIKeyConfig{
		Keys: []string{"good-key"},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r.Get("/secure", secured)

	// rate_limit: capacity-0 limiter denies every request -> 429, JSON (task-03).
	limited := httpmiddleware.RateLimitMiddleware(
		httpmiddleware.NewTokenBucketLimiter(0, 0),
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r.Get("/limited", limited)

	return r
}

// espressoErrExtractor is an extractor that fails with an *espresso.Error so the
// extract path carries an explicit status/code (422) instead of the 400 default.
type espressoErrExtractor struct{}

func (*espressoErrExtractor) Extract(*http.Request) error {
	return ErrUnprocessableEntity("entity cannot be processed")
}

func TestErrorStatusMatrix(t *testing.T) {
	r := buildErrorMatrixRouter(t)

	jsonBody := func() *strings.Reader { return strings.NewReader(`{"name":"test"}`) }

	cases := []struct {
		name        string
		newRequest  func() *http.Request
		wantStatus  int
		wantCode    string
		wantDetails bool // assert the "details" key is present
		noDetails   bool // assert the "details" key is ABSENT
	}{
		{
			name: "extract_error",
			newRequest: func() *http.Request {
				// Malformed JSON body -> JSON extractor returns a plain error.
				req := httptest.NewRequest(http.MethodPost, "/extract-bad",
					strings.NewReader("{not-json"))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name: "extract_espresso_error",
			newRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/extract-esp", nil)
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "UNPROCESSABLE_ENTITY",
		},
		{
			name: "handler_error",
			newRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/not-found", nil)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name: "handler_plain_error",
			newRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/boom", nil)
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL",
		},
		{
			name: "service_validation",
			newRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/validate", jsonBody())
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus:  http.StatusBadRequest,
			wantCode:    "VALIDATION_ERROR",
			wantDetails: true,
		},
		{
			name: "service_circuit_breaker_open",
			newRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/cb", jsonBody())
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "SERVICE_UNAVAILABLE",
		},
		{
			name: "service_timeout",
			newRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/slow", jsonBody())
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "SERVICE_UNAVAILABLE",
		},
		{
			name: "panic",
			newRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/panic", nil)
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "PANIC",
			noDetails:  true,
		},
		{
			name: "auth_failure",
			newRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/secure", nil)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
		{
			name: "rate_limit",
			newRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/limited", nil)
			},
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "TOO_MANY_REQUESTS",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, tc.newRequest())

			// (a) status code.
			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d; body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}

			// (b) Content-Type is JSON with charset. No row may carry text/plain.
			ct := rec.Header().Get("Content-Type")
			const wantCT = "application/json; charset=utf-8"
			if ct != wantCT {
				t.Errorf("content-type: got %q, want %q", ct, wantCT)
			}

			// (c) body decodes into the canonical envelope with code + message.
			var env matrixEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("body is not the canonical envelope: %v; body=%s", err, rec.Body.String())
			}
			if env.Error.Code != tc.wantCode {
				t.Errorf("error.code: got %q, want %q", env.Error.Code, tc.wantCode)
			}
			if env.Error.Message == "" {
				t.Error("error.message: expected non-empty")
			}

			// details presence/absence via a raw-key decode.
			keys := errorMatrixKeys(t, rec.Body.Bytes())
			if tc.wantDetails && !keys["details"] {
				t.Errorf("expected details key present, got keys=%v", keys)
			}
			if tc.noDetails && keys["details"] {
				t.Errorf("expected details key ABSENT (panic must omit details), got keys=%v", keys)
			}
			// The validation row must carry details.errors with the field error.
			if tc.name == "service_validation" {
				errsRaw, ok := env.Error.Details["errors"]
				if !ok {
					t.Fatalf("validation row: details.errors missing; details=%v", env.Error.Details)
				}
				if errsList, ok := errsRaw.([]any); !ok || len(errsList) == 0 {
					t.Errorf("validation row: details.errors should be a non-empty list, got %#v", errsRaw)
				}
			}
		})
	}
}

// errorMatrixKeys returns the set of keys present under "error" in the raw JSON.
// Decoding into json.RawMessage distinguishes an absent key from a null value,
// which a struct-with-omitempty decode cannot.
func errorMatrixKeys(t *testing.T, body []byte) map[string]bool {
	t.Helper()
	var wrapper struct {
		Error map[string]json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		t.Fatalf("raw envelope decode failed: %v; body=%s", err, string(body))
	}
	keys := make(map[string]bool, len(wrapper.Error))
	for k := range wrapper.Error {
		keys[k] = true
	}
	return keys
}
