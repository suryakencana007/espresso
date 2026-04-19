# Server-Sent Events (SSE)

## Overview

Espresso provides typed SSE streaming with `espresso.SSEStream` and handler wrappers `Stream[T]()` and `StreamSimple()`. This replaces the older low-level `SSEWriter` API.

## Basic Usage

### Simple Stream

```go
func timeStream(ctx context.Context, stream *espresso.SSEStream) error {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            if err := stream.SendText("time", time.Now().Format(time.RFC3339)); err != nil {
                return err
            }
        }
    }
}

router.Get("/stream", espresso.StreamSimple(timeStream))
```

### Stream with Extractor

```go
type LogReq struct {
    App string `query:"app"`
}

func logStream(ctx context.Context, req *extractor.Query[LogReq], stream *espresso.SSEStream) error {
    app := req.Data.App
    for {
        select {
        case <-ctx.Done():
            return nil
        case entry := <-watchLogs(app):
            if err := stream.SendJSON("log", entry); err != nil {
                return err
            }
        }
    }
}

router.Get("/logs", espresso.Stream(logStream))
```

### Stream with State Injection

```go
type AppState struct {
    Broker *EventBroker
}

func eventsStream(ctx context.Context, stream *espresso.SSEStream) error {
    state := espresso.MustGetState[AppState](ctx)
    ch := state.Broker.Subscribe()
    defer state.Broker.Unsubscribe(ch)

    for {
        select {
        case <-ctx.Done():
            return nil
        case event := <-ch:
            if err := stream.SendJSON("event", event); err != nil {
                return err
            }
        }
    }
}

router.WithState(appState).Get("/events", espresso.StreamSimple(eventsStream))
```

## Configuration Options

### Keep-Alive

Send periodic keep-alive comments to prevent proxy timeouts:

```go
espresso.StreamSimple(handler, espresso.WithKeepAlive(30*time.Second))
```

### Retry Hint

Tell clients how long to wait before reconnecting:

```go
espresso.StreamSimple(handler, espresso.WithRetryHint(5*time.Second))
```

### Last-Event-ID

Clients can reconnect with the last event ID:

```go
func stream(ctx context.Context, stream *espresso.SSEStream) error {
    lastID := stream.LastEventID()
    // Resume from lastID
    // ...
}
```

## Event Methods

| Method | Description |
|--------|-------------|
| `Send(name, data)` | Send an event with name and string data |
| `SendText(name, text)` | Send a text event |
| `SendJSON(name, v)` | Send a JSON-encoded event |
| `SendData(data)` | Send a data-only event (no event name) |
| `Comment(text)` | Send a comment (not shown to client) |
| `SetRetry(d)` | Set the retry interval |
| `LastEventID()` | Get the Last-Event-ID header |
| `Context()` | Get the stream context (cancelled on disconnect) |
| `Close()` | Close the stream |

## Client-Side Usage

```javascript
const source = new EventSource('/stream');
source.addEventListener('time', (e) => {
    console.log('Time:', e.data);
});
source.onerror = () => {
    console.log('Reconnecting...');
};
```

## Reconnection Handling

When a client reconnects after a disconnect, it sends the `Last-Event-ID` header:

```go
func stream(ctx context.Context, stream *espresso.SSEStream) error {
    lastID := stream.LastEventID()
    if lastID != "" {
        // Resume from last event
        sendEventsSince(stream, lastID)
    }
    // Continue with live events
    // ...
}
```

## Concurrency

`SSEStream.Send` is safe for concurrent use. Multiple goroutines can write to the same stream:

```go
func handler(ctx context.Context, stream *espresso.SSEStream) error {
    var wg sync.WaitGroup
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            ticker := time.NewTicker(time.Second)
            defer ticker.Stop()
            for {
                select {
                case <-ctx.Done():
                    return
                case <-ticker.C:
                    _ = stream.SendText("src", fmt.Sprintf("source-%d: tick", id))
                }
            }
        }(i)
    }
    <-ctx.Done()
    wg.Wait()
    return nil
}
```

## Graceful Shutdown

On server shutdown, all SSE streams receive a final comment event and are closed. The handler's context is cancelled:

```go
func handler(ctx context.Context, stream *espresso.SSEStream) error {
    for {
        select {
        case <-ctx.Done():
            // Server is shutting down
            return nil
        case event := <-someChannel:
            _ = stream.SendJSON("event", event)
        }
    }
}
```

## SSE vs WebSocket

| Feature | SSE | WebSocket |
|---------|-----|-----------|
| Direction | Server → Client only | Bidirectional |
| Protocol | HTTP | WebSocket |
| Auto-reconnect | Built-in (EventSource) | Manual |
| Binary data | No | Yes |
| Proxy-friendly | Yes | Varies |
| Use case | Live updates, logs, feeds | Chat, terminals, games |

Choose SSE for server-push scenarios where the client doesn't need to send data over the same connection.

## See Also

- [WebSocket](websocket.md) - Bidirectional real-time communication
- [Error Handling](error-handling.md) - Structured error responses
- [Graceful Shutdown](../README.md#graceful-shutdown) - Shutdown hooks