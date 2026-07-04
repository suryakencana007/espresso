# Task 2: TokenBucketLimiter Refill Starvation

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 0.5 day
**Dependencies:** None

## Context

`TokenBucketLimiter` starves all traffic under sustained sub-second load — a defect audit-confirmed by a live repro (rate=100/s, cap=5, one request every 100ms for 3s admits **5 of 30** requests in both global and per-key modes; a drained bucket polled every 200ms admits 0 for 2s).

Both `allowGlobal` (`middleware/http/middleware.go:310-315`) and `allowPerKey` (`middleware.go:345-350`) compute:

```go
refill := int(elapsed.Seconds()) * l.rate / int(time.Second.Seconds())
```

and then unconditionally set `lastRefill = now`. `int(elapsed.Seconds())` truncates to whole seconds, so any request arriving <1s after the previous one computes `refill=0` yet still advances `lastRefill`. Under sustained traffic ≥1 req/sec on a key — the normal case for a busy service — `elapsed` never reaches 1s between calls, tokens are consumed down to 0 and never replenished, and the limiter rejects everything until traffic pauses for a full second.

The existing test (`middleware_test.go:397-420`) sleeps 1.1s once and masks the bug. This is the P0 with the smallest fix but the largest silent user impact: any production deployment running the token bucket at ≥1 req/s is effectively rate-limited to zero after the initial burst.

## Acceptance Criteria

- [ ] `TokenBucketLimiter` at `rate=100/s`, `cap=5`, requests every 100ms for 3s admits approximately `rate * duration` requests (~300, allowing ±5% for edge timing), not `cap` and then nothing.
- [ ] Same holds for `TokenBucketLimiterPerKey` under the same load pattern.
- [ ] Fractional-second refill is credited — a request 500ms after the previous one on `rate=10/s` credits ~5 tokens toward the bucket, not 0.
- [ ] `lastRefill` advances only by the amount of time actually credited to tokens (no free clock advance for un-credited fractional seconds).
- [ ] The existing 1.1s-sleep test still passes.
- [ ] No public API signature change.

## Technical Approach

### Step 2.1 — Reproduce the starvation

Add a regression test:

```go
func TestTokenBucketLimiter_SustainedSubSecondTraffic(t *testing.T) {
    lim := NewTokenBucketLimiter(100, 5) // rate=100/s, cap=5
    admitted := 0
    for range 30 {
        if lim.Allow() { admitted++ }
        time.Sleep(100 * time.Millisecond) // 30 requests over 3s = 10 rps
    }
    // Post-fix expectation: 100 rps allowed, 10 rps offered = ~30 admissions
    // Pre-fix behavior: exactly cap=5 admissions (regression lock)
    if admitted < 25 || admitted > 30 {
        t.Fatalf("expected ~30 admissions at rate=100/s, cap=5, got %d", admitted)
    }
}
```

Confirm this test fails with `admitted == 5` on pre-fix code.

### Step 2.2 — Fix the refill math (global + per-key)

Two viable shapes:

- **(a) Nanosecond math**: `refill := int(elapsed.Nanoseconds()) * l.rate / int(time.Second.Nanoseconds())`. Loses no precision.
- **(b) Advance `lastRefill` only by credited time**: keep whole-second math but do `lastRefill = lastRefill.Add(time.Duration(whole) * time.Second)` where `whole := int(elapsed.Seconds())`. Un-credited fractional seconds carry to the next call.

Prefer (a): simpler, monotonic, no accumulated drift. Apply to both `allowGlobal` (`middleware.go:306-320`) and `allowPerKey` (`middleware.go:337-358`).

### Step 2.3 — Keep the existing test green

The `1.1s sleep` test at `middleware_test.go:397-420` currently passes on the buggy code because 1.1s > 1s. Under the fix it must still pass — verify.

## Tests Required

- `TestTokenBucketLimiter_SustainedSubSecondTraffic` (global): ~300 of 300 admissions at 100 rps for 3s.
- `TestTokenBucketLimiterPerKey_SustainedSubSecondTraffic` (per-key): same load on a fixed key, same admission count.
- `TestTokenBucketLimiter_FractionalRefillCredited`: bucket drained, wait 500ms on rate=10/s, one request admitted (5 tokens > 1 request needed).
- `TestTokenBucketLimiter_ClockDoesNotAdvancePastCredit`: verify sequential fractional-second waits accumulate to a full refill on the third call rather than losing time.
- Run with `-race -count=2`.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -race ./middleware/http/... -count=2` clean.
- [ ] `golangci-lint run ./...` clean.
- [ ] CI's `Test (race)` job green on the PR.
- [ ] CHANGELOG `[Unreleased]` entry under `Fixed`: `TokenBucketLimiter` and `TokenBucketLimiterPerKey` correctly refill under sustained sub-second traffic; previously truncated fractional seconds to zero while advancing the refill clock, starving all traffic after the initial burst.
- [ ] No public API signature changed.
