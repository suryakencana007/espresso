# Task 1: WebSocket Handler Support

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 5-7 days
**Dependencies:** None (can start immediately)

## Context

Barista, the PaaS project that Espresso will power, needs a **web terminal** feature. Users will open a browser-based terminal that connects to a running Kubernetes pod and execute commands inside that pod (similar to `kubectl exec -it`).

The flow is:

1. Browser opens a WebSocket connection to an Espresso endpoint
2. Espresso handler receives the upgrade, gets access to app state (including Kubernetes client)
3. Handler opens a bidirectional `exec` stream to the target Kubernetes pod
4. Handler bridges data between the browser WebSocket and the Kubernetes exec stream
5. Handler manages ping/pong keepalives, close events, terminal resize events

This is **bidirectional streaming** — different from Server-Sent Events (Task 2), which only flow server-to-client. The WebSocket pattern must integrate naturally with Espresso's existing state injection and middleware chain.

## Library Choice

Use `github.com/coder/websocket` (not `gorilla/websocket`).

**Reasons:**

- **Context-first API** — Every operation accepts `context.Context`, matching Espresso's context-heavy pattern
- **Smaller API surface** — Approximately one-third the size of Gorilla; easier to wrap in an Espresso abstraction
- **Modern Go idioms** — Designed after context propagation became standard in Go; no legacy baggage
- **Active maintenance** — Maintained by Coder, Inc. (the `nhooyr.io/websocket` library was transferred to this organization)

## Acceptance Criteria

- [ ] Handler signature: `func(ctx context.Context, req *ExtractorT, ws *espresso.WS) error`
- [ ] Handler can be registered via `router.Get(path, espresso.WebSocket(handler))`
- [ ] HTTP to WebSocket upgrade happens automatically on request
- [ ] State injection (`espresso.MustGetState[T]`) works inside the handler
- [ ] Both text and binary frames are supported
- [ ] Ping frames are sent automatically every 30 seconds (configurable)
- [ ] Client disconnect triggers `ctx.Done()` in the handler
- [ ] Handler returning an error closes the connection with an appropriate close code
- [ ] Handler returning `nil` closes the connection normally (code 1000)
- [ ] Compatible with existing middleware: `RequestIDMiddleware`, `RecoverMiddleware`, `LoggingMiddleware`
- [ ] Existing extractors work before upgrade: `Path[T]`, `Query[T]`, `Header[T]` (JSON body extractors are not applicable to WebSocket endpoints)
- [ ] Graceful shutdown: open WebSockets receive a close frame before server shutdown

## Technical Approach

### Step 1.1: Add Dependency

Add to `go.mod`:

```
github.com/coder/websocket v1.8.x
```

Run `go mod tidy` to update `go.sum`.

### Step 1.2: Create WebSocket Wrapper Type

Create a new file `websocket.go` in the root package.

```go
package espresso

import (
    "context"
    "time"
    "github.com/coder/websocket"
)

// WS wraps a WebSocket connection with an Espresso-friendly API.
// Obtain a *WS instance by registering a handler via WebSocket() or
// WebSocketSimple() — do not construct directly.
type WS struct {
    conn   *websocket.Conn
    ctx    context.Context
    config WSConfig
}

// WSConfig holds configuration for a WebSocket handler.
type WSConfig struct {
    // PingInterval is how often the server sends ping frames to the client.
    // Set to 0 to disable. Default: 30s.
    PingInterval time.Duration

    // ReadTimeout is the timeout for Read operations.
    // Set to 0 for no timeout. Default: 60s.
    ReadTimeout time.Duration

    // WriteTimeout is the timeout for Write operations.
    // Default: 10s.
    WriteTimeout time.Duration

    // MaxMessageSize is the maximum allowed message size in bytes.
    // Default: 1 MiB (1048576).
    MaxMessageSize int64

    // Subprotocols are the WebSocket subprotocols the server supports.
    // Optional.
    Subprotocols []string

    // OriginPatterns specifies allowed Origin header patterns for CORS.
    // Default: same-origin only.
    OriginPatterns []string

    // CompressionMode controls per-message deflate compression.
    // Default: disabled.
    CompressionMode websocket.CompressionMode
}

// MessageType distinguishes text and binary WebSocket messages.
type MessageType int

const (
    // MessageText indicates a UTF-8 encoded text message.
    MessageText MessageType = iota + 1
    // MessageBinary indicates a binary message.
    MessageBinary
)

// CloseCode is a WebSocket close status code as defined in RFC 6455.
type CloseCode int

const (
    CloseNormal           CloseCode = 1000 // Normal closure
    CloseGoingAway        CloseCode = 1001 // Endpoint is going away (e.g., server shutdown)
    CloseProtocolError    CloseCode = 1002 // Protocol error
    CloseUnsupportedData  CloseCode = 1003 // Received unsupported data type
    CloseNoStatusReceived CloseCode = 1005 // No status code in close frame (internal use)
    CloseAbnormalClosure  CloseCode = 1006 // Connection closed abnormally (internal use)
    CloseInvalidPayload   CloseCode = 1007 // Received data not consistent with frame type
    ClosePolicyViolation  CloseCode = 1008 // Received message violates policy
    CloseMessageTooBig    CloseCode = 1009 // Received message too large
    CloseInternalError    CloseCode = 1011 // Server encountered an internal error
    CloseServiceRestart   CloseCode = 1012 // Server is restarting
    CloseTryAgainLater    CloseCode = 1013 // Server is overloaded
)

// Read reads the next message from the WebSocket connection.
// It blocks until a message is received, the context is cancelled,
// or the connection is closed.
// Returns the message type, the payload bytes, and any error.
func (w *WS) Read(ctx context.Context) (MessageType, []byte, error)

// Write sends a message to the WebSocket connection.
func (w *WS) Write(ctx context.Context, msgType MessageType, data []byte) error

// WriteText is a convenience method that writes a UTF-8 text frame.
func (w *WS) WriteText(ctx context.Context, text string) error

// WriteBinary is a convenience method that writes a binary frame.
func (w *WS) WriteBinary(ctx context.Context, data []byte) error

// WriteJSON marshals v to JSON and sends it as a text frame.
func (w *WS) WriteJSON(ctx context.Context, v any) error

// ReadJSON reads the next message and unmarshals it into v.
// v must be a non-nil pointer.
// Returns an error if the message is not valid JSON or if unmarshalling fails.
func (w *WS) ReadJSON(ctx context.Context, v any) error

// Close closes the WebSocket connection with the given status code and reason.
// If the connection is already closed, Close returns nil.
func (w *WS) Close(code CloseCode, reason string) error

// Context returns the WebSocket's context.
// This context is cancelled when the WebSocket is closed, either by
// the server, the client, or due to an error.
func (w *WS) Context() context.Context

// Subprotocol returns the negotiated WebSocket subprotocol.
// Returns an empty string if no subprotocol was negotiated.
func (w *WS) Subprotocol() string
```

### Step 1.3: Handler Wrapper

Append to `handler.go`:

```go
// WebSocketHandler is the function signature for WebSocket handlers.
// The type parameter T is the extractor type (e.g., Path[Req], Query[Req])
// used to parse request data before upgrading to WebSocket.
// Use WebSocketSimple for handlers that don't need extractors.
type WebSocketHandler[T any] func(ctx context.Context, req *T, ws *WS) error

// WebSocket wraps a WebSocket handler so it can be registered as a route.
// It handles:
//   - HTTP to WebSocket upgrade
//   - Extractor parsing (Path, Query, Header, etc.)
//   - State injection from context
//   - Ping/pong keepalive
//   - Graceful cleanup on handler return
//   - Close frame sending on server shutdown
//
// Example:
//
//	router.Get("/ws/{room}", espresso.WebSocket(echoHandler))
func WebSocket[T any](h WebSocketHandler[T], opts ...WSOption) Handler

// WebSocketSimple wraps a WebSocket handler that doesn't need an extractor.
// This is the Ristretto-equivalent for WebSockets.
//
// Example:
//
//	router.Get("/ws", espresso.WebSocketSimple(pingHandler))
func WebSocketSimple(h func(ctx context.Context, ws *WS) error, opts ...WSOption) Handler

// WSOption configures a WebSocket handler.
type WSOption func(*WSConfig)

// WithPingInterval sets how often ping frames are sent. Set to 0 to disable.
func WithPingInterval(d time.Duration) WSOption

// WithMaxMessageSize sets the maximum allowed message size in bytes.
func WithMaxMessageSize(size int64) WSOption

// WithSubprotocols sets the supported WebSocket subprotocols.
func WithSubprotocols(protos ...string) WSOption

// WithOriginPatterns sets the allowed Origin header patterns for CORS validation.
func WithOriginPatterns(patterns ...string) WSOption

// WithReadTimeout sets the timeout for Read operations. 0 means no timeout.
func WithReadTimeout(d time.Duration) WSOption

// WithWriteTimeout sets the timeout for Write operations.
func WithWriteTimeout(d time.Duration) WSOption
```

### Step 1.4: Upgrade Flow Implementation

Inside `WebSocket[T]()`:

1. **Validate upgrade request**
   - Check that the request has `Connection: Upgrade` and `Upgrade: websocket` headers
   - If not present, respond with `426 Upgrade Required` and return

2. **Parse extractor**
   - Call `T.Extract(r)` to parse request data (path params, query, headers)
   - If extraction fails, respond with `400 Bad Request` and the error

3. **Perform WebSocket upgrade**
   - Call `websocket.Accept(w, r, &websocket.AcceptOptions{...})` with config
   - This handles the WebSocket handshake and returns a `*websocket.Conn`

4. **Wrap the connection**
   - Create `*WS` with the accepted connection, context, and config
   - Register the WS in the global WebSocket registry (for graceful shutdown)

5. **Start ping goroutine**
   - If `PingInterval > 0`, start a goroutine that sends ping frames on interval
   - Stop the goroutine when handler returns

6. **Call user handler**
   - Call the user's `h(ctx, req, ws)` function
   - Track handler duration for metrics/logging

7. **Cleanup on handler return**
   - If handler returned an error: close with `CloseInternalError` (1011) and error message
   - If handler returned `nil`: close with `CloseNormal` (1000)
   - Cancel the ping goroutine
   - Deregister from WebSocket registry

8. **Panic recovery**
   - Wrap the handler call in a deferred recover
   - If panic: close with `CloseInternalError`, log the panic with stack trace

### Step 1.5: State Injection

State injection works automatically because:

1. The state middleware stores state in the request context
2. Our `WebSocket[T]()` wrapper passes this context to the handler
3. The handler calls `espresso.MustGetState[T](ctx)` which reads from the context

No additional code should be needed. Verify with a test that `MustGetState` works inside a WebSocket handler.

### Step 1.6: Graceful Shutdown Integration

In `server.go`, add a WebSocket connection registry:

```go
// wsRegistry tracks open WebSocket connections for graceful shutdown.
type wsRegistry struct {
    mu    sync.RWMutex
    conns map[*WS]struct{}
}

func newWSRegistry() *wsRegistry {
    return &wsRegistry{conns: make(map[*WS]struct{})}
}

// Add registers a WebSocket connection.
func (r *wsRegistry) Add(ws *WS) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.conns[ws] = struct{}{}
}

// Remove deregisters a WebSocket connection.
func (r *wsRegistry) Remove(ws *WS) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.conns, ws)
}

// CloseAll sends a close frame with the given code and reason to all
// registered connections. Safe to call concurrently with Add/Remove.
func (r *wsRegistry) CloseAll(code CloseCode, reason string) {
    r.mu.RLock()
    conns := make([]*WS, 0, len(r.conns))
    for ws := range r.conns {
        conns = append(conns, ws)
    }
    r.mu.RUnlock()

    for _, ws := range conns {
        _ = ws.Close(code, reason) // best-effort
    }
}
```

In `Brew()`'s shutdown path:

1. Before calling `http.Server.Shutdown()`, call `wsRegistry.CloseAll(CloseGoingAway, "server shutting down")`
2. This gives clients a chance to reconnect elsewhere

## File Structure

### New Files

- `websocket.go` — `WS` type, `WSConfig`, `MessageType`, `CloseCode`, method implementations
- `websocket_test.go` — Unit tests (see Tests Required below)
- `websocket_bench_test.go` — Benchmarks for Read/Write operations
- `cmd/example/websocket/main.go` — Echo WebSocket server example
- `cmd/example/websocket/README.md` — Example explanation and usage instructions

### Modified Files

- `handler.go` — Add `WebSocket[T]()`, `WebSocketSimple()`, `WSOption` types
- `server.go` — Add `wsRegistry`, integrate with shutdown flow
- `go.mod`, `go.sum` — Add `github.com/coder/websocket` dependency
- `README.md` — Add WebSocket section
- `CHANGELOG.md` — Add entry under `[Unreleased]`

## Tests Required

Create `websocket_test.go` with these tests (minimum):

```go
// Tests basic HTTP to WebSocket upgrade with proper headers.
func TestWebSocket_Upgrade(t *testing.T)

// Tests that a plain HTTP request to a WebSocket endpoint returns 426.
func TestWebSocket_RejectNonUpgrade(t *testing.T)

// Tests text message round-trip (write then read).
func TestWebSocket_EchoText(t *testing.T)

// Tests binary message round-trip.
func TestWebSocket_EchoBinary(t *testing.T)

// Tests WriteJSON and ReadJSON with a struct.
func TestWebSocket_JSON(t *testing.T)

// Tests that MustGetState[T] works inside a WS handler.
func TestWebSocket_StateInjection(t *testing.T)

// Tests that Path[T] extractor works before upgrade.
func TestWebSocket_PathExtractor(t *testing.T)

// Tests that Query[T] extractor works before upgrade.
func TestWebSocket_QueryExtractor(t *testing.T)

// Tests that client disconnect triggers ctx.Done() in handler.
func TestWebSocket_ClientDisconnect(t *testing.T)

// Tests that handler returning triggers connection close.
func TestWebSocket_ServerClose(t *testing.T)

// Tests that handler error causes close with code 1011.
func TestWebSocket_HandlerError(t *testing.T)

// Tests that ping frames are sent on interval.
func TestWebSocket_PingKeepalive(t *testing.T)

// Tests that open connections receive 1001 on server shutdown.
func TestWebSocket_GracefulShutdown(t *testing.T)

// Tests that existing middleware (RequestID, Logging) works.
func TestWebSocket_MiddlewareChain(t *testing.T)

// Tests that panic in handler is recovered and connection closed with 1011.
func TestWebSocket_Recover(t *testing.T)

// Tests that oversized messages are rejected.
func TestWebSocket_MaxMessageSize(t *testing.T)
```

All tests must:

- Use `go test -race` without race conditions
- Use `httptest.NewServer` for realistic HTTP handling
- Use `github.com/coder/websocket` client-side methods for connecting

Aim for **≥85% coverage** on `websocket.go`.

## Example to Ship

Create `cmd/example/websocket/main.go`:

```go
// Example: Echo WebSocket server with state injection.
// Demonstrates:
//   - Basic WebSocket upgrade
//   - Path extractor and state injection
//   - Text and binary frame handling
//   - Context cancellation on client disconnect
//   - Graceful shutdown

package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
)

// AppState holds dependencies for the application.
type AppState struct {
    Logger *slog.Logger
}

// EchoReq is the request data extracted from the URL path.
type EchoReq struct {
    Room string `path:"room"`
}

// echoHandler reads messages from the WebSocket and echoes them back
// with a room prefix.
func echoHandler(ctx context.Context, req *extractor.Path[EchoReq], ws *espresso.WS) error {
    state := espresso.MustGetState[AppState](ctx)
    state.Logger.Info("client connected", "room", req.Data.Room)
    defer state.Logger.Info("client disconnected", "room", req.Data.Room)

    for {
        msgType, data, err := ws.Read(ctx)
        if err != nil {
            return nil // client disconnected normally
        }

        response := []byte("[" + req.Data.Room + "] " + string(data))
        if err := ws.Write(ctx, msgType, response); err != nil {
            return err
        }
    }
}

func main() {
    state := AppState{Logger: slog.Default()}

    router := espresso.Portafilter().
        WithState(state).
        Get("/ws/{room}", espresso.WebSocket(echoHandler,
            espresso.WithPingInterval(30*time.Second),
        ))

    // Graceful shutdown setup
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    go func() {
        _ = router.Brew(espresso.WithAddr(":8080"))
    }()

    slog.Info("listening on :8080")
    <-ctx.Done()
    slog.Info("shutting down")
    // Server will close all WebSockets with code 1001 before exiting.
}
```

Create `cmd/example/websocket/README.md`:

```markdown
# WebSocket Echo Example

Demonstrates WebSocket support in Espresso.

## Run

    go run ./cmd/example/websocket

## Test

Connect with wscat:

    wscat -c ws://localhost:8080/ws/lobby

Or use any WebSocket client to connect to `ws://localhost:8080/ws/{room}`.
```

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked
- [ ] All unit tests pass, coverage ≥85% on `websocket.go`
- [ ] Example runs and works (verified manually with `wscat` or similar)
- [ ] Benchmarks added for Read/Write operations
- [ ] Godoc comments on all public APIs with runnable examples
- [ ] README has a complete WebSocket section
- [ ] `CHANGELOG.md` has an entry under `[Unreleased]`
- [ ] All existing tests still pass (`go test ./... -race`)
- [ ] `golangci-lint run ./...` passes
- [ ] PR description includes any decisions that deviate from this spec

## Potential Follow-Up Issues

These are out of scope for this task but may be worth tracking:

- Subprotocol selection helpers
- permessage-deflate extension configuration presets
- WebSocket reconnection pattern documentation
- Integration with OpenAPI spec generation (WebSocket endpoints don't fit OpenAPI 3.0 well; may need OpenAPI 3.1 or AsyncAPI)
