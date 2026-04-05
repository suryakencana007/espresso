package servicemiddleware

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Service represents a typed request/response service.
type Service[Req any, Res any] interface {
	Call(ctx context.Context, req Req) (Res, error)
}

type serviceFunc[Req any, Res any] func(context.Context, Req) (Res, error)

func (f serviceFunc[Req, Res]) Call(ctx context.Context, req Req) (Res, error) {
	return f(ctx, req)
}

// Layer wraps a service with cross-cutting behavior.
type Layer[Req any, Res any] func(Service[Req, Res]) Service[Req, Res]

// LoggingLayer logs service execution latency and errors.
func LoggingLayer[Req any, Res any](logger zerolog.Logger, serviceName string) Layer[Req, Res] {
	return func(next Service[Req, Res]) Service[Req, Res] {
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			start := time.Now()

			res, err := next.Call(ctx, req)

			duration := time.Since(start)
			logEvent := logger.Info()
			if err != nil {
				logEvent = logger.Error().Err(err)
			}

			logEvent.
				Str("service", serviceName).
				Dur("latency", duration).
				Msg("Request processed")

			return res, err
		})
	}
}

// TimeoutLayer applies a timeout to service calls.
func TimeoutLayer[Req any, Res any](timeout time.Duration) Layer[Req, Res] {
	return func(next Service[Req, Res]) Service[Req, Res] {
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			type result struct {
				res Res
				err error
			}
			ch := make(chan result, 1)

			go func() {
				res, err := next.Call(ctx, req)
				ch <- result{res, err}
			}()

			select {
			case <-ctx.Done():
				var zero Res
				return zero, ctx.Err()
			case r := <-ch:
				return r.res, r.err
			}
		})
	}
}

// BackoffStrategy controls retry delay progression.
type BackoffStrategy int

const (
	// BackoffFixed keeps retry delay constant.
	BackoffFixed BackoffStrategy = iota
	// BackoffExponential doubles retry delay each attempt.
	BackoffExponential
	// BackoffLinear adds initial delay each attempt.
	BackoffLinear
)

// RetryLayer retries failed service calls using the chosen backoff strategy.
func RetryLayer[Req any, Res any](maxRetries int, initialBackoff time.Duration, strategy BackoffStrategy) Layer[Req, Res] {
	return func(next Service[Req, Res]) Service[Req, Res] {
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			var lastErr error
			backoff := initialBackoff

			for attempt := 0; attempt <= maxRetries; attempt++ {
				res, err := next.Call(ctx, req)
				if err == nil {
					return res, nil
				}

				lastErr = err
				if attempt == maxRetries {
					break
				}

				select {
				case <-ctx.Done():
					var zero Res
					return zero, ctx.Err()
				case <-time.After(backoff):
				}

				switch strategy {
				case BackoffExponential:
					backoff *= 2
				case BackoffLinear:
					backoff += initialBackoff
				case BackoffFixed:
				}
			}

			var zero Res
			return zero, lastErr
		})
	}
}

// CircuitState is the state of a circuit breaker.
type CircuitState int32

const (
	// StateClosed allows all requests.
	StateClosed CircuitState = 0
	// StateOpen rejects requests until timeout elapses.
	StateOpen CircuitState = 1
	// StateHalfOpen allows probing requests after open timeout.
	StateHalfOpen CircuitState = 2
)

// CircuitBreakerConfig configures circuit breaker behavior.
type CircuitBreakerConfig struct {
	ServiceName      string
	FailureThreshold int
	Timeout          time.Duration
	SuccessThreshold int
}

// DefaultCircuitBreakerConfig provides sensible defaults.
var DefaultCircuitBreakerConfig = CircuitBreakerConfig{
	FailureThreshold: 5,
	Timeout:          30 * time.Second,
	SuccessThreshold: 3,
}

// CircuitBreakerState stores mutable runtime circuit breaker state.
type CircuitBreakerState struct {
	mu           sync.RWMutex
	state        CircuitState
	failures     int
	successes    int
	lastFailTime time.Time
}

// CircuitBreakerError indicates a rejected call due to an open circuit.
type CircuitBreakerError struct {
	ServiceName string
	State       CircuitState
	Message     string
}

func (e *CircuitBreakerError) Error() string {
	if e.Message != "" {
		return "circuit breaker open for service " + e.ServiceName + ": " + e.Message
	}
	return "circuit breaker open for service " + e.ServiceName
}

func (e *CircuitBreakerError) Unwrap() error {
	return nil
}

// Is reports whether target is a CircuitBreakerError.
func (e *CircuitBreakerError) Is(target error) bool {
	_, ok := target.(*CircuitBreakerError)
	return ok
}

// NewCircuitBreakerError creates a new CircuitBreakerError.
func NewCircuitBreakerError(serviceName string, state CircuitState, message string) *CircuitBreakerError {
	return &CircuitBreakerError{
		ServiceName: serviceName,
		State:       state,
		Message:     message,
	}
}

// IsCircuitBreakerError reports whether err is a CircuitBreakerError.
func IsCircuitBreakerError(err error) bool {
	var cbErr *CircuitBreakerError
	return errorsAs(err, &cbErr)
}

func errorsAs(err error, target any) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*CircuitBreakerError); ok {
		if targetPtr, ok := target.(**CircuitBreakerError); ok {
			*targetPtr = e
			return true
		}
	}
	return false
}

// ErrCircuitBreakerOpen is a sentinel error for open-circuit rejection.
var ErrCircuitBreakerOpen = NewCircuitBreakerError("", StateOpen, "circuit breaker is open")

// CircuitBreakerLayer applies circuit breaker protection around a service.
func CircuitBreakerLayer[Req any, Res any](config CircuitBreakerConfig) Layer[Req, Res] {
	state := &CircuitBreakerState{state: StateClosed}

	return func(next Service[Req, Res]) Service[Req, Res] {
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			state.mu.RLock()
			currentState := state.state
			state.mu.RUnlock()

			switch currentState {
			case StateOpen:
				state.mu.RLock()
				lastFail := state.lastFailTime
				state.mu.RUnlock()

				if time.Since(lastFail) > config.Timeout {
					state.mu.Lock()
					state.state = StateHalfOpen
					state.successes = 0
					state.mu.Unlock()
				} else {
					var zero Res
					return zero, NewCircuitBreakerError(config.ServiceName, StateOpen, "circuit breaker is open")
				}
			}

			res, err := next.Call(ctx, req)

			if err != nil {
				state.mu.Lock()
				state.failures++
				state.lastFailTime = time.Now()
				if state.failures >= config.FailureThreshold {
					state.state = StateOpen
				}
				state.mu.Unlock()
				return res, err
			}

			if currentState == StateHalfOpen {
				state.mu.Lock()
				state.successes++
				if state.successes >= config.SuccessThreshold {
					state.state = StateClosed
					state.failures = 0
				}
				state.mu.Unlock()
			}

			return res, nil
		})
	}
}

// ConcurrencyLimitLayer limits concurrent in-flight service calls.
func ConcurrencyLimitLayer[Req any, Res any](maxConcurrent int) Layer[Req, Res] {
	sem := make(chan struct{}, maxConcurrent)

	return func(next Service[Req, Res]) Service[Req, Res] {
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				return next.Call(ctx, req)
			case <-ctx.Done():
				var zero Res
				return zero, ctx.Err()
			}
		})
	}
}

// MetricsCollector records per-request metrics.
type MetricsCollector interface {
	RecordRequest(serviceName string, duration time.Duration, err error)
	RecordActiveRequests(serviceName string, delta int)
}

// MetricsLayer records duration, errors, and active request counts.
func MetricsLayer[Req any, Res any](collector MetricsCollector, serviceName string) Layer[Req, Res] {
	return func(next Service[Req, Res]) Service[Req, Res] {
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			collector.RecordActiveRequests(serviceName, 1)
			defer collector.RecordActiveRequests(serviceName, -1)

			start := time.Now()
			res, err := next.Call(ctx, req)
			duration := time.Since(start)

			collector.RecordRequest(serviceName, duration, err)

			return res, err
		})
	}
}

// Validator validates request values before service execution.
type Validator[Req any] interface {
	Validate(ctx context.Context, req Req) error
}

// ValidatorFunc adapts a function to the Validator interface.
type ValidatorFunc[Req any] func(ctx context.Context, req Req) error

// Validate runs the wrapped validator function.
func (f ValidatorFunc[Req]) Validate(ctx context.Context, req Req) error {
	return f(ctx, req)
}

// ErrValidation wraps a validation failure.
type ErrValidation struct {
	Err error
}

func (e ErrValidation) Error() string {
	return "validation error: " + e.Err.Error()
}

func (e ErrValidation) Unwrap() error {
	return e.Err
}

// ValidationLayer validates requests before calling the next service.
func ValidationLayer[Req any, Res any](validator Validator[Req]) Layer[Req, Res] {
	return func(next Service[Req, Res]) Service[Req, Res] {
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			if err := validator.Validate(ctx, req); err != nil {
				var zero Res
				return zero, ErrValidation{Err: err}
			}
			return next.Call(ctx, req)
		})
	}
}
