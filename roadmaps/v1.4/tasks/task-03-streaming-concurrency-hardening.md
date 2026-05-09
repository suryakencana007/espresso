# Task 3: Streaming Concurrency Hardening

**Priority:** 🟡 P1 — Hardening
**Estimated Effort:** 2 days
**Dependencies:** v1.3 long-lived integration tests (`tests/integration/longlived_test.go`)

## Context

The v1.3 long-lived tests caught real concurrency defects in the streaming and WebSocket paths that short-lived unit tests never reached:

1. **`WS.closed` plain-bool data race.** Two handler-end guards read `WS.closed` without holding the mutex that protected its writer. `go test -race` flagged it.
2. **`WS.Close` registry leak.** When the client disconnected before the handler explicitly called `Close`, the registry entry was never removed, so graceful shutdown saw a "live" connection that wasn't.
3. **`WS.readLoop` channel-send blocking.** After the handler returned, nothing was reading `msgCh`; the read loop could block indefinitely on the next inbound frame.
4. **`serveStream` / `serveStreamSimple` divergence.** ~90% of the SSE transport code was duplicated between `Stream[Req]` and `StreamSimple`. A bug fixed in one would silently regress in the other.

This task closes all four.

## Acceptance Criteria

- [x] `WS.closed` migrated from `bool` to `atomic.Bool`. All read sites use `Load()`; all writes use `Store(true)` or `CompareAndSwap`.
- [x] `WS.Close` is idempotent via CAS. Calling twice is a no-op on the second call.
- [x] `WS.Close` always removes the entry from the registry, even when the client initiated the disconnect.
- [x] `WS.readLoop` channel sends guarded by `select { case msgCh <- m: case <-ctx.Done(): return }` so a returned handler cannot deadlock the read loop.
- [x] `serveStream` and `serveStreamSimple` collapsed into a single helper. `Stream[Req]` and `StreamSimple` now delegate via a closure that supplies the per-variant extraction step.
- [x] `go test -race ./...` clean.
- [x] `tests/integration/longlived_test.go` runs clean for at least 30 seconds under `-race`.

## Technical Approach

### Step 3.1: Reproduce before you fix

Run `go test -race ./...` in a loop until the WS race fires. Capture the report. Confirm both racing reads are in handler-wrapper end-of-func guards (not in user code).

### Step 3.2: Atomic migration

```go
// Before
type WS struct {
    mu     sync.Mutex
    closed bool
    // ...
}

// After
type WS struct {
    closed atomic.Bool
    // ...
}
```

`Close` becomes:

```go
func (w *WS) Close(code CloseCode, reason string) error {
    if !w.closed.CompareAndSwap(false, true) {
        return nil  // already closed
    }
    defer defaultRegistry.remove(w)  // always; not just on first close
    return w.conn.Close(websocket.StatusCode(code), reason)
}
```

### Step 3.3: readLoop guard

```go
for {
    typ, data, err := w.conn.Read(ctx)
    if err != nil {
        return  // ctx canceled or peer closed
    }
    select {
    case msgCh <- message{typ, data}:
    case <-ctx.Done():
        return
    }
}
```

### Step 3.4: serveStream unification

Extract the shared transport (header writes, keepalive goroutine, registry register/remove, write loop) into:

```go
func serveStream(w http.ResponseWriter, r *http.Request, opts streamOpts, run func(*SSEStream) error) { ... }
```

`Stream[Req]` and `StreamSimple` both build a `run` closure: the typed variant performs extraction first, the simple variant runs the handler directly.

### Step 3.5: Registry tests

Add `TestWebSocket_GracefulShutdown` and `TestShutdown_WebSocketsClosed` (Task 5) to lock the registry-removal contract.

## Tests Required

- `go test -race -run TestWS ./... -count=10` clean across 10 reps.
- The new shutdown tests close active connections within the configured shutdown timeout.
- `tests/integration/longlived_test.go` under `-race -timeout=2m` clean.

## Definition of Done

- [x] No `sync.Mutex` access to `WS.closed` remains; only `atomic.Bool` ops.
- [x] No `defaultRegistry.remove` call lives outside `Close`. Single removal site.
- [x] `serveStreamSimple` removed; only `serveStream` remains.
- [x] `go test ./... -race` clean.
- [x] `golangci-lint run ./...` clean.
- [x] CHANGELOG entries under `[Unreleased]` → `Changed` (atomic migration, idempotent Close, readLoop guard, serveStream unification) and `Fixed` (data race, registry leak).
