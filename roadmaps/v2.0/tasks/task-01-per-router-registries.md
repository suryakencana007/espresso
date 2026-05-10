# Task 1: Per-Router Stream Registries

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 3-4 days
**Dependencies:** None (can start immediately)


> **Status: ✅ Shipped 2026-05-10.** Delivered via #21.

## Context

In v1.3, open WebSocket connections and SSE streams are tracked in package-level globals:

```go
// websocket.go
var defaultRegistry = newWSRegistry()

// sse.go
var defaultSSERegistry = newSSERegistry()
```

Graceful shutdown iterates these globals to close all active streams before `srv.Shutdown()`. This works for the single-router-per-process case (the vast majority of users) but leaks between routers when two `Portafilter()` instances run in the same process:

- Test suites that spin up multiple routers see cross-test registry pollution (observed during v1.3 development — `TestShutdown_WebSocketsClosed` flaked until registry cleanup was tightened).
- A library that embeds Espresso and wants to shut down *its* router without touching a sibling router cannot.
- Multi-tenant hosts that partition routers per tenant have no way to drain one tenant's streams without draining every tenant's.

v1.3's analysis flagged this as v2.0 scope. This task delivers.

## Acceptance Criteria

- [x] `defaultRegistry` and `defaultSSERegistry` package variables are **deleted**.
- [x] Each `*Router` owns a `*wsRegistry` and `*sseStreamRegistry` at construction time.
- [x] WebSocket handler wrappers (`WebSocket[T]`, `WebSocketSimple`) register connections against the owning router's registry.
- [x] SSE handler wrappers (`Stream[T]`, `StreamSimple`) register streams against the owning router's registry.
- [x] `gracefulShutdown` drains the router's registries, not globals.
- [x] Two routers in the same process shut down independently — closing router A does **not** close router B's streams.
- [x] All existing v1.3 tests pass unmodified **unless** they directly reached into `defaultRegistry` (those must migrate to the per-router API).

## Technical Approach

### Step 1.1 — Move Registry Ownership to Router

In `router.go`, add:

```go
type Router struct {
    // ... existing fields
    wsReg  *wsRegistry
    sseReg *sseStreamRegistry
}

func Portafilter() *Router {
    return &Router{
        // ... existing init
        wsReg:  newWSRegistry(),
        sseReg: newSSERegistry(),
    }
}
```

### Step 1.2 — Plumb Registry Through Handler Wrappers

The wrappers currently look like:

```go
func WebSocketSimple(h func(context.Context, *WS) error, opts ...WSOption) http.HandlerFunc {
    // ...
    return func(w http.ResponseWriter, r *http.Request) {
        // ...
        defaultRegistry.add(ws)
        // ...
    }
}
```

Two viable refactors, pick one and document it:

**Option A — Router-level method:**

```go
// On *Router:
func (r *Router) WebSocketSimple(h func(ctx context.Context, ws *WS) error, opts ...WSOption) http.HandlerFunc {
    return wrapWebSocketSimple(r.wsReg, h, opts...)
}
```

Pro: explicit, type-safe. Con: users must call `router.WebSocketSimple(...)` instead of `espresso.WebSocketSimple(...)`; breaks every existing call site.

**Option B — Context-injected registry:**

Router's `ServeHTTP` injects a pointer to the registry into `r.Context()`, handler wrappers read it out:

```go
type registryKey struct{}

// Router.ServeHTTP:
ctx := context.WithValue(r.Context(), registryKey{}, routerRegs{ws: r.wsReg, sse: r.sseReg})

// Wrapper:
regs := ctx.Value(registryKey{}).(routerRegs)
regs.ws.add(ws)
```

Pro: existing call sites (`espresso.WebSocketSimple(h)`) keep working. Con: one extra context lookup per request, and registry lookup at handler-time fails gracefully (panic vs silent skip must be decided).

**Recommended:** Option B. Keeps the migration small and the ergonomics of the package-level wrapper. Benchmark before and after to confirm the context lookup is negligible against actual request cost.

### Step 1.3 — Graceful Shutdown

Update `gracefulShutdown` in `server.go`:

```go
func (r *Router) gracefulShutdown(ctx context.Context, srv *http.Server, timeout time.Duration) {
    // ... hooks

    r.sseReg.closeAll("server shutting down")
    r.wsReg.closeAll(CloseGoingAway, "server shutting down")

    // ... srv.Shutdown
}
```

Remove the `if defaultRegistry != nil` guard; `r.wsReg` is always non-nil by construction.

### Step 1.4 — Test Multi-Router Isolation

New test in `server_test.go`:

```go
func TestShutdown_MultiRouter_Isolation(t *testing.T) {
    // Spin up routerA and routerB on separate httptest.Servers
    // Open a WebSocket against each
    // Call routerA.gracefulShutdown(...)
    // Assert routerA's WS received 1001, routerB's WS is still alive
}
```

## Tests Required

- Update any test that uses `defaultRegistry.len()` to use `router.wsReg.len()` via a test helper.
- Add the multi-router isolation test above.
- Add an SSE equivalent of the multi-router isolation test.
- Run `go test -race -count=3` to flush out any residual global-state assumptions.

## Breaking Changes

- `defaultRegistry` and `defaultSSERegistry` are removed. Any external caller that referenced them breaks at compile time.
- If Option A is chosen: `WebSocket`, `WebSocketSimple`, `Stream`, `StreamSimple` move from package-level functions to `*Router` methods.
- If Option B is chosen: no call-site changes. The registry move is internal.

Migration-guide entry required regardless of option chosen.

## Definition of Done

- All acceptance criteria ticked
- `go test ./... -race` passes
- `golangci-lint run ./...` clean
- Migration recipe added to `docs/migration-v1-to-v2.md` (Task 6 collects these)
- CHANGELOG `[Unreleased]` entry under `Changed` / `Breaking`
