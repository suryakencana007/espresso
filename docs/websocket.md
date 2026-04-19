# WebSocket Handlers

## Overview

Espresso provides first-class WebSocket support with the same type-safe patterns used for HTTP handlers. WebSocket handlers receive a `*espresso.WS` connection object and can use extractors (path params, query params, state injection) just like regular handlers.

## Basic Usage

### Simple WebSocket (No Extractor)

```go
func echoHandler(ctx context.Context, ws *espresso.WS) error {
    for {
        _, msg, err := ws.Read(ctx)
        if err != nil {
            return nil // client disconnected
        }
        if err := ws.WriteText(ctx, string(msg)); err != nil {
            return err
        }
    }
}

router.Get("/ws/echo", espresso.WebSocketSimple(echoHandler))
```

### WebSocket with Path Extractor

```go
type RoomReq struct {
    Room string `path:"room"`
}

func chatHandler(ctx context.Context, req *extractor.Path[RoomReq], ws *espresso.WS) error {
    room := req.Data.Room
    for {
        _, msg, err := ws.Read(ctx)
        if err != nil {
            return nil
        }
        broadcast(room, msg)
    }
}

router.Get("/ws/chat/{room}", espresso.WebSocket(chatHandler))
```

### WebSocket with State Injection

```go
type AppState struct {
    Hub *ChatHub
}

func chatHandler(ctx context.Context, ws *espresso.WS) error {
    state := espresso.MustGetState[AppState](ctx)
    state.Hub.Register(ws)
    defer state.Hub.Unregister(ws)
    // ...
}

router.WithState(appState).Get("/ws/chat", espresso.WebSocketSimple(chatHandler))
```

## Configuration Options

### Ping/Pong Keepalive

```go
espresso.WebSocketSimple(handler, espresso.WithPingInterval(30*time.Second))
```

Default: 30 seconds. Set to 0 to disable pings.

### Read/Write Limits

```go
espresso.WebSocketSimple(handler,
    espresso.WithPingInterval(30*time.Second),
)
```

The `WS` type handles message fragmentation and encoding automatically.

### Subprotocols

```go
espresso.WebSocketSimple(handler,
    espresso.WithSubprotocols("proto1", "proto2"),
)
```

Check the negotiated subprotocol in the handler:

```go
func handler(ctx context.Context, ws *espresso.WS) error {
    proto := ws.Subprotocol()
    // ...
}
```

## Methods

| Method | Description |
|--------|-------------|
| `Read(ctx)` | Read a message, returns (MessageType, []byte, error) |
| `Write(ctx, msgType, data)` | Write a message |
| `WriteText(ctx, text)` | Write a text message |
| `WriteBinary(ctx, data)` | Write a binary message |
| `WriteJSON(ctx, v)` | Write a JSON-encoded message |
| `ReadJSON(ctx, v)` | Read and decode a JSON message |
| `Close(code, reason)` | Close the connection |
| `Context()` | Get the connection context (cancelled on disconnect) |
| `Subprotocol()` | Get the negotiated subprotocol |

## Error Handling

Handler errors are logged but don't crash the server. Return `nil` for clean disconnects:

```go
func handler(ctx context.Context, ws *espresso.WS) error {
    for {
        _, _, err := ws.Read(ctx)
        if err != nil {
            return nil // clean disconnect
        }
    }
}
```

## Graceful Shutdown

On server shutdown, all WebSocket connections are closed with code 1001 (Going Away) and reason "server shutting down". The handler's context is cancelled, allowing cleanup:

```go
func handler(ctx context.Context, ws *espresso.WS) error {
    for {
        select {
        case <-ctx.Done():
            // Server is shutting down
            return nil
        case msg := <-someChannel:
            ws.WriteText(ctx, msg)
        }
    }
}
```

## Testing

```go
func TestWebSocket_Echo(t *testing.T) {
    handler := func(ctx context.Context, ws *espresso.WS) error {
        _, msg, err := ws.Read(ctx)
        if err != nil {
            return nil
        }
        return ws.WriteText(ctx, "Echo: "+string(msg))
    }

    router := espresso.Portafilter().Get("/ws", espresso.WebSocketSimple(handler))
    server := httptest.NewServer(router)
    defer server.Close()

    wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
    conn, resp, err := websocket.Dial(context.Background(), wsURL, nil)
    if err != nil {
        t.Fatal(err)
    }
    if resp != nil && resp.Body != nil {
        _ = resp.Body.Close()
    }
    defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _ = conn.Write(ctx, websocket.MessageText, []byte("hello"))

    _, data, err := conn.Read(ctx)
    if string(data) != "Echo: hello" {
        t.Errorf("expected 'Echo: hello', got %s", data)
    }
}
```

## Performance Considerations

- The `Read` method runs a background goroutine to detect disconnection
- `Write` methods are safe for concurrent use from multiple goroutines
- Ping/pong uses proper WebSocket ping frames (not text messages)
- Connection registry enables graceful shutdown of all active connections

## See Also

- [Streaming (SSE)](streaming.md) - Server-sent events alternative
- [Error Handling](error-handling.md) - Structured error responses
- [Graceful Shutdown](../README.md#graceful-shutdown) - Shutdown hooks