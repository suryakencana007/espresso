# Task 2: Circuit breaker half-open probe-counter race

**Priority:** 🟡 P1 — hardening
**Estimated Effort:** 0.5 day
**Dependencies:** None (surfaced by Task 1's CI)

> **Status: ✅ Shipped 2026-11-15 (v2.5.0).** Delivered via #83, closing
> issue #82.

## Context

`CircuitBreakerConfig.HalfOpenMaxProbes` (added in v2.4 task-07, D4) is meant
to bound how many concurrent requests probe a recovering upstream. Under
concurrency the bound was not enforced.

The entry gate in `CircuitBreakerLayer` (`middleware/service/layer.go`) reads
`currentState` once, then in the `StateOpen` branch every goroutine that sees
the open timeout elapsed takes the write lock and **unconditionally** runs the
`Open → HalfOpen` transition, including `state.halfOpenInFlight.Store(0)`.
Two goroutines interleave as:

- A: lock → `HalfOpen`, `Store(0)` → unlock → `Add(1)` = 1 → **probe**
- B: lock → `HalfOpen`, `Store(0)` (resets A's 1 to 0) → unlock → `Add(1)` = 1
  → **probe**

So more than `HalfOpenMaxProbes` probes reach the upstream — the exact stampede
the limit exists to prevent.

CI surfaced it as an intermittent failure of
`TestCircuitBreaker_HalfOpenLimitsConcurrentProbes` on #79:

```
layer_test.go:505: expected 1 probe to reach the wrapped service, got 2 (pre-fix behavior: unbounded probes)
```

## Acceptance Criteria

- [x] Concurrent requests observing `StateOpen` with the timeout elapsed admit
      at most `HalfOpenMaxProbes` probes.
- [x] The atomic `halfOpenInFlight.Add(1) > max` check remains the single
      admission gate.
- [x] Exactly one goroutine owns the transition and the counter reset.
- [x] No change to `CircuitBreakerConfig` or any public surface.
- [x] A regression test that fails on the pre-fix revision.

## Technical Approach

### Step 2.1 — Re-check under the write lock

```go
state.mu.Lock()
if state.state == StateOpen && time.Since(state.lastFailTime) > config.Timeout {
	state.state = StateHalfOpen
	state.successes = 0
	state.halfOpenInFlight.Store(0)
}
state.mu.Unlock()
// fall through to the atomic probe-slot acquisition
```

The re-check means the first goroutine transitions and resets; everyone else
fails the guard and falls through to the atomic `Add`, which stays the only
gate. The `Store(0)` now provably happens before any peer's `Add(1)` (they
cannot pass the lock until this critical section ends).

### Step 2.2 — Harden the regression test

The v2.4 test ran a single 10-goroutine burst and tripped the race only once
per few hundred runs. The hardened test:

- wraps the burst in a 10-round loop, and
- releases all ten callers together behind a `start` channel barrier.

Validated by reverting `layer.go` to the pre-fix version and running the new
test once — it fails deterministically on `round 0`.

## Tests Required

- `TestCircuitBreaker_HalfOpenLimitsConcurrentProbes` (hardened) — must fail on
  the unfixed revision. Verified:
  `go test -race -run TestCircuitBreaker_HalfOpenLimitsConcurrentProbes -count=1 ./middleware/service`
  red pre-fix, `-count=30` green post-fix.

## Definition of Done

- [x] Full `go test -race -shuffle=on -count=1 ./...` green.
- [x] CHANGELOG `[2.5.0]` `Fixed` entry references #82.
- [x] Migration guide documents the observable behavior change.
