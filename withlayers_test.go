package espresso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/suryakencana007/espresso/extractor"
	servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
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

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 (timeout), got %d", rec.Code)
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

	// Should infer Req=struct{} for Ristretto
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

func TestLayerConfig_Metrics(t *testing.T) {
	collector := &mockMetricsCollector{}
	cfg := Metrics(collector, "test")
	if cfg == nil {
		t.Error("expected non-nil config")
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
