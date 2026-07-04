# Task 7: Circuit Breaker — Four State-Machine Defects

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 1 day
**Dependencies:** Task 1 (shares `middleware/service/layer.go`)

## Context

The audit reproduced four defects in `CircuitBreakerLayer` (`middleware/service/layer.go:222-274`), three verified with live tests:

- **D1 — failures count over process lifetime.** In `StateClosed`, successes never reset the failure counter — `failures` only resets on the `HalfOpen→Closed` transition (`layer.go:264-267`). So `FailureThreshold=5` means the circuit opens after 5 failures over the entire process lifetime (5 transient errors across days of 99.99% success), not 5 consecutive/windowed failures. Reproduced: 5 failures across 102 calls at 99% success opens the circuit.
- **D2 — probe success not counted.** The goroutine that performs the `Open→HalfOpen` transition captured `currentState=StateOpen` before transitioning (`layer.go:227-241`), so when its probe succeeds, `if currentState == StateHalfOpen` (`layer.go:261`) is false and the first probe's success is never counted toward `SuccessThreshold`. Reproduced: a successful probe with `SuccessThreshold=1` failed to close the circuit.
- **D3 — success path mutates on stale local state.** The success path increments `successes` and closes the circuit based on the stale local `currentState` without re-checking under the write lock (`layer.go:261-269`). A concurrent failure can reopen the circuit and a straggling success can immediately force it Closed. Verified by inspection; live repro is timing-sensitive.
- **D4 — half-open admits unlimited concurrent probes.** Every request that observes `HalfOpen` passes through as a probe (no counter/semaphore capping concurrency), defeating the purpose of probing. Reproduced.

Also secondary: there are **two parallel `CircuitBreakerError` types** (`middleware/service/layer.go` and `error.go:21`) with a hand-rolled `errorsAs` (`layer.go:205-216`) — `translateLayerError` uses one, mostly. Consolidate to one type while touching the file.

## Acceptance Criteria

- [ ] `failures` resets to 0 on any successful call while `StateClosed` (or use a rolling window over N most-recent calls).
- [ ] The first probe's success (after `Open→HalfOpen` transition) is counted toward `SuccessThreshold`.
- [ ] The success path re-reads state under the write lock before mutating, so a concurrent state change is observed and the mutation is skipped or adjusted accordingly.
- [ ] Half-open admits at most `HalfOpenMaxProbes` (default 1) concurrent probes; extra requests observing `HalfOpen` return `ErrCircuitBreakerOpen`-equivalent immediately.
- [ ] Only one `CircuitBreakerError` type exists in the module; the hand-rolled `errorsAs` is removed in favor of `errors.As` where possible.
- [ ] No breaking change to `CircuitBreaker(...)` `LayerConfig` constructor signature; new fields on the config (like `HalfOpenMaxProbes`) are additive with sensible defaults.

## Technical Approach

### Step 7.1 — Reproduce each defect

Four regression tests, each shaped like the audit's repro:

```go
// D1: 5 failures over 102 calls at 99% success opens the circuit.
func TestCircuitBreaker_DoesNotOpenOnNonConsecutiveFailures(t *testing.T) {
    // Configure FailureThreshold=5. Fire 102 calls: fail 1 in 20.
    // Assert the circuit stays Closed (successes reset the failure count).
}

// D2: transitioning probe's success is counted.
func TestCircuitBreaker_TransitioningProbeSuccessCounts(t *testing.T) {
    // Configure SuccessThreshold=1. Trip the circuit, wait past OpenTimeout,
    // fire one successful call. Assert circuit closes on that single call.
}

// D3: race between success and concurrent failure.
func TestCircuitBreaker_SuccessRespectsConcurrentStateChange(t *testing.T) {
    // Set up half-open state; fire success and concurrent failure via goroutines
    // under -race. Assert no invariant violations (state transitions are legal).
}

// D4: half-open admits at most HalfOpenMaxProbes.
func TestCircuitBreaker_HalfOpenLimitsConcurrentProbes(t *testing.T) {
    // Configure HalfOpenMaxProbes=1. Trip circuit, wait for HalfOpen, fire 10
    // concurrent requests. Assert 1 passes through, 9 short-circuit.
}
```

### Step 7.2 — Fix D1: reset failures on Closed success

In the success path within `StateClosed`, set `failures = 0`. Alternatively, adopt a rolling window (e.g. count failures in the last N calls). Prefer the simple reset — matches the mental model most users have of a circuit breaker and matches Netflix Hystrix's default semantics.

### Step 7.3 — Fix D2 + D3: re-read state under the write lock

Re-check the current state under the write lock at the point of mutation, not at the goroutine's entry:

```go
func (cb *circuitBreaker) recordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    switch cb.state { // read fresh under lock
    case StateClosed:
        cb.failures = 0 // D1
    case StateHalfOpen:
        cb.successes++
        if cb.successes >= cb.successThreshold {
            cb.state = StateClosed
            cb.failures = 0
            cb.successes = 0
        }
    case StateOpen:
        // A success arrived after the state reverted to Open — no-op.
    }
}
```

This eliminates the stale `currentState` capture at the goroutine entry (D2) and makes the success path race-safe (D3).

### Step 7.4 — Fix D4: cap half-open concurrency

Add an atomic counter (or a semaphore channel) for in-flight probes:

```go
type circuitBreaker struct {
    // ...
    halfOpenInFlight atomic.Int32
    halfOpenMaxProbes int32 // configurable, default 1
}

// On call() when state == HalfOpen:
if cb.halfOpenInFlight.Add(1) > cb.halfOpenMaxProbes {
    cb.halfOpenInFlight.Add(-1)
    return ErrCircuitBreakerOpen // or a distinct HalfOpenBusy sentinel
}
defer cb.halfOpenInFlight.Add(-1)
```

Add `HalfOpenMaxProbes` to the `LayerConfig` constructor `CircuitBreaker(...)` with default 1.

### Step 7.5 — Consolidate the two CircuitBreakerError types

Delete the `error.go:21` duplicate if it is unused externally (grep first — if part of a public API surface, deprecate but retain). Route `translateLayerError` (`error.go:250`) through `errors.As` against the single canonical type. Remove `errorsAs` (`layer.go:205-216`).

## Tests Required

- `TestCircuitBreaker_DoesNotOpenOnNonConsecutiveFailures` (D1).
- `TestCircuitBreaker_TransitioningProbeSuccessCounts` (D2).
- `TestCircuitBreaker_SuccessRespectsConcurrentStateChange` (D3, under `-race`).
- `TestCircuitBreaker_HalfOpenLimitsConcurrentProbes` (D4).
- `TestCircuitBreaker_ErrorTranslationUnified`: `CircuitBreakerError` from a layer translates to 503 via `translateLayerError`, one code path only.
- Run with `-race -count=2`.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -race ./middleware/service/... ./... -count=2` clean.
- [ ] `golangci-lint run ./...` clean.
- [ ] CI's `Test (race)` job green on the PR.
- [ ] CHANGELOG `[Unreleased]` entry under `Fixed`: four `CircuitBreakerLayer` correctness defects — failure count now resets on successful calls while Closed; the transitioning probe's success is counted; state-machine mutations re-check state under the lock; half-open concurrency is bounded (default `HalfOpenMaxProbes=1`). Under `Added` (if applicable): `HalfOpenMaxProbes` on the `LayerConfig` constructor.
- [ ] Migration note (Task 12): a service that was tripping the circuit under long-running low-rate transient failures will now stay closed; the previous behavior was cumulative over process lifetime.
- [ ] No public API signature changed on `CircuitBreaker(...)` (additive field on config only).
