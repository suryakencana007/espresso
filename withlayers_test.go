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

	"github.com/rs/zerolog"
	"github.com/suryakencana007/espresso/v2/extractor"
	servicemiddleware "github.com/suryakencana007/espresso/v2/middleware/service"
)

// ============================================
// Test LayerStack
// ============================================

func TestLayers(t *testing.T) {
	logger := zerolog.Nop()

	stack := Layers(
		Timeout(5*time.Second),
		Logging(logger, "test"),
	)

	if len(stack) != 2 {
		t.Errorf("expected 2 layers, got %d", len(stack))
	}
}

func TestLayerStack_Combine(t *testing.T) {
	common := Layers(
		Timeout(5*time.Second),
		Logging(zerolog.Nop(), "common"),
	)

	userLayers := Layers(
		Validation(&mockValidator{}),
	)

	combined := common.Combine(userLayers)

	if len(combined) != 3 {
		t.Errorf("expected 3 layers after combine, got %d", len(combined))
	}
}

func TestLayerStack_Append(t *testing.T) {
	stack := Layers(Timeout(5 * time.Second))

	stack = stack.Append(
		Logging(zerolog.Nop(), "test"),
		ConcurrencyLimit(100),
	)

	if len(stack) != 3 {
		t.Errorf("expected 3 layers after append, got %d", len(stack))
	}
}

func TestLayerStack_Prepend(t *testing.T) {
	stack := Layers(
		Timeout(5*time.Second),
		Logging(zerolog.Nop(), "test"),
	)

	stack = stack.Prepend(ConcurrencyLimit(100))

	if len(stack) != 3 {
		t.Errorf("expected 3 layers after prepend, got %d", len(stack))
	}
}

// ============================================
// Test WithLayersTyped (Explicit Types)
// ============================================

func TestWithLayersTyped_Doppio(t *testing.T) {
	logger := zerolog.Nop()

	handler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "created " + req.Data.Name}}, nil
	}

	layers := Layers(
		Timeout(5*time.Second),
		Logging(logger, "test"),
	)

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](handler, layers...)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestWithLayersTyped_Solo(t *testing.T) {
	handler := func(req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "created " + req.Data.Name}}, nil
	}

	layers := Layers(Timeout(5 * time.Second))

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](handler, layers...)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestWithLayersTyped_Service(t *testing.T) {
	svc := &testUserService{}

	layers := Layers(Timeout(5 * time.Second))

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](svc, layers...)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestWithLayersTyped_WithTimeout(t *testing.T) {
	slowHandler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return JSON[UserRes]{Data: UserRes{Message: "done"}}, nil
		case <-ctx.Done():
			return JSON[UserRes]{}, ctx.Err()
		}
	}

	layers := Layers(Timeout(10 * time.Millisecond))

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](slowHandler, layers...)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	// v2.2 (task-02): TimeoutLayer deadlines now map to 503, not 500.
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 (timeout), got %d", rec.Code)
	}

	if code := decodeErrorCode(t, rec); code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected error code SERVICE_UNAVAILABLE, got %q", code)
	}
}

func TestWithLayersTyped_WithLogging(t *testing.T) {
	logger := zerolog.Nop()
	callCount := 0

	handler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		callCount++
		return JSON[UserRes]{Data: UserRes{Message: "created"}}, nil
	}

	layers := Layers(Logging(logger, "test"))

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](handler, layers...)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	if callCount != 1 {
		t.Errorf("expected handler to be called once, got %d", callCount)
	}
}

func TestWithLayersTyped_WithRetry(t *testing.T) {
	attempts := 0

	handler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		attempts++
		if attempts < 3 {
			return JSON[UserRes]{}, context.DeadlineExceeded
		}
		return JSON[UserRes]{Data: UserRes{Message: "success"}}, nil
	}

	layers := Layers(Retry(5, 10*time.Millisecond, servicemiddleware.BackoffFixed))

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](handler, layers...)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	if attempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", attempts)
	}
}

// ============================================
// Test WithLayers (Type Inference)
// ============================================

func TestWithLayers_Doppio_Inference(t *testing.T) {
	logger := zerolog.Nop()

	handler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "created " + req.Data.Name}}, nil
	}

	layers := Layers(
		Timeout(5*time.Second),
		Logging(logger, "test"),
	)

	// Should infer types from handler signature
	httpHandler := WithLayers(handler, layers...)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestWithLayers_Solo_Inference(t *testing.T) {
	handler := func(req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "created"}}, nil
	}

	layers := Layers(Timeout(5 * time.Second))

	httpHandler := WithLayers(handler, layers...)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestWithLayers_Ristretto_Inference(t *testing.T) {
	handler := func() Text {
		return Text{Body: "OK"}
	}

	layers := Layers(Timeout(5 * time.Second))

	// Should infer Req=struct{} for the 0-arg HandlerNoReqNoErr shape
	// (Ristretto since v1.6 takes ctx — see refactor/ristretto-ctx-aware).
	httpHandler := WithLayers(handler, layers...)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "OK" {
		t.Errorf("expected body 'OK', got %s", rec.Body.String())
	}
}

func TestWithLayers_MultipleHandlers(t *testing.T) {
	logger := zerolog.Nop()

	// Shared layer stack
	commonLayers := Layers(
		Timeout(5*time.Second),
		Logging(logger, "api"),
	)

	createUserHandler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "user created"}}, nil
	}

	getUserHandler := func(req *extractor.Path[GetUserReq]) (JSON[User], error) {
		return JSON[User]{Data: User{ID: req.Data.ID}}, nil
	}

	healthHandler := func() Text {
		return Text{Body: "healthy"}
	}

	// Apply same layers to different handlers
	userHandler := WithLayers(createUserHandler, commonLayers...)
	getHandler := WithLayers(getUserHandler, commonLayers...)
	healthHTTPHandler := WithLayers(healthHandler, commonLayers...)

	// Test user handler
	req1 := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	userHandler(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("expected user handler status 200, got %d", rec1.Code)
	}

	// Test get handler
	req2 := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req2.SetPathValue("id", "123")
	rec2 := httptest.NewRecorder()
	getHandler(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected get handler status 200, got %d", rec2.Code)
	}

	// Test health handler
	req3 := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec3 := httptest.NewRecorder()
	healthHTTPHandler(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Errorf("expected health handler status 200, got %d", rec3.Code)
	}
}

// ============================================
// Test Error Cases
// ============================================

func TestWithLayers_InvalidHandler(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid handler")
		}
	}()

	invalidHandler := "not a function"

	WithLayers(invalidHandler, Timeout(5*time.Second))
}

func TestWithLayers_WrongSignature(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for wrong signature")
		}
	}()

	// Wrong signature: 3 input parameters (invalid)
	wrongHandler := func(ctx context.Context, req *JSON[CreateUserReq], extra string) (JSON[UserRes], error) {
		return JSON[UserRes]{}, nil
	}

	WithLayers(wrongHandler, Timeout(5*time.Second))
}

// ============================================
// Test LayerConfig Types
// ============================================

func TestLayerConfig_Timeout(t *testing.T) {
	cfg := Timeout(5 * time.Second)
	if cfg == nil {
		t.Error("expected non-nil config")
	}
}

func TestLayerConfig_Logging(t *testing.T) {
	cfg := Logging(zerolog.Nop(), "test")
	if cfg == nil {
		t.Error("expected non-nil config")
	}
}

func TestLayerConfig_Retry(t *testing.T) {
	cfg := Retry(3, 100*time.Millisecond, servicemiddleware.BackoffExponential)
	if cfg == nil {
		t.Error("expected non-nil config")
	}
}

func TestLayerConfig_CircuitBreaker(t *testing.T) {
	cfg := CircuitBreaker(servicemiddleware.DefaultCircuitBreakerConfig)
	if cfg == nil {
		t.Error("expected non-nil config")
	}
}

func TestLayerConfig_ConcurrencyLimit(t *testing.T) {
	cfg := ConcurrencyLimit(100)
	if cfg == nil {
		t.Error("expected non-nil config")
	}
}

func TestWithLayers_ExtractorErrorReturnsStructuredJSON(t *testing.T) {
	// Lock-in: extractor failures go through writeExtractError, producing a
	// JSON body shaped as errorResponse with code "BAD_REQUEST", not plain text.
	handler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "ok"}}, nil
	}

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](handler)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected JSON Content-Type, got %q", ct)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not JSON: %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected code BAD_REQUEST, got %q", body.Error.Code)
	}
	if body.Error.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestLayerConfig_Metrics(t *testing.T) {
	collector := &mockMetricsCollector{}
	cfg := Metrics(collector, "test")
	if cfg == nil {
		t.Error("expected non-nil config")
	}
}

// ============================================
// Test Service-Layer Error → HTTP Status Mapping (v2.2 task-02)
// ============================================

// decodeErrorCode asserts the recorder holds a canonical structured error
// envelope and returns its error.code. It fails the test if the body is not
// the {"error":{"code","message","details","request_id"}} shape.
func decodeErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected JSON Content-Type, got %q", ct)
	}

	var body struct {
		Error struct {
			Code      string         `json:"code"`
			Message   string         `json:"message"`
			Details   map[string]any `json:"details"`
			RequestID string         `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not JSON: %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Message == "" {
		t.Error("expected non-empty error message in envelope")
	}
	return body.Error.Code
}

// fieldErrorsValidator returns espresso.FieldErrors so the validation layer
// error carries structured field detail.
type fieldErrorsValidator struct{}

func (fieldErrorsValidator) Validate(ctx context.Context, req *JSON[CreateUserReq]) error {
	return FieldErrors{{Field: "name", Message: "is required"}}
}

// plainErrorValidator returns a non-FieldErrors error.
type plainErrorValidator struct{}

func (plainErrorValidator) Validate(ctx context.Context, req *JSON[CreateUserReq]) error {
	return errors.New("name is bad")
}

func newUserPostRequest() (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	return req, httptest.NewRecorder()
}

func TestLayerError_Validation_400(t *testing.T) {
	handler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "ok"}}, nil
	}

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](
		handler,
		Validation[*JSON[CreateUserReq]](fieldErrorsValidator{}),
	)

	req, rec := newUserPostRequest()
	httpHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "VALIDATION_ERROR" {
		t.Errorf("expected code VALIDATION_ERROR, got %q", code)
	}

	var body struct {
		Error struct {
			Details struct {
				Errors []ValidationError `json:"errors"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not JSON: %v", err)
	}
	if len(body.Error.Details.Errors) != 1 {
		t.Fatalf("expected 1 field error preserved in details.errors, got %d", len(body.Error.Details.Errors))
	}
	if body.Error.Details.Errors[0].Field != "name" {
		t.Errorf("expected field 'name', got %q", body.Error.Details.Errors[0].Field)
	}
}

func TestLayerError_Validation_NonFieldErrors_400(t *testing.T) {
	handler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{Data: UserRes{Message: "ok"}}, nil
	}

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](
		handler,
		Validation[*JSON[CreateUserReq]](plainErrorValidator{}),
	)

	req, rec := newUserPostRequest()
	httpHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "VALIDATION_ERROR" {
		t.Errorf("expected code VALIDATION_ERROR, got %q", code)
	}

	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not JSON: %v", err)
	}
	if body.Error.Message != "name is bad" {
		t.Errorf("expected message preserved as 'name is bad', got %q", body.Error.Message)
	}
}

func TestLayerError_CircuitBreakerOpen_503(t *testing.T) {
	failing := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{}, errors.New("upstream down")
	}

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](
		failing,
		CircuitBreaker(servicemiddleware.CircuitBreakerConfig{
			ServiceName:      "users",
			FailureThreshold: 2,
			Timeout:          time.Minute,
			SuccessThreshold: 1,
		}),
	)

	// Trip the breaker: FailureThreshold failures move it to Open.
	for i := 0; i < 2; i++ {
		req, rec := newUserPostRequest()
		httpHandler(rec, req)
	}

	// The next call is rejected with *servicemiddleware.CircuitBreakerError.
	req, rec := newUserPostRequest()
	httpHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected code SERVICE_UNAVAILABLE, got %q", code)
	}
}

func TestLayerError_Timeout_503(t *testing.T) {
	slowHandler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return JSON[UserRes]{Data: UserRes{Message: "done"}}, nil
		case <-ctx.Done():
			return JSON[UserRes]{}, ctx.Err()
		}
	}

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](
		slowHandler,
		Timeout(10*time.Millisecond),
	)

	req, rec := newUserPostRequest()
	httpHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected code SERVICE_UNAVAILABLE, got %q", code)
	}
}

func TestLayerError_Unknown_500(t *testing.T) {
	handler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{}, errors.New("boom")
	}

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](
		handler,
		Logging(zerolog.Nop(), "test"),
	)

	req, rec := newUserPostRequest()
	httpHandler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "INTERNAL" {
		t.Errorf("expected code INTERNAL, got %q", code)
	}
}

func TestLayerError_ExplicitEspressoError_Passthrough(t *testing.T) {
	handler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{}, ErrConflict("user already exists")
	}

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](
		handler,
		Timeout(5*time.Second),
		Logging(zerolog.Nop(), "test"),
	)

	req, rec := newUserPostRequest()
	httpHandler(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected status 409 (passthrough), got %d", rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "CONFLICT" {
		t.Errorf("expected code CONFLICT, got %q", code)
	}
}

// ============================================
// Test Helpers
// ============================================

type testUserService struct {
}

func (s *testUserService) Call(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
	return JSON[UserRes]{Data: UserRes{Message: "created"}}, nil
}

// mockValidator implements servicemiddleware.Validator[any] so it can be
// passed to Validation in tests where the request type is intentionally
// not the focus. Real call sites should pass Validator[*JSON[T]] or similar.
type mockValidator struct{}

func (v *mockValidator) Validate(ctx context.Context, req any) error {
	return nil
}

type mockMetricsCollector struct {
	requestCount int
	activeCount  int
}

func (m *mockMetricsCollector) RecordRequest(serviceName string, duration time.Duration, err error) {
	m.requestCount++
}

func (m *mockMetricsCollector) RecordActiveRequests(serviceName string, delta int) {
	m.activeCount += delta
}

type GetUserReq struct {
	ID int `path:"id"`
}

type UserRes struct {
	Message string `json:"message"`
}

// User type is already defined in server_test.go

// timeoutRaceReq is used by TestWithLayersTyped_TimeoutNoPoolRace to expose
// the pre-fix data race. The handler reads req.Marker in a loop after the
// timeout fires; the framework's pooled request struct is reset+returned
// (or, post-fix, leaked) — the race detector fires on the pre-fix code
// when the next request's Extract writes into the same *timeoutRaceReq
// while the abandoned goroutine is still reading it.
type timeoutRaceReq struct {
	Marker string
}

func (r *timeoutRaceReq) Extract(req *http.Request) error {
	r.Marker = req.URL.Query().Get("marker")
	return nil
}

func (r *timeoutRaceReq) Reset() {
	r.Marker = ""
}

// TestWithLayersTyped_TimeoutNoPoolRace is the audit's exact repro for the
// v2.4 task-01 defect. TimeoutLayer spawns a goroutine and returns on
// ctx.Done, leaving that goroutine referencing the pooled *timeoutRaceReq.
// applyLayersAndConvert pre-fix returned the struct to the pool
// unconditionally; the next request's pool.Get()+Extract wrote into it while
// the abandoned goroutine's tight read loop was still reading it. Under
// -race this fires reliably.
//
// Post-fix, applyLayersAndConvert checks servicemiddleware.IsAbandonedByTimeout
// on the returned error and skips pool.Put for that request, leaking one
// struct to GC rather than reusing it while a goroutine still holds it.
func TestWithLayersTyped_TimeoutNoPoolRace(t *testing.T) {
	// Handler reads req.Marker for 80 ms — long enough to outlive the 10 ms
	// timeout, short enough to complete during the test's wall-clock window.
	slowRead := func(ctx context.Context, req *timeoutRaceReq) (Text, error) {
		end := time.Now().Add(80 * time.Millisecond)
		var seen string
		for time.Now().Before(end) {
			seen = req.Marker
			_ = seen
		}
		return Text{Body: "ok"}, nil
	}

	handler := WithLayersTyped[*timeoutRaceReq, Text](slowRead, Timeout(10*time.Millisecond))

	// Fire enough requests that at least one pool.Get() returns a struct
	// still held by an abandoned goroutine from the previous request.
	// 50 iterations is comfortably above the pool churn needed.
	for i := range 50 {
		req := httptest.NewRequest(http.MethodGet, "/x?marker=req"+string(rune('0'+i%10)), nil)
		rec := httptest.NewRecorder()
		handler(rec, req)
		// Expected status is 503 (SERVICE_UNAVAILABLE via translateLayerError):
		// TimeoutLayer surfaces &abandonedByTimeoutErr{context.DeadlineExceeded},
		// which unwraps to context.DeadlineExceeded and maps through
		// translateLayerError to ErrServiceUnavailable.
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("iter %d: expected status 503 on timeout, got %d body=%s", i, rec.Code, rec.Body.String())
		}
	}
	// The important assertion is implicit — go test -race must not print
	// WARNING: DATA RACE. Pre-fix, it does; post-fix it does not.
}

// TestTranslateLayerError_ContextCanceledMapsTo503 locks the secondary
// behavior change: parent context.Canceled surfacing through TimeoutLayer
// (or directly from a handler) now maps to 503, not the previous 500
// fallback. Companion of TestWithLayersTyped_WithTimeout at :145.
func TestTranslateLayerError_ContextCanceledMapsTo503(t *testing.T) {
	cancelReturningHandler := func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[UserRes], error) {
		return JSON[UserRes]{}, context.Canceled
	}

	httpHandler := WithLayersTyped[*JSON[CreateUserReq], JSON[UserRes]](cancelReturningHandler)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 (context.Canceled), got %d", rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected error code SERVICE_UNAVAILABLE, got %q", code)
	}
}
