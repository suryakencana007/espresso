package servicemiddleware

import (
	"context"
	"errors"
	"sync"
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

// TestCircuitBreaker_DoesNotOpenOnNonConsecutiveFailures locks D1 from
// v2.4 task-07. Pre-fix, StateClosed successes did NOT reset the failure
// counter — it only reset on a HalfOpen→Closed transition. So
// FailureThreshold=5 meant "5 failures over the entire process lifetime"
// (5 transient errors across days of 99.99% success opens the circuit).
// The audit repro: fail every 20th call across 102 calls (5 failures,
// 97 successes, 95% success rate). Pre-fix the circuit opens around call
// 100; post-fix successes reset failures so it stays Closed.
func TestCircuitBreaker_DoesNotOpenOnNonConsecutiveFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		ServiceName:       "test",
		FailureThreshold:  5,
		Timeout:           100 * time.Millisecond,
		SuccessThreshold:  1,
		HalfOpenMaxProbes: 1,
	}

	attempt := 0
	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		attempt++
		if attempt%20 == 0 {
			return "", errors.New("transient")
		}
		return "ok", nil
	})

	layer := CircuitBreakerLayer[string, string](config)
	wrapped := layer(svc)

	for i := 1; i <= 102; i++ {
		_, err := wrapped.Call(context.Background(), "test")
		if i%20 == 0 {
			if err == nil {
				t.Fatalf("call %d: expected transient failure", i)
			}
			if IsCircuitBreakerError(err) {
				t.Fatalf("call %d: circuit opened on non-consecutive failures (pre-fix behavior); errs=%v", i, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("call %d: unexpected error %v (circuit likely opened — pre-fix behavior)", i, err)
		}
	}
}

// TestCircuitBreaker_TransitioningProbeSuccessCounts locks D2. The
// goroutine that flipped Open→HalfOpen captured currentState=StateOpen on
// entry; the pre-fix success-recording condition `if currentState ==
// StateHalfOpen` was false, so the transitioning probe's success was never
// counted toward SuccessThreshold. With SuccessThreshold=1, a single
// successful probe should close the circuit — pre-fix, it took a second
// call.
func TestCircuitBreaker_TransitioningProbeSuccessCounts(t *testing.T) {
	config := CircuitBreakerConfig{
		ServiceName:       "test",
		FailureThreshold:  2,
		Timeout:           50 * time.Millisecond,
		SuccessThreshold:  1,
		HalfOpenMaxProbes: 1,
	}

	attempt := 0
	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		attempt++
		if attempt <= 2 {
			return "", errors.New("initial failure")
		}
		return "ok", nil
	})

	layer := CircuitBreakerLayer[string, string](config)
	wrapped := layer(svc)

	// Trip the circuit.
	_, _ = wrapped.Call(context.Background(), "x")
	_, _ = wrapped.Call(context.Background(), "x")

	// Let the open-timeout elapse so the next call transitions to HalfOpen.
	time.Sleep(60 * time.Millisecond)

	// This single probe should close the circuit (SuccessThreshold=1).
	if _, err := wrapped.Call(context.Background(), "x"); err != nil {
		t.Fatalf("transitioning probe should succeed, got %v", err)
	}

	// Fire another call to verify the circuit is Closed — pre-fix, the
	// first probe's success was not counted so this second call was
	// still gated by the half-open state.
	if _, err := wrapped.Call(context.Background(), "x"); err != nil {
		if IsCircuitBreakerError(err) {
			t.Fatalf("circuit should be Closed after successful transitioning probe (pre-fix behavior): %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCircuitBreaker_HalfOpenLimitsConcurrentProbes locks D4. Pre-fix,
// every request observing HalfOpen passed through as a probe — defeating
// the purpose of probing (probes are supposed to test recovery with
// LIMITED traffic, not all pending traffic). With HalfOpenMaxProbes=1
// and 10 concurrent requests, exactly one should reach the wrapped
// service and nine should short-circuit with a CircuitBreakerError.
func TestCircuitBreaker_HalfOpenLimitsConcurrentProbes(t *testing.T) {
	config := CircuitBreakerConfig{
		ServiceName:       "test",
		FailureThreshold:  2,
		Timeout:           30 * time.Millisecond,
		SuccessThreshold:  1,
		HalfOpenMaxProbes: 1,
	}

	release := make(chan struct{})
	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		if req == "fail" {
			return "", errors.New("fail")
		}
		// Probes block until released so all 10 requests coexist.
		<-release
		return "ok", nil
	})

	layer := CircuitBreakerLayer[string, string](config)
	wrapped := layer(svc)

	// Trip the circuit.
	_, _ = wrapped.Call(context.Background(), "fail")
	_, _ = wrapped.Call(context.Background(), "fail")

	// Let the open-timeout elapse.
	time.Sleep(40 * time.Millisecond)

	var (
		mu       sync.Mutex
		passed   int
		rejected int
	)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := wrapped.Call(context.Background(), "probe")
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				passed++
				return
			}
			if !IsCircuitBreakerError(err) {
				t.Errorf("unexpected non-circuit-breaker error: %v", err)
				return
			}
			rejected++
		}()
	}
	// Give goroutines time to reach the wrapped service (or be
	// short-circuited) before releasing the passing one.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if passed != 1 {
		t.Errorf("expected 1 probe to reach the wrapped service, got %d (pre-fix behavior: unbounded probes)", passed)
	}
	if rejected != 9 {
		t.Errorf("expected 9 probes to be rejected, got %d", rejected)
	}
}

// TestCircuitBreaker_ConcurrentSuccessFailureRaceFree locks D3 under
// -race. Pre-fix, the success path mutated on the stale local
// currentState captured on entry, so concurrent Success + Failure paths
// could race the state-machine mutations without re-checking under the
// lock. Post-fix, both paths re-read state under the write lock at the
// mutation point; -race sees no data races and the state stays in a
// legal transition.
func TestCircuitBreaker_ConcurrentSuccessFailureRaceFree(t *testing.T) {
	config := CircuitBreakerConfig{
		ServiceName:       "test",
		FailureThreshold:  2,
		Timeout:           30 * time.Millisecond,
		SuccessThreshold:  4,
		HalfOpenMaxProbes: 8,
	}

	svc := serviceFunc[string, string](func(ctx context.Context, req string) (string, error) {
		if req == "fail" {
			return "", errors.New("fail")
		}
		return "ok", nil
	})

	layer := CircuitBreakerLayer[string, string](config)
	wrapped := layer(svc)

	// Trip and transition to HalfOpen.
	_, _ = wrapped.Call(context.Background(), "fail")
	_, _ = wrapped.Call(context.Background(), "fail")
	time.Sleep(40 * time.Millisecond)

	// Fire many concurrent success + failure probes so the state
	// machine takes multiple transitions under contention.
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := "ok"
			if i%2 == 0 {
				req = "fail"
			}
			_, _ = wrapped.Call(context.Background(), req)
		}(i)
	}
	wg.Wait()
	// Test passes if `go test -race` reports no WARNING: DATA RACE.
}
