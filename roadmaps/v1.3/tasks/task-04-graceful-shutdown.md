# Task 4: Graceful Shutdown Hooks

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 2-3 days
**Dependencies:** Tasks 1 (WebSocket) and 2 (SSE) for registry integration

## Context

Barista will run on Kubernetes, which sends SIGTERM during rolling updates and pod deletions. Long-lived connections (SSE log streams, WebSocket terminals) must be closed cleanly before the HTTP server shuts down — not abruptly reset.

Espresso already supports `WithShutdownTimeout` for setting the shutdown deadline, but there's no hook point for custom cleanup logic. For example:

- Closing database connections
- Flushing in-memory caches
- Sending "server shutting down, please reconnect" events to open streams
- Deregistering from service discovery

This task adds a structured shutdown hook mechanism and integrates it with the SSE and WebSocket registries from Tasks 1 and 2.

## Acceptance Criteria

- [ ] `OnShutdown(hook)` method on router for registering cleanup functions
- [ ] Multiple hooks can be registered; they run in registration order
- [ ] Hooks run **before** the HTTP server stops accepting new connections
- [ ] Each hook receives a context that respects `WithShutdownTimeout`
- [ ] Hook timeout → log warning and proceed to next hook (don't block shutdown)
- [ ] Hook panic → recover, log, proceed to next hook
- [ ] SSE streams auto-closed with final "shutdown" comment event
- [ ] WebSocket auto-closed with close code 1001 "going away"
- [ ] Open HTTP requests (non-streaming) complete normally up to shutdown timeout
- [ ] Clean shutdown order documented in code comments

## Technical Approach

### Step 4.1: Router API

Add to `server.go`:

```go
// ShutdownHook is a function invoked during graceful shutdown.
// The context respects the configured shutdown timeout.
// Hooks should return promptly; exceeding the context deadline will cause
// the shutdown process to proceed to the next hook.
type ShutdownHook func(ctx context.Context) error

// OnShutdown registers a hook to run during graceful shutdown.
// Hooks run in the order they were registered, before the HTTP server
// stops accepting new connections. Each hook has access to the shutdown
// context with the configured timeout.
//
// Multiple OnShutdown calls accumulate hooks. This method returns the
// router for chaining.
//
// Example:
//
//	router.
//	    OnShutdown(func(ctx context.Context) error {
//	        return db.Close()
//	    }).
//	    OnShutdown(func(ctx context.Context) error {
//	        return cache.Flush(ctx)
//	    })
func (r *Router) OnShutdown(hook ShutdownHook) *Router
```

### Step 4.2: Shutdown Flow

Update `Brew()` to follow this sequence:

```
1. Server is running, accepting requests.
2. SIGTERM or SIGINT received (or ctx passed to Brew is cancelled).
3. Log: "shutdown initiated"
4. Create shutdown context with configured timeout.
5. Run registered OnShutdown hooks in order.
   For each hook:
     - Call hook(shutdownCtx)
     - If hook returns error → log error, continue
     - If hook panics → recover, log, continue
     - If hook exceeds remaining timeout → cancel its context, continue
6. Close all SSE streams (send final comment, close body):
     registry.CloseAllSSE()
7. Close all WebSockets with code 1001 "going away":
     registry.CloseAllWS(CloseGoingAway, "server shutting down")
8. Stop accepting new HTTP connections.
9. Wait for in-flight HTTP requests with http.Server.Shutdown(ctx).
10. If shutdown context exceeded → force close remaining connections.
11. Log: "shutdown complete"
```

Example implementation:

```go
// shutdown runs the graceful shutdown sequence.
// Called when SIGTERM/SIGINT is received or ctx passed to Brew is cancelled.
func (s *Server) shutdown(ctx context.Context) error {
    slog.Info("shutdown initiated")

    // Create shutdown context with configured timeout
    shutdownCtx, cancel := context.WithTimeout(ctx, s.shutdownTimeout)
    defer cancel()

    // Run user-registered hooks in order
    for i, hook := range s.shutdownHooks {
        if err := runHookSafely(shutdownCtx, hook, i); err != nil {
            slog.Error("shutdown hook failed",
                "hook_index", i,
                "error", err)
        }
    }

    // Close streaming connections BEFORE stopping HTTP server
    // so clients receive proper close signals.
    if s.sseRegistry != nil {
        s.sseRegistry.CloseAll("shutdown")
    }
    if s.wsRegistry != nil {
        s.wsRegistry.CloseAll(CloseGoingAway, "server shutting down")
    }

    // Now stop accepting new connections and wait for in-flight requests.
    if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
        slog.Warn("http server shutdown error", "error", err)
        return err
    }

    slog.Info("shutdown complete")
    return nil
}

// runHookSafely executes a hook with panic recovery.
func runHookSafely(ctx context.Context, hook ShutdownHook, index int) (err error) {
    defer func() {
        if rec := recover(); rec != nil {
            err = fmt.Errorf("hook %d panicked: %v", index, rec)
        }
    }()
    return hook(ctx)
}
```

### Step 4.3: Stream Registry Integration

The WebSocket registry from Task 1 already tracks connections. Task 2's SSE implementation should also maintain a registry with the same pattern.

Create a unified shutdown-aware registry if it makes sense, or keep separate registries with a common interface:

```go
// connRegistry tracks open long-lived connections for graceful shutdown.
// Used by both WebSocket and SSE implementations.
type connRegistry[T any] struct {
    mu    sync.RWMutex
    conns map[T]struct{}
}

func newConnRegistry[T comparable]() *connRegistry[T] {
    return &connRegistry[T]{conns: make(map[T]struct{})}
}

func (r *connRegistry[T]) Add(conn T) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.conns[conn] = struct{}{}
}

func (r *connRegistry[T]) Remove(conn T) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.conns, conn)
}

func (r *connRegistry[T]) Snapshot() []T {
    r.mu.RLock()
    defer r.mu.RUnlock()
    snapshot := make([]T, 0, len(r.conns))
    for c := range r.conns {
        snapshot = append(snapshot, c)
    }
    return snapshot
}

func (r *connRegistry[T]) Len() int {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return len(r.conns)
}
```

Then provide concrete close-all helpers:

```go
func closeAllSSE(reg *connRegistry[*SSEStream], finalMsg string) {
    for _, s := range reg.Snapshot() {
        _ = s.Comment("shutdown: " + finalMsg)
        _ = s.Close()
    }
}

func closeAllWS(reg *connRegistry[*WS], code CloseCode, reason string) {
    for _, ws := range reg.Snapshot() {
        _ = ws.Close(code, reason)
    }
}
```

### Step 4.4: Server Config Updates

Update server configuration to hold hooks and registries:

```go
type serverConfig struct {
    addr             string
    readTimeout      time.Duration
    writeTimeout     time.Duration
    shutdownTimeout  time.Duration
    shutdownHooks    []ShutdownHook
    sseRegistry      *connRegistry[*SSEStream]
    wsRegistry       *connRegistry[*WS]
    // ... existing fields
}
```

Initialize registries in `Portafilter()` so they exist from the start.

## Tests Required

```go
// Tests that hooks run in registration order.
func TestShutdown_HooksRunInOrder(t *testing.T)

// Tests that hooks receive a context with the shutdown timeout applied.
func TestShutdown_HooksReceiveContext(t *testing.T)

// Tests that a hook returning an error doesn't block subsequent hooks.
func TestShutdown_HookError(t *testing.T)

// Tests that a hook panicking doesn't crash the server.
func TestShutdown_HookPanic(t *testing.T)

// Tests that a hook exceeding timeout is cancelled but shutdown continues.
func TestShutdown_HookTimeout(t *testing.T)

// Tests that open SSE streams receive shutdown comment and close.
func TestShutdown_SSEStreamsClosed(t *testing.T)

// Tests that open WebSockets close with code 1001.
func TestShutdown_WebSocketsClosed(t *testing.T)

// Tests that in-flight non-streaming requests complete normally.
func TestShutdown_InFlightRequestsComplete(t *testing.T)

// Tests that new requests are rejected after shutdown begins.
func TestShutdown_NewRequestsRejected(t *testing.T)

// Tests shutdown with no registered hooks still works correctly.
func TestShutdown_NoHooks(t *testing.T)

// Tests shutdown called multiple times (idempotent).
func TestShutdown_Idempotent(t *testing.T)

// Integration test: real server + real client sending SIGTERM.
func TestShutdown_Integration(t *testing.T)
```

All tests must pass with `go test -race`.

## Example Usage

Add to README "Graceful Shutdown" section:

````markdown
## Graceful Shutdown

Register cleanup hooks to run before the server stops:

```go
router := espresso.Portafilter().
    WithState(state).
    OnShutdown(func(ctx context.Context) error {
        slog.Info("flushing cache")
        return cache.Flush(ctx)
    }).
    OnShutdown(func(ctx context.Context) error {
        slog.Info("closing database")
        return db.Close()
    }).
    Get("/health", espresso.Ristretto(healthCheck))

// Brew blocks until SIGTERM/SIGINT or the passed context is cancelled,
// then runs graceful shutdown.
ctx, cancel := signal.NotifyContext(context.Background(),
    os.Interrupt, syscall.SIGTERM)
defer cancel()

if err := router.BrewContext(ctx, espresso.WithAddr(":8080")); err != nil {
    slog.Error("server error", "error", err)
}
```

### Shutdown Sequence

When shutdown is triggered:

1. Registered `OnShutdown` hooks run in order
2. All open SSE streams close with a final comment
3. All open WebSockets close with code 1001 (going away)
4. The HTTP server stops accepting new connections
5. In-flight HTTP requests complete up to the shutdown timeout
6. Remaining connections are force-closed

Use `WithShutdownTimeout` to control how long step 5 waits:

```go
router.Brew(
    espresso.WithAddr(":8080"),
    espresso.WithShutdownTimeout(30*time.Second),
)
```
````

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked
- [ ] All tests pass with `go test -race`
- [ ] Integration test with actual HTTP client connecting and server receiving SIGTERM
- [ ] SSE and WebSocket close behaviors verified end-to-end
- [ ] Documentation in README with real-world example
- [ ] Godoc comments on all new public APIs
- [ ] `CHANGELOG.md` entry under `[Unreleased]`
- [ ] `golangci-lint run ./...` passes
- [ ] PR description documents any deviations from this spec

## Potential Follow-Up Issues

Out of scope for this task:

- Startup hooks (`OnStartup`) — useful but different task
- Health check integration (mark unhealthy before shutdown) — aplication concern
- Per-route shutdown priority
- Async hooks that run in parallel (current design is sequential for simplicity)
