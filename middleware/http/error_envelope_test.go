package httpmiddleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================
// Error-envelope consistency (v2.2 task-03)
// ============================================
//
// These tests lock the canonical JSON error envelope on the three framework
// error paths that previously bypassed it: the panic recover path, auth (401),
// and rate-limit (429). They began as characterization tests asserting the
// pre-task-03 behavior (panic JSON without a "details" key; auth/rate-limit
// text/plain) and were flipped to assert the unified envelope once those paths
// were routed through internal/errorenvelope. The before/after of these
// assertions is the proof that the unification is a fix, not a regression.

// decodeEnvelope unmarshals a recorder body into the canonical error envelope
// shape and asserts the Content-Type is JSON. It returns the decoded inner
// error object plus the set of keys present under "error" (so a test can assert
// that "details" is present-or-absent, matching the omitempty contract).
func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) (code, message, requestID string, keys map[string]bool) {
	t.Helper()

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected JSON Content-Type, got %q", ct)
	}

	var wrapper struct {
		Error map[string]json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &wrapper); err != nil {
		t.Fatalf("response body is not the JSON envelope: %v; body=%s", err, rec.Body.String())
	}
	if wrapper.Error == nil {
		t.Fatalf("expected an %q object, got body=%s", "error", rec.Body.String())
	}

	keys = make(map[string]bool, len(wrapper.Error))
	for k := range wrapper.Error {
		keys[k] = true
	}
	if raw, ok := wrapper.Error["code"]; ok {
		_ = json.Unmarshal(raw, &code)
	}
	if raw, ok := wrapper.Error["message"]; ok {
		_ = json.Unmarshal(raw, &message)
	}
	if raw, ok := wrapper.Error["request_id"]; ok {
		_ = json.Unmarshal(raw, &requestID)
	}
	return code, message, requestID, keys
}

// TestRecoverMiddleware_EnvelopeShape: a panic recovered by RecoverMiddleware
// emits the canonical envelope (code "PANIC", request_id populated) and — like
// a writeHandlerError 500 — OMITS the "details" key (Details nil -> omitempty).
//
// Before task-03 this path hand-rolled an anonymous struct that also omitted
// "details" but never matched the canonical writer byte-for-byte; the flip here
// is that the body now comes from internal/errorenvelope.Write, the same writer
// the root package delegates to.
func TestRecoverMiddleware_EnvelopeShape(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	// RequestIDMiddleware in front so the envelope's request_id is populated.
	server := RequestIDMiddleware()(RecoverMiddleware()(handler))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	code, message, requestID, keys := decodeEnvelope(t, rec)
	if code != "PANIC" {
		t.Errorf("expected code PANIC, got %q", code)
	}
	if message == "" {
		t.Error("expected non-empty message")
	}
	if requestID == "" {
		t.Error("expected request_id to be populated by RequestIDMiddleware")
	}
	// Canonical 500 (writeHandlerError) omits "details" when nil; the panic
	// path must match that exactly.
	if keys["details"] {
		t.Errorf("expected NO %q key in panic envelope (must match writeHandlerError 500), got keys=%v", "details", keys)
	}
}

// TestAuthMiddleware_Unauthorized_Envelope: JWT/BasicAuth/APIKey rejections now
// emit the canonical JSON envelope with code UNAUTHORIZED and a request_id,
// instead of the pre-task-03 text/plain "Unauthorized" body.
func TestAuthMiddleware_Unauthorized_Envelope(t *testing.T) {
	pass := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not run on an unauthorized request")
	})

	cases := []struct {
		name       string
		middleware Middleware
		req        func() *http.Request
	}{
		{
			name: "JWT missing token",
			middleware: JWTMiddleware(JWTConfig{
				Secret: "secret",
				ClaimsExtractor: func(string) (map[string]any, error) {
					return map[string]any{}, nil
				},
			}),
			req: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t", nil) },
		},
		{
			name: "JWT invalid token",
			middleware: JWTMiddleware(JWTConfig{
				Secret: "secret",
				ClaimsExtractor: func(string) (map[string]any, error) {
					return nil, ErrNoToken
				},
			}),
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/t", nil)
				r.Header.Set("Authorization", "Bearer abc")
				return r
			},
		},
		{
			name:       "BasicAuth missing credentials",
			middleware: BasicAuthMiddleware(BasicAuthConfig{Users: map[string]string{"u": "p"}}),
			req:        func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t", nil) },
		},
		{
			name:       "BasicAuth wrong credentials",
			middleware: BasicAuthMiddleware(BasicAuthConfig{Users: map[string]string{"u": "p"}}),
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/t", nil)
				r.SetBasicAuth("u", "wrong")
				return r
			},
		},
		{
			name:       "APIKey missing key",
			middleware: APIKeyMiddleware(APIKeyConfig{Keys: []string{"k"}}),
			req:        func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t", nil) },
		},
		{
			name:       "APIKey invalid key",
			middleware: APIKeyMiddleware(APIKeyConfig{Keys: []string{"k"}}),
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/t", nil)
				r.Header.Set("X-API-Key", "nope")
				return r
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := RequestIDMiddleware()(tc.middleware(pass))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, tc.req())

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected status 401, got %d", rec.Code)
			}
			code, message, requestID, _ := decodeEnvelope(t, rec)
			if code != "UNAUTHORIZED" {
				t.Errorf("expected code UNAUTHORIZED, got %q", code)
			}
			if message == "" {
				t.Error("expected non-empty message in envelope")
			}
			if requestID == "" {
				t.Error("expected request_id to be populated")
			}
		})
	}
}

// TestRateLimit_TooManyRequests_Envelope: a denied request now emits the
// canonical JSON envelope with code TOO_MANY_REQUESTS instead of the
// pre-task-03 text/plain "Too Many Requests" body.
func TestRateLimit_TooManyRequests_Envelope(t *testing.T) {
	pass := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Capacity 0 -> always denied.
	limiter := NewTokenBucketLimiter(0, 0)
	server := RequestIDMiddleware()(RateLimitMiddleware(limiter)(pass))

	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", rec.Code)
	}
	code, message, requestID, _ := decodeEnvelope(t, rec)
	if code != "TOO_MANY_REQUESTS" {
		t.Errorf("expected code TOO_MANY_REQUESTS, got %q", code)
	}
	if message == "" {
		t.Error("expected non-empty message in envelope")
	}
	if requestID == "" {
		t.Error("expected request_id to be populated")
	}
}
