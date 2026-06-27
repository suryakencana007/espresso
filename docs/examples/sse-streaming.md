---
title: Server-Sent Events (SSE)
description: Real-time streaming with Server-Sent Events
---

# Server-Sent Events Example

This example shows how to implement real-time streaming using Server-Sent Events (SSE).

## Basic SSE

### Simple Event Stream

Register an SSE route with `espresso.StreamSimple`. The handler receives an
`*espresso.SSEStream`; the framework sets the SSE headers, flushes after every
send, and cleans up when the client disconnects.

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/suryakencana007/espresso/v2"
)

func main() {
    router := espresso.Portafilter()

    // SSE endpoint
    router.Get("/events", espresso.StreamSimple(streamHandler))

    fmt.Println("Server starting on :8080")
    router.Brew(espresso.WithAddr(":8080"))
}

func streamHandler(ctx context.Context, stream *espresso.SSEStream) error {
    // Send events
    for i := 0; i < 10; i++ {
        if err := stream.SendText("message", fmt.Sprintf("Event %d", i)); err != nil {
            return err // client disconnected
        }
        time.Sleep(1 * time.Second)
    }
    return stream.SendText("done", "Stream complete")
}
```

### Client-Side JavaScript

```javascript
const eventSource = new EventSource('/events');

eventSource.addEventListener('message', (event) => {
    console.log('Message:', event.data);
});

eventSource.addEventListener('done', (event) => {
    console.log('Done:', event.data);
    eventSource.close();
});

eventSource.onerror = (error) => {
    console.error('SSE Error:', error);
};
```

## Integration with Handlers

### Streaming with an Extractor

Use `espresso.Stream[Req]` when the stream needs request data. The typed
request is extracted first (via the `FromRequest` interface), then the handler
is invoked with both the request and the `*espresso.SSEStream`.

```go
import "github.com/suryakencana007/espresso/v2/extractor"

type StreamRequest struct {
    Topic string `query:"topic"`
}

func topicStream(ctx context.Context, req *extractor.Query[StreamRequest], stream *espresso.SSEStream) error {
    topic := req.Data.Topic
    for {
        select {
        case <-ctx.Done():
            return nil
        case msg := <-subscribe(topic):
            if err := stream.SendText("message", msg); err != nil {
                return err
            }
        }
    }
}

// A normal GET route — SSE works over plain HTTP GET.
router.Get("/stream", espresso.Stream(topicStream))
```

## Real-Time Updates

### Counter Example

```go
func counterHandler(ctx context.Context, stream *espresso.SSEStream) error {
    for i := 1; i <= 100; i++ {
        select {
        case <-ctx.Done():
            return nil
        default:
            if err := stream.SendText("count", fmt.Sprintf("%d", i)); err != nil {
                return err
            }
            time.Sleep(100 * time.Millisecond)
        }
    }
    return stream.SendText("complete", "done")
}

router.Get("/counter", espresso.StreamSimple(counterHandler))
```

### Chat Messages

```go
// Message broker (simplified)
var messageChan = make(chan string, 100)

func chatHandler(ctx context.Context, stream *espresso.SSEStream) error {
    for {
        select {
        case <-ctx.Done():
            return nil
        case msg := <-messageChan:
            if err := stream.SendText("message", msg); err != nil {
                return err
            }
        }
    }
}

// Send-message endpoint
type SendReq struct {
    Msg string `query:"msg"`
}

func sendHandler(ctx context.Context, req *extractor.Query[SendReq]) (espresso.Text, error) {
    messageChan <- req.Data.Msg
    return espresso.Text{Body: "ok"}, nil
}

// WithKeepAlive emits a comment frame periodically so idle proxies don't drop
// the connection — no manual keepalive loop needed.
router.Get("/chat/stream", espresso.StreamSimple(chatHandler, espresso.WithKeepAlive(30*time.Second)))
router.Get("/chat/send", espresso.Doppio(sendHandler))
```

## JSON Events

### Structured Data Events

```go
type StockPrice struct {
    Symbol string  `json:"symbol"`
    Price  float64 `json:"price"`
    Time   string  `json:"time"`
}

func stockHandler(ctx context.Context, stream *espresso.SSEStream) error {
    stocks := []StockPrice{
        {Symbol: "AAPL", Price: 178.50, Time: time.Now().Format(time.RFC3339)},
        {Symbol: "GOOGL", Price: 141.80, Time: time.Now().Format(time.RFC3339)},
        {Symbol: "MSFT", Price: 378.90, Time: time.Now().Format(time.RFC3339)},
    }

    for _, stock := range stocks {
        if err := stream.SendJSON("stock", stock); err != nil {
            return err
        }
    }
    return nil
}

router.Get("/stocks", espresso.StreamSimple(stockHandler))
```

### Client-Side JSON Parsing

```javascript
const eventSource = new EventSource('/stocks');

eventSource.addEventListener('stock', (event) => {
    const stock = JSON.parse(event.data);
    console.log(`${stock.symbol}: $${stock.price}`);
});
```

## Event with ID

### Resumable Events

```go
func resumableHandler(ctx context.Context, stream *espresso.SSEStream) error {
    // Hint the client to wait 5s before reconnecting.
    _ = stream.SetRetry(5 * time.Second)

    // Resume from the client's Last-Event-ID, if it sent one.
    startID := 0
    if lastID := stream.LastEventID(); lastID != "" {
        startID, _ = strconv.Atoi(lastID)
    }

    // Send events with explicit IDs so the client can resume after a drop.
    for i := startID + 1; i <= startID+10; i++ {
        err := stream.Send(espresso.Event{
            ID:   strconv.Itoa(i),
            Name: "message",
            Data: fmt.Sprintf("Event %d", i),
        })
        if err != nil {
            return err
        }
        time.Sleep(500 * time.Millisecond)
    }
    return nil
}

router.Get("/resumable", espresso.StreamSimple(resumableHandler))
```

### Client-Side Reconnection

```javascript
let lastEventId = null;

const eventSource = new EventSource('/resumable');

eventSource.addEventListener('message', (event) => {
    lastEventId = event.lastEventId;
    console.log('Message:', event.data, 'ID:', lastEventId);
});

// On reconnection, client sends Last-Event-ID header
```

## Keep-Alive

### Preventing Timeouts

Keepalive is a stream option, not something you hand-roll in the handler. The
framework sends a comment frame on the configured interval and stops it cleanly
when the handler returns.

```go
func feedHandler(ctx context.Context, stream *espresso.SSEStream) error {
    // Emit real events as they arrive; block until the client leaves.
    // Keepalive comment frames are sent automatically (see registration).
    <-ctx.Done()
    return nil
}

router.Get("/keepalive", espresso.StreamSimple(feedHandler, espresso.WithKeepAlive(15*time.Second)))
```

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/suryakencana007/espresso/v2"
    "github.com/suryakencana007/espresso/v2/extractor"
)

// Simple message broker
type Broker struct {
    clients   map[chan string]bool
    clientsMu sync.Mutex
}

func NewBroker() *Broker {
    return &Broker{
        clients: make(map[chan string]bool),
    }
}

func (b *Broker) Subscribe() chan string {
    b.clientsMu.Lock()
    ch := make(chan string, 10)
    b.clients[ch] = true
    b.clientsMu.Unlock()
    return ch
}

func (b *Broker) Unsubscribe(ch chan string) {
    b.clientsMu.Lock()
    delete(b.clients, ch)
    close(ch)
    b.clientsMu.Unlock()
}

func (b *Broker) Publish(msg string) {
    b.clientsMu.Lock()
    for ch := range b.clients {
        select {
        case ch <- msg:
        default:
        }
    }
    b.clientsMu.Unlock()
}

var broker = NewBroker()

func main() {
    router := espresso.Portafilter()

    // SSE stream endpoint (keepalive handled by the framework)
    router.Get("/stream", espresso.StreamSimple(streamHandler, espresso.WithKeepAlive(30*time.Second)))

    // Publish endpoint
    router.Post("/publish", espresso.Doppio(publishHandler))

    fmt.Println("Server starting on :8080")
    router.Brew(espresso.WithAddr(":8080"))
}

func streamHandler(ctx context.Context, stream *espresso.SSEStream) error {
    ch := broker.Subscribe()
    defer broker.Unsubscribe(ch)

    for {
        select {
        case <-ctx.Done():
            return nil
        case msg := <-ch:
            if err := stream.SendText("message", msg); err != nil {
                return err
            }
        }
    }
}

type PublishReq struct {
    Msg string `query:"msg"`
}

func publishHandler(ctx context.Context, req *extractor.Query[PublishReq]) (espresso.Text, error) {
    if req.Data.Msg == "" {
        return espresso.Text{}, espresso.ErrBadRequest("missing msg parameter")
    }
    broker.Publish(req.Data.Msg)
    return espresso.Text{Body: "ok"}, nil
}
```

### Test the Example

```bash
# Start server
go run main.go

# Stream events
curl -N http://localhost:8080/stream

# Publish message
curl -X POST "http://localhost:8080/publish?msg=Hello"
```

### Client Example

```html
<!DOCTYPE html>
<html>
<head>
    <title>SSE Demo</title>
</head>
<body>
    <h1>Server-Sent Events</h1>
    <div id="messages"></div>
    
    <script>
        const eventSource = new EventSource('/stream');
        
        eventSource.addEventListener('message', (event) => {
            const msg = document.createElement('div');
            msg.textContent = event.data;
            document.getElementById('messages').appendChild(msg);
        });
        
        eventSource.onerror = (error) => {
            console.error('SSE Error:', error);
        };
    </script>
</body>
</html>
```

## See Also

- [Response Types Guide](/guide/response) - SSE response type
- [Handlers Guide](/guide/handlers) - Handler patterns
- [Production Example](/examples/production) - Production setup
