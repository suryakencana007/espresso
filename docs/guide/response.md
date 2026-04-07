# Response Types

Espresso provides typed response types that implement `IntoResponse` interface.

## Built-in Response Types

### JSON

JSON response with status code:

```go
type User struct {
    ID    int64  `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func getUser(ctx context.Context, req *espresso.Path[UserPath]) (espresso.JSON[User], error) {
    user := findUser(req.Data.ID)
    return espresso.JSON[User]{Data: user}, nil
}

// With custom status
func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    user := createUser(req.Data)
    return espresso.JSON[User]{
        StatusCode: http.StatusCreated, // 201
        Data:       user,
    }, nil
}
```

### Text

Plain text response:

```go
func healthCheck() espresso.Text {
    return espresso.Text{Body: "OK"}
}

// With status
func notFound() espresso.Text {
    return espresso.Text{
        StatusCode: http.StatusNotFound,
        Body:       "Resource not found",
    }
}
```

### Status

Status-only response (no body):

```go
func deleteHandler(ctx context.Context, req *espresso.Path[ID]) (espresso.Status, error) {
    deleteResource(req.Data.ID)
    return espresso.Status(http.StatusNoContent), nil // 204
}
```

### Server-Sent Events (SSE)

Real-time streaming from server to client:

```go
func streamHandler(w http.ResponseWriter, r *http.Request) {
    writer := espresso.NewSSEWriter(w)
    
    // Send events
    writer.Event("message", "Hello, World!")
    writer.Event("update", `{"count": 42}`)
    writer.KeepAlive()
}

// With event ID and retry
writer.EventWithID("123", "message", "data here")

// JSON events
writer.EventJSON("data", map[string]any{"user": "john", "count": 42})

// Simple data messages
writer.Data("simple message")

// Reconnection time
writer.Retry(5000) // 5 seconds
```

#### SSE Event Format

```go
// Simple event
event: message
data: Hello, World!

// Event with ID
id: 123
event: message
data: Hello, World!

// Event with retry
event: message
retry: 5000
data: Hello, World!
```

#### Integration with Handlers

```go
func sseHandler(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers automatically
    writer := espresso.NewSSEWriter(w)
    
    // Flushing listener for real-time updates
    ctx := r.Context()
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-messages:
            writer.Event("message", msg)
        case <-time.After(30 * time.Second):
            writer.KeepAlive()
        }
    }
}

router.Get("/events", http.HandlerFunc(sseHandler))
```

## Custom Response Types

Implement `IntoResponse` interface:

```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
    Code    int    `json:"code"`
}

func (e ErrorResponse) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Error-Code", strconv.Itoa(e.Code))
    w.WriteHeader(e.Code)
    return json.NewEncoder(w).Encode(e)
}

// Usage
func handler(ctx context.Context, req *espresso.JSON[Req]) (ErrorResponse, error) {
    if err := validate(req.Data); err != nil {
        return ErrorResponse{
            Error:   "validation_error",
            Message: err.Error(),
            Code:    http.StatusBadRequest,
        }, nil
    }
    // ...
}
```

### Binary Response

```go
type BinaryResponse struct {
    Data       []byte
    Filename   string
    StatusCode int
}

func (b BinaryResponse) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "application/octet-stream")
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", b.Filename))
    status := b.StatusCode
    if status == 0 {
        status = http.StatusOK
    }
    w.WriteHeader(status)
    _, err := w.Write(b.Data)
    return err
}

// Usage
func downloadHandler(ctx context.Context, req *espresso.Path[FileReq]) (BinaryResponse, error) {
    data, err := getFile(req.Data.ID)
    if err != nil {
        return BinaryResponse{}, err
    }
    return BinaryResponse{
        Data:     data,
        Filename: "document.pdf",
    }, nil
}
```

### Streaming Response

```go
type StreamingResponse struct {
    ContentType string
    Stream      io.Reader
}

func (s StreamingResponse) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", s.ContentType)
    _, err := io.Copy(w, s.Stream)
    return err
}

// Usage
func streamVideoHandler(ctx context.Context, req *espresso.Path[VideoReq]) (StreamingResponse, error) {
    stream, err := getVideoStream(req.Data.ID)
    if err != nil {
        return StreamingResponse{}, err
    }
    return StreamingResponse{
        ContentType: "video/mp4",
        Stream:      stream,
    }, nil
}
```

### HTML Response

```go
type HTMLResponse struct {
    Body       string
    StatusCode int
}

func (h HTMLResponse) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    status := h.StatusCode
    if status == 0 {
        status = http.StatusOK
    }
    w.WriteHeader(status)
    _, err := w.Write([]byte(h.Body))
    return err
}

// Usage
func pageHandler(ctx context.Context, req *espresso.Path[PageReq]) (HTMLResponse, error) {
    body := renderTemplate(req.Data.Slug)
    return HTMLResponse{Body: body}, nil
}
```

## Error Responses

Return errors alongside responses:

```go
type APIError struct {
    Error   string `json:"error"`
    Message string `json:"message"`
}

func (e APIError) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusBadRequest)
    return json.NewEncoder(w).Encode(e)
}

func handler(ctx context.Context, req *espresso.JSON[Req]) (espresso.JSON[User], error) {
    user, err := findUser(req.Data.ID)
    if err != nil {
        // Return zero value and error
        var zero espresso.JSON[User]
        return zero, fmt.Errorf("user not found: %w", err)
    }
    return espresso.JSON[User]{Data: user}, nil
}
```

The framework handles errors by calling error handlers registered via middleware.

## Status Codes

Use standard HTTP status codes:

```go
// Success
return espresso.Status(http.StatusOK)          // 200
return espresso.JSON[User]{StatusCode: http.StatusCreated, Data: user} // 201

// Client errors
return espresso.Status(http.StatusNoContent)           // 204
return espresso.Status(http.StatusBadRequest)         // 400
return espresso.Status(http.StatusUnauthorized)        // 401
return espresso.Status(http.StatusForbidden)           // 403
return espresso.Text{StatusCode: http.StatusNotFound, Body: "not found"} // 404

// Server errors
return espresso.Text{StatusCode: http.StatusInternalServerError, Body: "internal error"} // 500
return espresso.Status(http.StatusServiceUnavailable)  // 503
```

## Redirects

```go
func redirectHandler(ctx context.Context, req *espresso.Path[Req]) (espresso.Text, error) {
    // For redirects, you can write headers directly
    return espresso.Text{}, nil
}

// Alternative: custom response type
type Redirect struct {
    URL string
}

func (r Redirect) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Location", r.URL)
    w.WriteHeader(http.StatusFound) // 302
    return nil
}

func handler(ctx context.Context, req *espresso.Path[Req]) (Redirect, error) {
    return Redirect{URL: "/new-location"}, nil
}
```

## Response Headers

Add custom headers in response types:

```go
type CachedJSON struct {
    StatusCode int
    Data       any
    ETag       string
    MaxAge     int
}

func (c CachedJSON) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("ETag", c.ETag)
    w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", c.MaxAge))
    status := c.StatusCode
    if status == 0 {
        status = http.StatusOK
    }
    w.WriteHeader(status)
    return json.NewEncoder(w).Encode(c.Data)
}
```

## Best Practices

1. **Use typed responses**: Always return specific response types
2. **Set appropriate status codes**: Default is 200 OK
3. **Set content types**: JSON, Text, HTML, etc.
4. **Handle errors gracefully**: Return typed error responses
5. **Use streaming for large data**: Don't load everything into memory
6. **Consider caching**: Add ETag/Last-Modified headers