# Task 6: Context Cancellation Propagation Test

**Priority:** 🔵 Verification
**Estimated Effort:** 1 day
**Dependencies:** Tasks 1 (WebSocket) and 2 (SSE) complete

## Context

When a client disconnects from any Espresso endpoint, the handler's `context.Context` should be cancelled **quickly** (under 1 second) so the handler can clean up resources and exit. This is critical for:

- Stopping Kubernetes log streams when the client closes the browser
- Terminating expensive database queries when the HTTP request is cancelled
- Releasing locks when the caller goes away
- Freeing goroutines that would otherwise leak

This task verifies cancellation propagation works correctly across all handler types: JSON, SSE, and WebSocket.

## Acceptance Criteria

- [ ] JSON handler: client disconnect → handler's `ctx.Done()` fires in <1s
- [ ] SSE handler: client disconnect → handler's `ctx.Done()` fires in <1s
- [ ] WebSocket handler: client close → handler's `ctx.Done()` fires in <1s
- [ ] Context cancellation propagates to goroutines started by the handler
- [ ] No goroutine leaks after disconnect
- [ ] Behavior documented in README

## Technical Approach

### Step 6.1: Test Harness

Create `context_cancellation_test.go` in the root package (not integration tests, since these should be fast).

Use a pattern where the handler records when `ctx.Done()` fires:

```go
package espresso_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/suryakencana007/espresso"
)

// cancelSignal helps tests verify context cancellation timing.
type cancelSignal struct {
    done    chan struct{}
    doneAt  time.Time
    started time.Time
}

func newCancelSignal() *cancelSignal {
    return &cancelSignal{
        done:    make(chan struct{}),
        started: time.Now(),
    }
}

func (c *cancelSignal) markCancelled() {
    c.doneAt = time.Now()
    close(c.done)
}

func (c *cancelSignal) waitWithTimeout(d time.Duration) (time.Duration, bool) {
    select {
    case <-c.done:
        return c.doneAt.Sub(c.started), true
    case <-time.After(d):
        return d, false
    }
}
```

### Step 6.2: JSON Handler Cancellation Test

```go
// TestContextCancellation_JSON verifies that cancelling a JSON handler's
// request context causes ctx.Done() to fire quickly.
func TestContextCancellation_JSON(t *testing.T) {
    signal := newCancelSignal()

    router := espresso.Portafilter().
        Get("/slow", espresso.Ristretto(func() espresso.JSON[any] {
            // This handler blocks, waiting for cancellation
            return espresso.JSON[any]{}
        }))

    // Use Doppio to get access to ctx
    router.Get("/watch", espresso.Doppio(
        func(ctx context.Context, _ *espresso.JSON[struct{}]) (espresso.JSON[any], error) {
            <-ctx.Done()
            signal.markCancelled()
            return espresso.JSON[any]{}, nil
        }))

    server := httptest.NewServer(router.HTTPHandler())
    defer server.Close()

    // Start a request with a short context timeout
    ctx, cancel := context.WithCancel(context.Background())
    req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/watch", nil)

    // Start the request
    go func() {
        _, _ = http.DefaultClient.Do(req)
    }()

    // Give the server a moment to start handling
    time.Sleep(50 * time.Millisecond)

    // Cancel the client's context — should propagate to server
    cancel()

    elapsed, ok := signal.waitWithTimeout(2 * time.Second)
    if !ok {
        t.Fatalf("handler ctx.Done() did not fire within 2s")
    }
    if elapsed > 1*time.Second {
        t.Errorf("handler ctx.Done() took %v, expected < 1s", elapsed)
    }

    t.Logf("handler ctx.Done() fired after %v", elapsed)
}
```

### Step 6.3: SSE Handler Cancellation Test

```go
// TestContextCancellation_SSE verifies that client disconnect from an
// SSE stream causes ctx.Done() to fire quickly.
func TestContextCancellation_SSE(t *testing.T) {
    signal := newCancelSignal()

    router := espresso.Portafilter().
        Get("/stream", espresso.StreamSimple(
            func(ctx context.Context, s *espresso.SSEStream) error {
                // Send initial event, then wait for cancellation
                if err := s.SendData("hello"); err != nil {
                    return err
                }
                <-ctx.Done()
                signal.markCancelled()
                return nil
            }))

    server := httptest.NewServer(router.HTTPHandler())
    defer server.Close()

    ctx, cancel := context.WithCancel(context.Background())
    req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("connect failed: %v", err)
    }

    // Read the initial event to confirm stream is open
    buf := make([]byte, 128)
    _, _ = resp.Body.Read(buf)

    // Cancel the client context (simulating disconnect)
    cancel()
    _ = resp.Body.Close()

    elapsed, ok := signal.waitWithTimeout(2 * time.Second)
    if !ok {
        t.Fatalf("SSE handler ctx.Done() did not fire within 2s")
    }
    if elapsed > 1*time.Second {
        t.Errorf("SSE handler ctx.Done() took %v, expected < 1s", elapsed)
    }

    t.Logf("SSE handler ctx.Done() fired after %v", elapsed)
}
```

### Step 6.4: WebSocket Handler Cancellation Test

```go
// TestContextCancellation_WS verifies that WebSocket close causes
// handler ctx.Done() to fire quickly.
func TestContextCancellation_WS(t *testing.T) {
    signal := newCancelSignal()

    router := espresso.Portafilter().
        Get("/ws", espresso.WebSocketSimple(
            func(ctx context.Context, ws *espresso.WS) error {
                <-ctx.Done()
                signal.markCancelled()
                return nil
            }))

    server := httptest.NewServer(router.HTTPHandler())
    defer server.Close()

    wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

    conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
    if err != nil {
        t.Fatalf("dial failed: %v", err)
    }

    // Give server a moment to register the connection
    time.Sleep(50 * time.Millisecond)

    // Close the WebSocket from client side
    _ = conn.Close(websocket.StatusNormalClosure, "test done")

    elapsed, ok := signal.waitWithTimeout(2 * time.Second)
    if !ok {
        t.Fatalf("WS handler ctx.Done() did not fire within 2s")
    }
    if elapsed > 1*time.Second {
        t.Errorf("WS handler ctx.Done() took %v, expected < 1s", elapsed)
    }

    t.Logf("WS handler ctx.Done() fired after %v", elapsed)
}
```

### Step 6.5: Goroutine Propagation Test

```go
// TestContextCancellation_GoroutinePropagation verifies that cancellation
// propagates to goroutines spawned within the handler.
func TestContextCancellation_GoroutinePropagation(t *testing.T) {
    handlerDone := make(chan struct{})
    goroutineDone := make(chan struct{})

    router := espresso.Portafilter().
        Get("/multi", espresso.StreamSimple(
            func(ctx context.Context, s *espresso.SSEStream) error {
                // Spawn a goroutine that also watches ctx
                go func() {
                    <-ctx.Done()
                    close(goroutineDone)
                }()

                <-ctx.Done()
                close(handlerDone)
                return nil
            }))

    // ... connect, disconnect, verify both channels close ...
}
```

### Step 6.6: Goroutine Leak Test

```go
// TestContextCancellation_NoGoroutineLeak verifies that after
// disconnecting many times, goroutine count returns to baseline.
func TestContextCancellation_NoGoroutineLeak(t *testing.T) {
    runtime.GC()
    baseline := runtime.NumGoroutine()

    router := espresso.Portafilter().
        Get("/stream", espresso.StreamSimple(
            func(ctx context.Context, s *espresso.SSEStream) error {
                <-ctx.Done()
                return nil
            }))

    server := httptest.NewServer(router.HTTPHandler())
    defer server.Close()

    // Connect and disconnect 100 times
    for i := 0; i < 100; i++ {
        ctx, cancel := context.WithCancel(context.Background())
        req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("iteration %d: %v", i, err)
        }
        time.Sleep(10 * time.Millisecond)
        cancel()
        _ = resp.Body.Close()
    }

    // Give goroutines time to exit
    time.Sleep(1 * time.Second)
    runtime.GC()

    final := runtime.NumGoroutine()
    if final > baseline+5 {
        t.Errorf("goroutine leak: baseline=%d, final=%d (diff=%d)",
            baseline, final, final-baseline)
    }

    t.Logf("baseline=%d, final=%d", baseline, final)
}
```

## Tests Required

All tests in the sections above, at minimum:

- `TestContextCancellation_JSON`
- `TestContextCancellation_SSE`
- `TestContextCancellation_WS`
- `TestContextCancellation_GoroutinePropagation`
- `TestContextCancellation_NoGoroutineLeak`

All tests must pass with `go test -race`. Run time for the full suite should be <30 seconds.

## Documentation

Add to README or `docs/performance.md`:

````markdown
## Context Cancellation

Espresso handlers receive a `context.Context` that is cancelled when:

- The client disconnects (closes the connection)
- The server is shutting down gracefully
- The request context was already cancelled by the caller

Cancellation propagates to handlers within 1 second (typically <100ms).
Use `ctx.Done()` to clean up resources and exit when appropriate:

```go
func myHandler(ctx context.Context, req *Req, stream *espresso.SSEStream) error {
    for {
        select {
        case <-ctx.Done():
            return nil // clean exit on disconnect
        case event := <-eventChan:
            if err := stream.SendJSON("event", event); err != nil {
                return err // also triggers cleanup
            }
        }
    }
}
```

Goroutines spawned by handlers should watch `ctx.Done()` to avoid leaks:

```go
go func() {
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // do work
        }
    }
}()
```
````

## Definition of Done

- [ ] All cancellation tests pass consistently
- [ ] Propagation time verified <1 second for all handler types
- [ ] Goroutine leak test passes (100 connect/disconnect cycles, no leak)
- [ ] Behavior documented in README
- [ ] Tests run with `-race` without issues
- [ ] `CHANGELOG.md` entry noting "verified context cancellation behavior"
- [ ] Any discovered bugs filed as separate issues

## Expected Failures → Action Items

If a test fails, document it and file a separate issue. Common failure modes to watch for:

- SSE stream doesn't detect client disconnect until next Send (because detection requires a write attempt)
- WebSocket ctx cancellation is delayed by ping/pong interval
- Goroutines in handler don't exit because they don't watch `ctx.Done()`

Each of these is a real bug worth fixing, but separately from this verification task.
