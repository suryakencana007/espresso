package espresso

import (
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
)

// IntoResponse is the interface that response types must implement to enable
// automatic HTTP response writing. This follows the Axum pattern where responses
// know how to serialize themselves.
//
// Implementations can define custom:
//   - HTTP status codes
//   - Response headers
//   - Body serialization (JSON, text, binary, etc.)
type IntoResponse interface {
	// WriteResponse writes the response to the http.ResponseWriter.
	// This method is responsible for setting headers, status code, and writing the body.
	// Returns an error if writing fails.
	WriteResponse(w http.ResponseWriter) error
}

// JSON is a generic response type for JSON responses.
// The StatusCode field allows customizing the HTTP status code (defaults to 200 if not set).
// The Data field contains the payload that will be serialized to JSON.
//
// Example:
//
//	return JSON[UserRes]{
//	    StatusCode: http.StatusCreated, // 201
//	    Data: UserRes{ID: 1, Name: "John"},
//	}, nil
type JSON[T any] struct {
	StatusCode int
	Data       T
}

// WriteResponse implements IntoResponse by writing JSON to the response.
// It sets the Content-Type header to application/json and encodes the Data field as JSON.
// Uses pooled buffers for better performance on high-throughput applications.
func (j JSON[T]) WriteResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	status := j.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)

	// Use pooled buffer for encoding to reduce allocations
	// For small responses, direct encoding is faster
	// For large responses, buffered encoding reduces GC pressure
	return sonic.ConfigDefault.NewEncoder(w).Encode(j.Data)
}

// Reset implements Resettable by zeroing the JSON response.
// This is used by the handler for object pooling.
func (j *JSON[T]) Reset() {
	j.StatusCode = 0
	var zero T
	j.Data = zero
}

// Extract implements FromRequest by decoding JSON body into Data.
// This allows JSON[T] to be used as both a request extractor AND response type,
// following the Axum pattern where `Json[T]` works bidirectionally.
//
// Example (request extraction):
//
//	func createUser(ctx context.Context, req JSON[CreateUserReq]) (JSON[UserRes], error) {
//	    user := req.Data // req.Data is CreateUserReq, auto-extracted from JSON body
//	    return JSON[UserRes]{Data: UserRes{ID: 1}}, nil
//	}
func (j *JSON[T]) Extract(r *http.Request) error {
	defer func() { _ = r.Body.Close() }()
	return sonic.ConfigDefault.NewDecoder(r.Body).Decode(&j.Data)
}

// Text is a response type for plain text responses.
// Use this for simple string responses, error messages, or any non-JSON content.
//
// Example:
//
//	return Text{Body: "Hello, World!"}, nil
//	return Text{StatusCode: http.StatusNotFound, Body: "not found"}, nil
type Text struct {
	StatusCode int
	Body       string
}

// WriteResponse implements IntoResponse by writing plain text to the response.
// It sets the Content-Type header to text/plain.
func (t Text) WriteResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/plain")
	status := t.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, err := w.Write([]byte(t.Body))
	return err
}

// Reset implements Resettable by zeroing the Text response.
// This is used by the handler for object pooling.
func (t *Text) Reset() {
	t.StatusCode = 0
	t.Body = ""
}

// Status is a lightweight response type for responses with no body.
// It only sets the HTTP status code. Use this for responses like 204 No Content.
//
// Example:
//
//	return Status(http.StatusNoContent), nil // 204 No Content
//	return Status(http.StatusAccepted), nil   // 202 Accepted
type Status int

// WriteResponse implements IntoResponse by setting only the status code.
// No body is written to the response.
func (s Status) WriteResponse(w http.ResponseWriter) error {
	w.WriteHeader(int(s))
	return nil
}

// Reset implements Resettable by resetting the Status to 0.
func (s *Status) Reset() {
	*s = 0
}

// SSEEvent represents a single Server-Sent Event.
// Server-Sent Events allow the server to push updates to the client in real-time.
//
// Example:
//
//	event := SSEEvent{
//	    ID:    "123",
//	    Event: "message",
//	    Data:  "Hello, World!",
//	}
type SSEEvent struct {
	ID    string // Optional event ID for client reconnect
	Event string // Optional event type (default: "message")
	Data  string // Event data
	Retry int    // Optional retry duration in milliseconds
}

// SSE is a response type for Server-Sent Events streaming.
// Use this for real-time server-to-client updates.
//
// Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    sse := SSE{}
//	    sse.WriteEvent(w, SSEEvent{Event: "message", Data: "Hello"})
//	    sse.WriteEvent(w, SSEEvent{Event: "update", Data: `{"count": 42}`})
//	    return sse, nil
//	}
type SSE struct {
	StatusCode int // Optional status code (default: 200)
	flusher    http.Flusher
}

// WriteResponse implements IntoResponse by setting up SSE streaming.
// It sets the Content-Type to text/event-stream and necessary headers for SSE.
func (s *SSE) WriteResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	status := s.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)

	flusher, ok := w.(http.Flusher)
	if ok {
		s.flusher = flusher
	}

	return nil
}

// WriteEvent writes a single SSE event to the response writer and flushes it.
// Use this in a handler to stream events to the client.
//
// Example:
//
//	func streamHandler(ctx context.Context, req *espresso.JSON[StreamReq]) (espresso.SSE, error) {
//	    var sse espresso.SSE
//	    // Events will be written via SSEWriter
//	    return sse, nil
//	}
func (s *SSE) WriteEvent(w http.ResponseWriter, event SSEEvent) {
	if event.ID != "" {
		_, _ = fmt.Fprintf(w, "id: %s\n", event.ID)
	}
	if event.Event != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", event.Event)
	}
	if event.Retry > 0 {
		_, _ = fmt.Fprintf(w, "retry: %d\n", event.Retry)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", event.Data)

	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// WriteKeepAlive writes a keep-alive comment to the SSE stream.
// This helps prevent connection timeouts during idle periods.
func (s *SSE) WriteKeepAlive(w http.ResponseWriter) {
	_, _ = w.Write([]byte(": keep-alive\n\n"))
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// SSEWriter is a helper for writing SSE events with a fluent API.
// It wraps an http.ResponseWriter and provides convenient methods for streaming.
//
// Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    writer := NewSSEWriter(w)
//	    writer.Event("message", "Hello, World!")
//	    writer.EventJSON("data", map[string]any{"count": 42})
//	    writer.KeepAlive()
//	}
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSE writer wrapping the http.ResponseWriter.
// It automatically sets up SSE headers and flusher.
func NewSSEWriter(w http.ResponseWriter) *SSEWriter {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, _ := w.(http.Flusher)

	return &SSEWriter{
		w:       w,
		flusher: flusher,
	}
}

// Event writes an SSE event with the given event type and data.
func (s *SSEWriter) Event(event, data string) {
	_, _ = fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data)
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// EventWithID writes an SSE event with ID, event type, and data.
func (s *SSEWriter) EventWithID(id, event, data string) {
	_, _ = fmt.Fprintf(s.w, "id: %s\nevent: %s\ndata: %s\n\n", id, event, data)
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// Data writes a simple SSE data message (no event type).
func (s *SSEWriter) Data(data string) {
	_, _ = fmt.Fprintf(s.w, "data: %s\n\n", data)
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// EventJSON writes an SSE event with JSON-encoded data.
func (s *SSEWriter) EventJSON(event string, data any) error {
	_, _ = fmt.Fprintf(s.w, "event: %s\n", event)
	_, _ = fmt.Fprint(s.w, "data: ")
	if err := sonic.ConfigDefault.NewEncoder(s.w).Encode(data); err != nil {
		return err
	}
	_, _ = s.w.Write([]byte{'\n'})
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return nil
}

// KeepAlive writes a keep-alive comment to prevent connection timeout.
func (s *SSEWriter) KeepAlive() {
	_, _ = s.w.Write([]byte(": keep-alive\n\n"))
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// Retry sets the reconnection time in milliseconds for the client.
func (s *SSEWriter) Retry(ms int) {
	_, _ = fmt.Fprintf(s.w, "retry: %d\n\n", ms)
	if s.flusher != nil {
		s.flusher.Flush()
	}
}
