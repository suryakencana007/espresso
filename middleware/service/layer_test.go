package servicemiddleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestTimeoutLayer_Timeout(t *testing.T) {
	layer := TimeoutLayer[string, string](10 * time.Millisecond)

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "result", nil
	})

	wrapped := layer(svc)

	_, err := wrapped.Call(context.Background(), "test")
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestTimeoutLayer_Success(t *testing.T) {
	layer := TimeoutLayer[string, string](100 * time.Millisecond)

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		return "result", nil
	})

	wrapped := layer(svc)

	res, err := wrapped.Call(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if res != "result" {
		t.Errorf("expected 'result', got '%s'", res)
	}
}

func TestRetryLayer_Success(t *testing.T) {
	layer := RetryLayer[string, string](3, 10*time.Millisecond, BackoffFixed)

	attempts := 0
	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		attempts++
		if attempts < 2 {
			return "", errors.New("temporary error")
		}
		return "result", nil
	})

	wrapped := layer(svc)

	res, err := wrapped.Call(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if res != "result" {
		t.Errorf("expected 'result', got '%s'", res)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryLayer_MaxRetries(t *testing.T) {
	layer := RetryLayer[string, string](2, 10*time.Millisecond, BackoffFixed)

	attempts := 0
	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		attempts++
		return "", errors.New("always fails")
	})

	wrapped := layer(svc)

	_, err := wrapped.Call(context.Background(), "test")
	if err == nil {
		t.Error("expected error after max retries")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", attempts)
	}
}

func TestCircuitBreakerLayer_Open(t *testing.T) {
	config := CircuitBreakerConfig{
		ServiceName:      "test",
		FailureThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}

	attempts := 0
	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		attempts++
		return "", errors.New("always fails")
	})

	layer := CircuitBreakerLayer[string, string](config)
	wrapped := layer(svc)

	for i := 0; i < 3; i++ {
		_, _ = wrapped.Call(context.Background(), "test")
	}

	_, err := wrapped.Call(context.Background(), "test")
	if err == nil {
		t.Error("expected circuit breaker error")
	}

	var cbErr *CircuitBreakerError
	if !errors.As(err, &cbErr) {
		t.Errorf("expected CircuitBreakerError, got %T", err)
	}
}

func TestCircuitBreakerLayer_Closed(t *testing.T) {
	config := CircuitBreakerConfig{
		ServiceName:      "test",
		FailureThreshold: 5,
		Timeout:          100 * time.Millisecond,
	}

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		return "result", nil
	})

	layer := CircuitBreakerLayer[string, string](config)
	wrapped := layer(svc)

	res, err := wrapped.Call(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if res != "result" {
		t.Errorf("expected 'result', got '%s'", res)
	}
}

func TestConcurrencyLimitLayer(t *testing.T) {
	layer := ConcurrencyLimitLayer[string, string](2)

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "result", nil
	})

	wrapped := layer(svc)

	done := make(chan string, 3)

	for i := 0; i < 3; i++ {
		go func() {
			res, _ := wrapped.Call(context.Background(), "test")
			done <- res
		}()
	}

	results := 0
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			results++
		case <-time.After(200 * time.Millisecond):
		}
	}

	if results != 3 {
		t.Errorf("expected 3 results, got %d", results)
	}
}

type mockMetricsCollector struct {
	requests []struct {
		serviceName string
		duration    time.Duration
		err         error
	}
	activeRequests []struct {
		serviceName string
		delta       int
	}
}

func (m *mockMetricsCollector) RecordRequest(serviceName string, duration time.Duration, err error) {
	m.requests = append(m.requests, struct {
		serviceName string
		duration    time.Duration
		err         error
	}{serviceName, duration, err})
}

func (m *mockMetricsCollector) RecordActiveRequests(serviceName string, delta int) {
	m.activeRequests = append(m.activeRequests, struct {
		serviceName string
		delta       int
	}{serviceName, delta})
}

func TestMetricsLayer(t *testing.T) {
	collector := &mockMetricsCollector{}
	layer := MetricsLayer[string, string](collector, "test-service")

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		return "result", nil
	})

	wrapped := layer(svc)

	_, err := wrapped.Call(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(collector.requests) != 1 {
		t.Errorf("expected 1 recorded request, got %d", len(collector.requests))
	}
	if len(collector.activeRequests) != 2 {
		t.Errorf("expected 2 active request records (+1, -1), got %d", len(collector.activeRequests))
	}
}

type mockValidator struct {
	shouldFail bool
}

func (m *mockValidator) Validate(ctx context.Context, req string) error {
	if m.shouldFail {
		return errors.New("validation failed")
	}
	return nil
}

func TestValidationLayer_Success(t *testing.T) {
	validator := &mockValidator{shouldFail: false}
	layer := ValidationLayer[string, string](validator)

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		return "result", nil
	})

	wrapped := layer(svc)

	res, err := wrapped.Call(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if res != "result" {
		t.Errorf("expected 'result', got '%s'", res)
	}
}

func TestValidationLayer_Failure(t *testing.T) {
	validator := &mockValidator{shouldFail: true}
	layer := ValidationLayer[string, string](validator)

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		return "result", nil
	})

	wrapped := layer(svc)

	_, err := wrapped.Call(context.Background(), "test")
	if err == nil {
		t.Error("expected validation error")
	}

	var validationErr ErrValidation
	if !errors.As(err, &validationErr) {
		t.Errorf("expected ErrValidation, got %T", err)
	}
}

func TestLoggingLayer(t *testing.T) {
	logger := zerolog.Nop()
	layer := LoggingLayer[string, string](logger, "test-service")

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		return "result", nil
	})

	wrapped := layer(svc)

	res, err := wrapped.Call(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if res != "result" {
		t.Errorf("expected 'result', got '%s'", res)
	}
}

func TestCircuitBreakerError(t *testing.T) {
	err := NewCircuitBreakerError("test-service", StateOpen, "circuit breaker is open")

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}

	if !IsCircuitBreakerError(err) {
		t.Error("expected IsCircuitBreakerError to return true")
	}
}

func BenchmarkTimeoutLayer(b *testing.B) {
	layer := TimeoutLayer[string, string](100 * time.Millisecond)

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		return "result", nil
	})

	wrapped := layer(svc)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = wrapped.Call(context.Background(), "test")
	}
}

func BenchmarkRetryLayer(b *testing.B) {
	layer := RetryLayer[string, string](0, 10*time.Millisecond, BackoffFixed)

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		return "result", nil
	})

	wrapped := layer(svc)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = wrapped.Call(context.Background(), "test")
	}
}
