package espresso

import (
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
