package servicemiddleware

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// abandonedByTimeoutErr signals that TimeoutLayer returned from the select
// on ctx.Done while a handler goroutine is still executing with the request
// pointer. Callers that own pooled request storage (root package's
// applyLayersAndConvert) must detect this signal and NOT return the request
// struct to the pool — the abandoned goroutine may still be reading it,
// and re-pooling the struct would race the next request's Extract into it.
// The wrapped error is the original ctx.Err(): DeadlineExceeded when the
// timeout fired, or Canceled when the caller's context was canceled.
type abandonedByTimeoutErr struct{ err error }

func (e *abandonedByTimeoutErr) Error() string { return e.err.Error() }
func (e *abandonedByTimeoutErr) Unwrap() error { return e.err }

// IsAbandonedByTimeout reports whether err was produced by TimeoutLayer
// after abandoning a still-running handler goroutine. The typed handler
// wrapper uses this to skip returning the request struct to sync.Pool,
// preventing a data race between the abandoned goroutine and the next
// request's Extract.
func IsAbandonedByTimeout(err error) bool {
	var target *abandonedByTimeoutErr
	return errors.As(err, &target)
}

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
//
// When the deadline fires (or the caller's context is canceled), the layer
// returns from its select while the inner goroutine may still be executing.
// The returned error wraps the underlying context.Err() in an
// abandonedByTimeoutErr sentinel so callers that own pooled request storage
// can detect the abandonment and skip returning the request to the pool —
// see IsAbandonedByTimeout. Callers that only care about the semantic error
// (i.e. status-code mapping via translateLayerError) can rely on
// errors.Is(err, context.DeadlineExceeded) / errors.Is(err, context.Canceled)
// working normally through the sentinel's Unwrap.
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
				return zero, &abandonedByTimeoutErr{err: ctx.Err()}
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
//
// HalfOpenMaxProbes bounds the number of concurrent requests that pass
// through when the breaker is HalfOpen. Extra requests receive an immediate
// CircuitBreakerError instead of reaching the wrapped service — the point of
// half-open is to probe with LIMITED traffic, not to unleash the pending
// backlog. Default is 1; zero or negative values are normalized to 1 at
// layer-construction time so misconfiguration cannot open the floodgate.
type CircuitBreakerConfig struct {
	ServiceName       string
	FailureThreshold  int
	Timeout           time.Duration
	SuccessThreshold  int
	HalfOpenMaxProbes int
}

// DefaultCircuitBreakerConfig provides sensible defaults.
var DefaultCircuitBreakerConfig = CircuitBreakerConfig{
	FailureThreshold:  5,
	Timeout:           30 * time.Second,
	SuccessThreshold:  3,
	HalfOpenMaxProbes: 1,
}

// CircuitBreakerState stores mutable runtime circuit breaker state.
//
// halfOpenInFlight tracks probe concurrency in HalfOpen and is bounded by
// CircuitBreakerConfig.HalfOpenMaxProbes. It resets on every Open→HalfOpen
// transition; leftover counts from a prior half-open attempt do not carry
// forward because in-flight only matters while in HalfOpen.
type CircuitBreakerState struct {
	mu               sync.RWMutex
	state            CircuitState
	failures         int
	successes        int
	lastFailTime     time.Time
	halfOpenInFlight atomic.Int64
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

// IsCircuitBreakerError reports whether err is a *CircuitBreakerError.
func IsCircuitBreakerError(err error) bool {
	var cbErr *CircuitBreakerError
	return errors.As(err, &cbErr)
}

// ErrCircuitBreakerOpen is a sentinel error for open-circuit rejection.
var ErrCircuitBreakerOpen = NewCircuitBreakerError("", StateOpen, "circuit breaker is open")

// CircuitBreakerLayer applies circuit breaker protection around a service.
//
// State-machine invariants (fixed in v2.4 task-07):
//   - Successes while StateClosed reset the failure counter; the threshold
//     is consecutive-ish, not process-lifetime cumulative.
//   - The transitioning probe (the request that flips Open→HalfOpen) has
//     its result counted: the outcome-recording path re-reads state under
//     the write lock rather than trusting a stale currentState captured on
//     entry.
//   - Success and failure paths re-check state under the write lock before
//     mutating; a concurrent state change is observed and the mutation is
//     tailored to the observed state (Closed/HalfOpen/Open).
//   - Half-open admits at most HalfOpenMaxProbes concurrent probes; extras
//     receive an immediate CircuitBreakerError with State=StateHalfOpen and
//     never reach the wrapped service.
func CircuitBreakerLayer[Req any, Res any](config CircuitBreakerConfig) Layer[Req, Res] {
	// Defensive default so misconfiguration cannot admit unlimited probes.
	if config.HalfOpenMaxProbes <= 0 {
		config.HalfOpenMaxProbes = 1
	}

	state := &CircuitBreakerState{state: StateClosed}

	return func(next Service[Req, Res]) Service[Req, Res] {
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			state.mu.RLock()
			currentState := state.state
			state.mu.RUnlock()

			// Entry gate: reject fast when Open (unless the timeout has
			// elapsed, in which case transition to HalfOpen), or acquire
			// an in-flight probe slot when HalfOpen (D4 — bounded probes).
			switch currentState {
			case StateOpen:
				state.mu.RLock()
				lastFail := state.lastFailTime
				state.mu.RUnlock()

				if time.Since(lastFail) > config.Timeout {
					state.mu.Lock()
					state.state = StateHalfOpen
					state.successes = 0
					state.halfOpenInFlight.Store(0)
					state.mu.Unlock()
					// Fall through to acquire a probe slot for THIS request.
					if state.halfOpenInFlight.Add(1) > int64(config.HalfOpenMaxProbes) {
						state.halfOpenInFlight.Add(-1)
						var zero Res
						return zero, NewCircuitBreakerError(config.ServiceName, StateHalfOpen, "half-open probe capacity exceeded")
					}
					defer state.halfOpenInFlight.Add(-1)
				} else {
					var zero Res
					return zero, NewCircuitBreakerError(config.ServiceName, StateOpen, "circuit breaker is open")
				}
			case StateHalfOpen:
				if state.halfOpenInFlight.Add(1) > int64(config.HalfOpenMaxProbes) {
					state.halfOpenInFlight.Add(-1)
					var zero Res
					return zero, NewCircuitBreakerError(config.ServiceName, StateHalfOpen, "half-open probe capacity exceeded")
				}
				defer state.halfOpenInFlight.Add(-1)
			}

			res, err := next.Call(ctx, req)
			if err != nil {
				recordCircuitFailure(state, config)
				return res, err
			}
			recordCircuitSuccess(state, config)
			return res, nil
		})
	}
}

// recordCircuitFailure applies a failure to the circuit state under the
// write lock (D3). Behavior is state-dependent, so re-reading the state at
// the mutation point is load-bearing — a concurrent state change since the
// call entered is observed and handled correctly:
//   - Closed: increment failures; if threshold met, transition to Open.
//   - HalfOpen: any probe failure reverts to Open immediately (reset counters).
//   - Open: someone else already reopened; slide the lastFailTime window.
func recordCircuitFailure(state *CircuitBreakerState, config CircuitBreakerConfig) {
	state.mu.Lock()
	defer state.mu.Unlock()
	switch state.state {
	case StateClosed:
		state.failures++
		state.lastFailTime = time.Now()
		if state.failures >= config.FailureThreshold {
			state.state = StateOpen
		}
	case StateHalfOpen:
		state.state = StateOpen
		state.lastFailTime = time.Now()
		state.failures = 1
		state.successes = 0
	case StateOpen:
		state.lastFailTime = time.Now()
	}
}

// recordCircuitSuccess applies a success to the circuit state under the
// write lock (D1 + D2 + D3). Rules:
//   - Closed: reset failures — the counter is consecutive-ish, not
//     process-lifetime cumulative (D1).
//   - HalfOpen: increment successes; if threshold met, close and reset
//     both counters. The transitioning probe's success is counted here
//     because we read state at the mutation point rather than at
//     goroutine entry (D2).
//   - Open: a concurrent failure reopened while this success was in
//     flight — leave state alone.
func recordCircuitSuccess(state *CircuitBreakerState, config CircuitBreakerConfig) {
	state.mu.Lock()
	defer state.mu.Unlock()
	switch state.state {
	case StateClosed:
		state.failures = 0
	case StateHalfOpen:
		state.successes++
		if state.successes >= config.SuccessThreshold {
			state.state = StateClosed
			state.failures = 0
			state.successes = 0
		}
	case StateOpen:
		// Stale probe result — no-op.
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
