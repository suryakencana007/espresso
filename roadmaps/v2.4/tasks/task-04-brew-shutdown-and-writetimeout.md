# Task 4: BrewContext Drain + SSE-vs-WriteTimeout + cmd/example Fixes

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 1 day
**Dependencies:** None (but Task 3 makes SSE-through-middleware possible; independent files)

> **Status: ✅ Shipped 2026-07-05 (v2.4.0).** Delivered via #62 + #66 + #67 — BrewContext drains in-flight requests on cancel (04a); cmd/example use BrewContext (04c); SSE clears WriteTimeout per-connection (04b).

## Context

Three related shutdown/lifecycle defects, bundled because they share `server.go`, `sse.go`, and `cmd/example/`:

- **`BrewContext` runs graceful shutdown with an already-canceled context (D1).** `BrewContext` waits on `<-ctx.Done()` (`server.go:159-163`) and then calls `r.gracefulShutdown(ctx, srv, cfg.ShutdownTimeout)` (`server.go:166`) passing the **same canceled context** as parent. `context.WithTimeout(parentCtx, timeout)` (`server.go:179`) yields an already-expired shutdown context: (1) every `OnShutdown` hook receives a dead context — ctx-aware cleanup fails immediately and `runHookSafely` mislabels it "shutdown hook timed out"; (2) `srv.Shutdown(shutdownCtx)` (`server.go:194`) returns `context.Canceled` immediately without waiting for in-flight requests. `Brew()` is unaffected (passes `context.Background()`). All existing `TestShutdown_Hooks*` tests bypass the bug by calling `gracefulShutdown` with `context.Background()` directly.

- **Default server `WriteTimeout` (10s) kills every SSE stream (D2).** `defaultConfig` sets `WriteTimeout: 10s` (`server.go:31`) and both `Brew` and `BrewContext` apply it to `http.Server` (`server.go:94, 145`). `net/http` sets the connection write deadline to `now+WriteTimeout` when the request is read, covering the entire response; SSE responses are not hijacked, so every `Send` fails with an i/o timeout ~10s after the stream opens, regardless of `WithKeepAlive`. `cmd/example/sse/main.go:52` serves an infinite ticker stream via `router.Brew(espresso.WithAddr(":8080"))` with defaults — it dies at 10s. All SSE tests use `httptest.NewServer`, which sets no `WriteTimeout`, so CI never sees it.

- **`cmd/example/sse` and `cmd/example/websocket` race their own shutdown (D3).** Both `cmd/example/sse/main.go:51-57` and `cmd/example/websocket/main.go:53-59` run `router.Brew(...)` in a goroutine while main blocks on its own `signal.NotifyContext` for the same signals `Brew` traps. Main returns and the process exits while `Brew`'s `gracefulShutdown` (OnShutdown hooks, SSE frames, WS 1001 close, `srv.Shutdown`) is still running. The comment "Router handles graceful shutdown internally" is made false by the surrounding code.

## Acceptance Criteria

- [x] `BrewContext` with an in-flight 500ms request, `WithShutdownTimeout(5s)`, and ctx cancelled: the request drains to completion; `BrewContext` returns after ≥500ms (not ~0ms); `OnShutdown` hooks see a live ctx with a future deadline.
- [x] An SSE stream served via `router.Brew()` with default settings survives past 10s (no `WriteTimeout` kill); the fix uses `http.NewResponseController(w).SetWriteDeadline(...)` per write in `sse.go`, not a `WriteTimeout` default change.
- [x] The default `WriteTimeout=10s` remains in place for non-stream routes (its DoS-protective purpose is correct).
- [x] `cmd/example/sse/main.go` and `cmd/example/websocket/main.go` no longer race the framework's graceful shutdown — either use plain `router.Brew(...)` (blocks on signals internally) or `router.BrewContext(ctx, ...)` with `signal.NotifyContext` and no separate goroutine.
- [x] No public API signature change on `Brew`/`BrewContext`; `SetWriteDeadline` usage in `sse.go` is a hidden implementation detail.

## Technical Approach

### Step 4.1 — Reproduce the drain skip

```go
func TestBrewContext_DrainsInFlightRequest(t *testing.T) {
    r := espresso.Portafilter()
    r.Get("/slow", espresso.Ristretto(func(ctx context.Context) espresso.Text {
        time.Sleep(500 * time.Millisecond)
        return espresso.Text{Body: "done"}
    }))
    ctx, cancel := context.WithCancel(context.Background())
    started := make(chan struct{})
    done := make(chan error)
    go func() { close(started); done <- r.BrewContext(ctx, espresso.WithAddr(":0"), espresso.WithShutdownTimeout(5*time.Second)) }()
    <-started
    time.Sleep(50 * time.Millisecond) // let server listen
    // fire request, then cancel while it's in flight
    // assert request body == "done" AND BrewContext returned after ≥500ms
}
```

Confirm pre-fix: request does not drain (client sees connection reset or truncated body), `BrewContext` returns in ~0ms.

### Step 4.2 — Fix BrewContext with context.WithoutCancel

Change `server.go:166`:

```go
// Before:
r.gracefulShutdown(ctx, srv, cfg.ShutdownTimeout)
// After:
r.gracefulShutdown(context.WithoutCancel(ctx), srv, cfg.ShutdownTimeout)
```

`context.WithoutCancel` (Go 1.21+; module go.mod pins 1.23) returns a context that inherits values but not cancellation. `gracefulShutdown` then derives `WithTimeout` from a fresh parent, and `srv.Shutdown` gets a live ctx with the full `ShutdownTimeout` to drain in-flight requests. `OnShutdown` hooks also receive that live ctx.

### Step 4.3 — Reproduce and fix the SSE WriteTimeout kill

```go
func TestSSE_SurvivesDefaultWriteTimeout(t *testing.T) {
    // Use a real *http.Server (not httptest) so WriteTimeout applies.
    // Start server with defaultConfig's WriteTimeout=10s (or shortened to 2s for test speed).
    // Open an SSE stream. Send an event at t=0, another at t=(WriteTimeout+1s).
    // Post-fix: second Send succeeds. Pre-fix: connection is dead by then.
}
```

Fix: in `SSEStream.Send`, `SSEStream.Comment`, and `SSEStream.SetRetry`, call `http.NewResponseController(s.w).SetWriteDeadline(time.Time{})` (zero = no deadline) once at stream creation, and re-set a fresh short deadline before each write. Two shapes:

- **(a) Disable WriteTimeout at stream open**: `NewResponseController(w).SetWriteDeadline(time.Time{})` in `serveStream` after headers commit; simplest.
- **(b) Refresh per-write deadline**: before every `Fprintf`, `SetWriteDeadline(time.Now().Add(cfg.writeDeadline))` where `writeDeadline` is a configurable option (default 10s inherited from the server's).

Prefer (a): SSE streams are inherently long-lived; a per-write deadline would false-positive on slow clients (which is what keepalives are for). Docstring the choice.

Requires `statusRecorder` from Task 3 to forward `Unwrap()` for `ResponseController` to walk through. Order Task 3 before Task 4 or land them together.

### Step 4.4 — Fix cmd/example

Rewrite `cmd/example/sse/main.go:51-57` and `cmd/example/websocket/main.go:53-59` to the plain-Brew pattern:

```go
// Before (broken):
go func() { router.Brew(espresso.WithAddr(":8080")) }()
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
<-ctx.Done()

// After — option A (simplest):
router.Brew(espresso.WithAddr(":8080")) // blocks on signals internally

// After — option B (explicit ctx):
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
if err := router.BrewContext(ctx, espresso.WithAddr(":8080")); err != nil { log.Fatal(err) }
```

Use option A for `sse/main.go` and `websocket/main.go` — matches the documented "Router handles graceful shutdown internally" comment they carry.

## Tests Required

- `TestBrewContext_DrainsInFlightRequest`: pre-fix ~0ms, post-fix ≥500ms, response body intact.
- `TestBrewContext_OnShutdownHookSeesLiveContext`: hook receives ctx with `ctx.Err() == nil` and a future `Deadline`.
- `TestSSE_SurvivesDefaultWriteTimeout`: SSE stream Sends survive past `WriteTimeout` when served via a real `http.Server` with defaultConfig.
- `TestSSE_WriteTimeoutStillProtectsNonStreamRoutes`: a plain Ristretto route with a handler that hangs longer than `WriteTimeout` is still killed (the DoS-protective default is preserved for non-stream routes).
- Manual smoke: `go run ./cmd/example/sse` and `go run ./cmd/example/websocket`, connect a client, Ctrl-C, verify clean shutdown.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `go test -race ./... -count=2` clean.
- [x] `go build ./cmd/example/...` clean; smoke-run of both examples exits cleanly on SIGINT.
- [x] `golangci-lint run ./...` clean.
- [x] CI's `Test (race)` job green on the PR.
- [x] CHANGELOG `[Unreleased]` entry under `Fixed`: `BrewContext` now correctly drains in-flight requests on ctx cancellation; SSE streams survive the default `WriteTimeout` via per-stream `SetWriteDeadline` clearing; `cmd/example/sse` and `cmd/example/websocket` no longer race their own graceful shutdown.
- [x] Migration note (Task 12): SSE now works out of the box on `router.Brew()` defaults; users who worked around the previous behavior via `WithWriteTimeout(0)` may remove that.
- [x] No public API signature changed on `Brew`/`BrewContext`.
