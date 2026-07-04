# Task 1: TimeoutLayer + Pool Data Race

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 1.5 days
**Dependencies:** None

## Context

The 2026-07-02 audit reproduced a `-race`-detected data race in the flagship `TimeoutLayer` when combined with `WithLayersTyped`'s pooled request struct. `TimeoutLayer` (`middleware/service/layer.go:50-73`) spawns a goroutine running `next.Call(ctx, req)` and returns on `ctx.Done()`, leaving that goroutine holding a reference to `req`. The outer handler from `applyLayersAndConvert` (`withlayers.go:414-435`) then runs its deferred `resetReq(req); pool.Put(req)` unconditionally (`withlayers.go:421-424`); the next request `pool.Get()`s the same struct and `Extract()` writes into it while the abandoned goroutine may still be reading it.

Reproduced deterministically: `go test -race` reports `WARNING: DATA RACE — Write by (*slowReq).Extract at withlayers.go:428 / Previous read by TimeoutLayer goroutine created at middleware/service/layer.go:62`. `WithLayersTyped+Timeout` is the documented pairing (`CLAUDE.md` lists Timeout as a flagship service layer), so ordinary user code hits this whenever a handler outlives its timeout — exactly the scenario Timeout exists for. Cross-request data corruption is possible; users see intermittent handler failures with body data belonging to a prior request.

Secondary: on parent-context cancellation `TimeoutLayer` returns `context.Canceled`, which `translateLayerError` (`error.go:250`) does not map — it becomes a 500 rather than a client-disconnect classification. Address in the same PR.

## Acceptance Criteria

- [ ] `TimeoutLayer` + `WithLayersTyped` + a handler that outlives the deadline no longer produces `WARNING: DATA RACE` under `-race`, verified by a regression test shaped like the audit's repro.
- [ ] The regression test reliably fails on the pre-fix commit (spot-check via a temporary revert during Task 11 verification).
- [ ] `core.go`'s `Resettable` godoc documents that pooled request structs must not be retained past handler return.
- [ ] `context.Canceled` from `TimeoutLayer` maps to a client-disconnect status rather than 500 via `translateLayerError` (either propagate the classification or return a sentinel).
- [ ] No public API signature change; no breaking change to `Timeout(d time.Duration)`'s `LayerConfig` constructor.

## Technical Approach

### Step 1.1 — Reproduce and lock the race

Add a regression test that mirrors the audit's shape exactly, in a new file `middleware_service_timeout_race_test.go` (or extend an existing test file):

```go
type slowReq struct{ Name string }
func (r *slowReq) Extract(req *http.Request) error {
    r.Name = req.URL.Query().Get("name")
    return nil
}
// Handler that reads req in a loop for 80ms — outlives the 10ms timeout.
func slowHandler(ctx context.Context, req *slowReq) (espresso.Text, error) {
    end := time.Now().Add(80 * time.Millisecond)
    for time.Now().Before(end) {
        _ = req.Name  // read after timeout return
        time.Sleep(1 * time.Millisecond)
    }
    return espresso.Text{Body: "ok"}, nil
}
// TestTimeoutLayer_NoPoolRace: fire 50 sequential requests with Timeout(10ms).
// Verify go test -race prints WARNING: DATA RACE on the pre-fix code.
```

Confirm the test fails under `-race` before proceeding.

### Step 1.2 — Skip pool.Put on abandonment

In `applyLayersAndConvert` (`withlayers.go:405-424`), detect when the wrapped call returned a cancellation/timeout indicating an abandoned goroutine, and skip the pool return. Two viable shapes:

- **(a) Sentinel from `TimeoutLayer`**: `TimeoutLayer` wraps its return with a sentinel type (e.g. `type timeoutAbandoned struct{ err error }` where `err` is `ctx.Err()`); `applyLayersAndConvert` type-checks and skips `pool.Put` if seen, unwrapping the underlying error for translation.
- **(b) Post-hoc check**: `applyLayersAndConvert` checks `errors.Is(err, context.DeadlineExceeded)` (or `context.Canceled`) after the wrapped call returns. Simpler; slight over-broad — non-timeout deadline errors also skip pooling.

Prefer (a): explicit intent, no false positives, and keeps the pool return path fast for the happy case.

### Step 1.3 — Document the retention rule

Add to `core.go`'s `Resettable` godoc (near line where `Resettable` is declared):

> Pooled request structs must not be retained past handler return. The framework pools request structs for reuse; a goroutine that outlives its handler and continues to read the request pointer races against the next request's `Extract`. If a service layer abandons a goroutine (e.g. `TimeoutLayer` on deadline), the framework's pooling machinery detects this and skips the pool return for that request, leaking one struct to GC rather than reusing it. Consequences for user code: do not spawn a goroutine that closes over the request pointer and outlives the handler function.

### Step 1.4 — Fix the context.Canceled → 500 secondary

`translateLayerError` (`error.go:250`) should map `context.Canceled` to a client-disconnect classification (`499` is non-standard; `503 SERVICE_UNAVAILABLE` matches `context.DeadlineExceeded`'s current mapping post-v2.2). Verify against `TestWithLayersTyped_WithTimeout` — the v2.2 status-lock test — to ensure the update does not regress that lock.

## Tests Required

- `TestTimeoutLayer_NoPoolRace`: the audit's repro shape; must fail under `-race` on pre-fix code, pass after.
- `TestTimeoutLayer_PoolStructNotReused`: after a timeout fires, assert the next request's request-struct is a different pointer (or that the abandoned struct's fields are untouched — the framework may keep pooling but under a per-abandonment fresh allocation).
- `TestTimeoutLayer_ParentCancelMapsToServiceUnavailable`: parent ctx cancel produces 503 with the canonical envelope, not 500.
- Run with `-race -count=2`.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -race ./... -count=2` clean.
- [ ] `golangci-lint run ./...` clean.
- [ ] `govulncheck ./...` clean.
- [ ] CI's `Test (race)` job green on the PR.
- [ ] CHANGELOG `[Unreleased]` entry under `Fixed`: `TimeoutLayer` no longer races the request-struct pool on abandoned handlers; `context.Canceled` from a service layer maps to 503 rather than 500.
- [ ] No public API signature changed.
