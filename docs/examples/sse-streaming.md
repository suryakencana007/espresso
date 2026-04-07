---
title: Server-Sent Events (SSE)
description: Real-time streaming with Server-Sent Events
---

# Server-Sent Events Example

This example shows how to implement real-time streaming using Server-Sent Events (SSE).

## Basic SSE

### Simple Event Stream

```go
package main

import (
    "fmt"
    "net/http"
    "time"
    
    "github.com/suryakencana007/espresso"
)

func main() {
    router := espresso.Portafilter()
    
    // SSE endpoint
    router.Get("/events", http.HandlerFunc(streamHandler))
    
    fmt.Println("Server starting on :8080")
    router.Brew(espresso.WithAddr(":8080"))
}

func streamHandler(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    writer := espresso.NewSSEWriter(w)
    
    // Send events
    for i := 0; i < 10; i++ {
        writer.Event("message", fmt.Sprintf("Event %d", i))
        time.Sleep(1 * time.Second)
    }
    
    writer.Event("done", "Stream complete")
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

### Using SSE Type

```go
type StreamRequest struct {
    Topic string `query:"topic"`
}

func sseHandler(ctx context.Context, req *espresso.JSON[StreamRequest]) (*espresso.SSE, error) {
    // SSE type is used for streaming endpoints
    return &espresso.SSE{}, nil
}

// Route with handler pattern
router.Get("/stream", espresso.Doppio(sseHandler))
```

## Real-Time Updates

### Counter Example

```go
func counterHandler(w http.ResponseWriter, r *http.Request) {
    writer := espresso.NewSSEWriter(w)
    
    ctx := r.Context()
    for i := 1; i <= 100; i++ {
        select {
        case <-ctx.Done():
            return
        default:
            writer.Event("count", fmt.Sprintf("%d", i))
            time.Sleep(100 * time.Millisecond)
        }
    }
    
    writer.Event("complete", "done")
}

router.Get("/counter", http.HandlerFunc(counterHandler))
```

### Chat Messages

```go
// Message broker (simplified)
var messageChan = make(chan string, 100)

func chatHandler(w http.ResponseWriter, r *http.Request) {
    writer := espresso.NewSSEWriter(w)
    
    ctx := r.Context()
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-messageChan:
            writer.Event("message", msg)
        case <-time.After(30 * time.Second):
            writer.KeepAlive()
        }
    }
}

// Send message endpoint
func sendHandler(w http.ResponseWriter, r *http.Request) {
    msg := r.URL.Query().Get("msg")
    messageChan <- msg
    w.WriteHeader(http.StatusOK)
}

router.Get("/chat/stream", http.HandlerFunc(chatHandler))
router.Get("/chat/send", http.HandlerFunc(sendHandler))
```

## JSON Events

### Structured Data Events

```go
type StockPrice struct {
    Symbol string  `json:"symbol"`
    Price  float64 `json:"price"`
    Time   string  `json:"time"`
}

func stockHandler(w http.ResponseWriter, r *http.Request) {
    writer := espresso.NewSSEWriter(w)
    
    stocks := []StockPrice{
        {Symbol: "AAPL", Price: 178.50, Time: time.Now().Format(time.RFC3339)},
        {Symbol: "GOOGL", Price: 141.80, Time: time.Now().Format(time.RFC3339)},
        {Symbol: "MSFT", Price: 378.90, Time: time.Now().Format(time.RFC3339)},
    }
    
    for _, stock := range stocks {
        writer.EventJSON("stock", stock)
    }
}

router.Get("/stocks", http.HandlerFunc(stockHandler))
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
func resumableHandler(w http.ResponseWriter, r *http.Request) {
    writer := espresso.NewSSEWriter(w)
    
    // Set retry time
    writer.Retry(5000) // 5 seconds
    
    // Get last event ID from client
    lastID := r.Header.Get("Last-Event-ID")
    startID := 0
    if lastID != "" {
        startID, _ = strconv.Atoi(lastID)
    }
    
    // Send events with IDs
    for i := startID + 1; i <= startID+10; i++ {
        writer.EventWithID(
            fmt.Sprintf("%d", i),
            "message",
            fmt.Sprintf("Event %d", i),
        )
        time.Sleep(500 * time.Millisecond)
    }
}

router.Get("/resumable", http.HandlerFunc(resumableHandler))
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

```go
func keepAliveHandler(w http.ResponseWriter, r *http.Request) {
    writer := espresso.NewSSEWriter(w)
    
    ctx := r.Context()
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            writer.KeepAlive()
        }
    }
}

router.Get("/keepalive", http.HandlerFunc(keepAliveHandler))
```

## Complete Example

```go
package main

import (
    "fmt"
    "net/http"
    "sync"
    "time"
    
    "github.com/suryakencana007/espresso"
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
    
    // SSE stream endpoint
    router.Get("/stream", http.HandlerFunc(streamHandler))
    
    // Publish endpoint
    router.Post("/publish", http.HandlerFunc(publishHandler))
    
    fmt.Println("Server starting on :8080")
    router.Brew(espresso.WithAddr(":8080"))
}

func streamHandler(w http.ResponseWriter, r *http.Request) {
    writer := espresso.NewSSEWriter(w)
    
    ch := broker.Subscribe()
    defer broker.Unsubscribe(ch)
    
    ctx := r.Context()
    keepAlive := time.NewTicker(30 * time.Second)
    defer keepAlive.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-ch:
            writer.Event("message", msg)
        case <-keepAlive.C:
            writer.KeepAlive()
        }
    }
}

func publishHandler(w http.ResponseWriter, r *http.Request) {
    msg := r.URL.Query().Get("msg")
    if msg == "" {
        http.Error(w, "missing msg parameter", http.StatusBadRequest)
        return
    }
    
    broker.Publish(msg)
    w.WriteHeader(http.StatusOK)
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