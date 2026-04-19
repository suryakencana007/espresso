# Task 2: Typed Streaming Response (SSE)

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 3-5 days
**Dependencies:** None (can start in parallel with Task 1)

## Context

Barista needs Server-Sent Events (SSE) for three primary use cases:

- **Live log streaming** from Kubernetes pods (streams may run for hours)
- **BuildKit progress events** during container image builds
- **Deployment status updates** (pending → building → deploying → healthy)

Espresso currently provides a low-level `SSEWriter` that operates directly on `http.ResponseWriter`. This is inconsistent with Espresso's typed handler pattern (`JSON[T]`, `Ristretto`, `Doppio`). The goal of this task is to make SSE a first-class handler type that:

- Integrates with state injection
- Works with extractors (`Path[T]`, `Query[T]`)
- Supports context cancellation on client disconnect
- Follows Espresso's naming and style conventions

## Acceptance Criteria

- [ ] Handler signature: `func(ctx context.Context, req *ExtractorT, stream *espresso.SSEStream) error`
- [ ] Handler can be registered via `router.Get(path, espresso.Stream(handler))`
- [ ] Response headers auto-set: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `X-Accel-Buffering: no`
- [ ] Every call to `Send()` is auto-flushed (not buffered)
- [ ] State injection works inside the handler
- [ ] Client disconnect triggers `ctx.Done()` in the handler
- [ ] Handler returning an error closes the stream and optionally sends an error event
- [ ] Compatible with existing middleware chain
- [ ] Event ID support for reconnection (parses `Last-Event-ID` header)
- [ ] Retry interval is configurable per-stream via `SetRetry()` or option
- [ ] Keepalive comments can be sent on a configurable interval

## Technical Approach

### Step 2.1: Create SSEStream Type

Create a new file `sse.go` in the root package.

```go
package espresso

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

// SSEStream represents a Server-Sent Events stream to a client.
// Obtain an SSEStream by registering a handler via Stream() or StreamSimple().
// Do not construct SSEStream directly.
//
// SSEStream is safe for concurrent use by multiple goroutines.
// All Send methods internally acquire a mutex to ensure frame integrity.
type SSEStream struct {
    w           http.ResponseWriter
    flusher     http.Flusher
    ctx         context.Context
    mu          sync.Mutex
    closed      atomic.Bool
    eventID     atomic.Uint64
    lastEventID string
}

// Event represents a single Server-Sent Event.
type Event struct {
    // ID is the event identifier. Optional.
    // If empty, an auto-incremented ID is assigned on send.
    ID string

    // Name is the event type (maps to the "event:" field in SSE format).
    // If empty, no event field is sent (client receives a default "message" event).
    Name string

    // Data is the event payload (maps to the "data:" field).
    // Multi-line data is automatically split into multiple "data:" lines.
    Data string

    // Retry is a hint to the client about how long to wait before reconnecting.
    // Optional. If zero, no retry field is sent.
    Retry time.Duration
}

// Send sends an event to the client.
// The event is automatically flushed to the client after writing.
// Returns an error if the client has disconnected or the stream is closed.
func (s *SSEStream) Send(event Event) error

// SendJSON marshals v to JSON and sends it as an event with the given name.
// Equivalent to Send(Event{Name: name, Data: string(json.Marshal(v))}).
func (s *SSEStream) SendJSON(name string, v any) error

// SendText sends an event with the given name and plain text data.
func (s *SSEStream) SendText(name, data string) error

// SendData sends an event with just data, no event name.
// The client will receive this as a default "message" event.
func (s *SSEStream) SendData(data string) error

// Comment sends an SSE comment line.
// Comments are ignored by SSE clients but useful as keepalive pings.
func (s *SSEStream) Comment(comment string) error

// SetRetry sets the reconnection retry interval hint for the client.
// This is sent as a one-time "retry:" field. Clients will use this value
// when reconnecting after a disconnection.
func (s *SSEStream) SetRetry(d time.Duration) error

// LastEventID returns the value of the Last-Event-ID request header,
// which clients send when reconnecting. Use this to resume from where
// the client left off.
// Returns an empty string if the client did not send this header.
func (s *SSEStream) LastEventID() string

// Context returns the stream's context.
// This context is cancelled when the client disconnects or the stream is closed.
func (s *SSEStream) Context() context.Context

// Close closes the stream. Safe to call multiple times.
// After Close, all Send calls return an error.
func (s *SSEStream) Close() error
```

### Step 2.2: Handler Wrapper

Append to `handler.go`:

```go
// StreamHandler is the function signature for SSE handlers.
// The type parameter T is the extractor type (e.g., Path[Req], Query[Req]).
type StreamHandler[T any] func(ctx context.Context, req *T, stream *SSEStream) error

// Stream wraps an SSE handler so it can be registered as a route.
// It handles:
//   - Setting SSE response headers
//   - Extractor parsing
//   - State injection
//   - Keepalive (if configured via WithKeepAlive)
//   - Cleanup on client disconnect or handler return
//
// Example:
//
//	router.Get("/stream", espresso.Stream(counterStream))
func Stream[T any](h StreamHandler[T], opts ...StreamOption) Handler

// StreamSimple wraps an SSE handler that doesn't need an extractor.
// This is the Ristretto-equivalent for SSE.
//
// Example:
//
//	router.Get("/time", espresso.StreamSimple(timeStream))
func StreamSimple(h func(ctx context.Context, stream *SSEStream) error, opts ...StreamOption) Handler

// StreamOption configures a streaming handler.
type StreamOption func(*streamConfig)

// WithKeepAlive sets the interval for sending keepalive comment frames.
// Keepalive is useful for detecting disconnections and keeping proxies
// from closing idle connections.
// Set to 0 to disable. Default: disabled.
func WithKeepAlive(interval time.Duration) StreamOption

// WithRetryHint sets an initial retry hint sent at stream start.
// This tells the client how long to wait before reconnecting.
func WithRetryHint(d time.Duration) StreamOption

// streamConfig holds internal configuration for a stream handler.
type streamConfig struct {
    keepAliveInterval time.Duration
    initialRetryHint  time.Duration
}
```

### Step 2.3: Implementation Details

Inside `Stream[T]()` wrapper, the flow is:

1. **Set SSE headers before handler runs**
   ```go
   w.Header().Set("Content-Type", "text/event-stream")
   w.Header().Set("Cache-Control", "no-cache")
   w.Header().Set("Connection", "keep-alive")
   w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
   ```

2. **Assert Flusher support**
   - Cast `w` to `http.Flusher`. If not supported, return `500 Internal Server Error`.
   - In practice, Go's default response writer supports this.

3. **Parse Last-Event-ID header**
   - Read `r.Header.Get("Last-Event-ID")` and store in the stream

4. **Run extractor**
   - Call `T.Extract(r)` to get typed request data
   - If extraction fails, respond with `400 Bad Request` and return

5. **Create SSEStream**
   - Instantiate with ctx, writer, flusher, lastEventID

6. **Send initial retry hint** (if `WithRetryHint` configured)
   - Write `retry: N\n\n` once at stream start

7. **Start keepalive goroutine** (if `WithKeepAlive > 0`)
   - Goroutine that sends `:\n\n` (empty comment) on interval
   - Stop on context cancel or handler return

8. **Register stream in global registry** (for graceful shutdown)

9. **Call user handler**
   - `err := h(ctx, req, stream)`

10. **Cleanup**
    - If context was cancelled (client disconnect): normal cleanup, no log
    - If handler returned error: log error; optionally send final error event
    - Cancel keepalive goroutine
    - Close the stream
    - Deregister from registry

### Step 2.4: Concurrent Write Safety

`SSEStream.Send()` must be safe for concurrent use because users may spawn goroutines (e.g., one goroutine sending log lines, another sending periodic status). Use a `sync.Mutex` to protect writes.

The flow for any Send method is:

```go
func (s *SSEStream) Send(event Event) error {
    if s.closed.Load() {
        return errors.New("stream closed")
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    // Re-check after acquiring lock
    if s.closed.Load() {
        return errors.New("stream closed")
    }

    // Format and write the event
    if err := s.writeEvent(event); err != nil {
        s.closed.Store(true)
        return err
    }

    s.flusher.Flush()
    return nil
}
```

### Step 2.5: SSE Format Compliance

Format events according to the SSE specification:

```
id: <event_id>\n
event: <event_name>\n
data: <data>\n
retry: <milliseconds>\n
\n
```

Required behaviors:

- Empty fields are omitted (don't send `event: \n`)
- Multi-line data must be split with `data:` prefix per line:
  ```
  data: line 1\n
  data: line 2\n
  \n
  ```
- An event is terminated by a blank line (`\n\n`)
- Comments start with `:` and are terminated by `\n`:
  ```
  : this is a comment\n
  ```

Here's an implementation sketch for writing an event:

```go
func (s *SSEStream) writeEvent(e Event) error {
    var sb strings.Builder

    if e.ID == "" {
        // Auto-generate sequential ID
        e.ID = strconv.FormatUint(s.eventID.Add(1), 10)
    }
    sb.WriteString("id: ")
    sb.WriteString(e.ID)
    sb.WriteByte('\n')

    if e.Name != "" {
        sb.WriteString("event: ")
        sb.WriteString(e.Name)
        sb.WriteByte('\n')
    }

    if e.Data != "" {
        for _, line := range strings.Split(e.Data, "\n") {
            sb.WriteString("data: ")
            sb.WriteString(line)
            sb.WriteByte('\n')
        }
    }

    if e.Retry > 0 {
        sb.WriteString("retry: ")
        sb.WriteString(strconv.FormatInt(e.Retry.Milliseconds(), 10))
        sb.WriteByte('\n')
    }

    sb.WriteByte('\n') // event terminator

    _, err := s.w.Write([]byte(sb.String()))
    return err
}
```

## File Structure

### New Files

- `sse.go` — `SSEStream`, `Event` types and implementations
- `sse_test.go` — Unit tests
- `cmd/example/sse/main.go` — Counter SSE example
- `cmd/example/sse/README.md` — Example explanation

### Modified Files

- `handler.go` — Add `Stream[T]()`, `StreamSimple()`, `StreamOption`
- `response.go` — Keep existing `SSEWriter` for backward compat; add deprecation comment
- `README.md` — Update SSE section to show new API
- `CHANGELOG.md` — Add entry

## Tests Required

Create `sse_test.go` with these tests:

```go
// Tests single event send and client-side receive.
func TestSSE_BasicStream(t *testing.T)

// Tests multiple events received correctly with parsing client.
func TestSSE_MultipleEvents(t *testing.T)

// Tests SendJSON with a struct.
func TestSSE_JSONEvent(t *testing.T)

// Tests that data containing \n is properly encoded with data: prefix per line.
func TestSSE_MultilineData(t *testing.T)

// Tests that client disconnect cancels the handler's context.
func TestSSE_ClientDisconnect(t *testing.T)

// Tests handler error cleanup.
func TestSSE_HandlerError(t *testing.T)

// Tests Last-Event-ID header propagation to stream.
func TestSSE_LastEventID(t *testing.T)

// Tests keepalive comments sent on interval.
func TestSSE_KeepAlive(t *testing.T)

// Tests MustGetState[T] inside SSE handler.
func TestSSE_StateInjection(t *testing.T)

// Tests concurrent Send calls with race detector.
func TestSSE_ConcurrentWrites(t *testing.T)

// Tests that each Send is immediately flushed to the client.
func TestSSE_NoBuffering(t *testing.T)

// Tests that response headers are set correctly.
func TestSSE_HeadersSet(t *testing.T)

// Tests integration with Logging and RequestID middleware.
func TestSSE_MiddlewareChain(t *testing.T)

// Tests that SetRetry sends the retry field.
func TestSSE_SetRetry(t *testing.T)

// Tests auto-generated event IDs are sequential.
func TestSSE_AutoEventID(t *testing.T)

// Tests that Comment sends a proper SSE comment line.
func TestSSE_Comment(t *testing.T)
```

All tests must pass with `go test -race`. Target ≥85% coverage on `sse.go`.

### Testing Strategy

Use `net/http/httptest.Server` and the standard library's `bufio.Scanner` to parse SSE events on the client side. Example pattern:

```go
func readSSEEvents(t *testing.T, resp *http.Response) []Event {
    t.Helper()
    defer resp.Body.Close()

    scanner := bufio.NewScanner(resp.Body)
    var events []Event
    var current Event

    for scanner.Scan() {
        line := scanner.Text()
        if line == "" {
            if current != (Event{}) {
                events = append(events, current)
                current = Event{}
            }
            continue
        }
        // Parse fields: "id:", "event:", "data:", "retry:"
        // ... (implementation here)
    }

    return events
}
```

## Example to Ship

Create `cmd/example/sse/main.go`:

```go
// Example: Counter SSE endpoint.
// Demonstrates:
//   - Basic SSE streaming
//   - Query extractor for request parameters
//   - JSON events
//   - Client disconnect handling
//   - Keepalive comments

package main

import (
    "context"
    "log/slog"
    "time"

    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
)

// CounterReq is the query-string extractor for the counter stream.
type CounterReq struct {
    Start int `query:"start"`
    Step  int `query:"step"`
}

// CountEvent is the event payload sent to the client.
type CountEvent struct {
    Value     int       `json:"value"`
    Timestamp time.Time `json:"timestamp"`
}

func counterStream(ctx context.Context, req *extractor.Query[CounterReq],
    stream *espresso.SSEStream) error {

    count := req.Data.Start
    step := req.Data.Step
    if step == 0 {
        step = 1
    }

    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil // client disconnected
        case <-ticker.C:
            count += step
            if err := stream.SendJSON("count", CountEvent{
                Value:     count,
                Timestamp: time.Now(),
            }); err != nil {
                return err // likely client disconnect
            }
        }
    }
}

func main() {
    router := espresso.Portafilter().
        Get("/stream/count", espresso.Stream(counterStream,
            espresso.WithKeepAlive(30*time.Second),
        ))

    slog.Info("listening on :8080")
    _ = router.Brew(espresso.WithAddr(":8080"))
}
```

Create `cmd/example/sse/README.md`:

```markdown
# SSE Counter Example

Demonstrates Server-Sent Events (SSE) support in Espresso.

## Run

    go run ./cmd/example/sse

## Test

With curl:

    curl -N "http://localhost:8080/stream/count?start=0&step=1"

You should see JSON events streamed every second.
```

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked
- [ ] All unit tests pass with `go test -race`, coverage ≥85% on `sse.go`
- [ ] Example runs and streams correctly (verified with `curl -N`)
- [ ] Godoc comments on all public APIs with examples
- [ ] README SSE section updated to show new API
- [ ] Old `SSEWriter` has deprecation comment pointing to new API
- [ ] `CHANGELOG.md` entry under `[Unreleased]`
- [ ] `golangci-lint run ./...` passes
- [ ] PR description documents any deviations from this spec

## Potential Follow-Up Issues

Out of scope for this task:

- HTTP/2 server push integration (if applicable)
- Named event type helpers (e.g., `stream.OnEvent("log", func() {...})`)
- Backpressure strategies (slow client handling)
- Event batching for high-throughput streams
