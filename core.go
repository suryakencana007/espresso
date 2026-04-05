// Package espresso provides a production-grade, Axum-style HTTP routing framework for Go.
// It offers type-safe request extraction, response handling, and middleware composition
// with zero-allocation object pooling for high-performance applications.
package espresso

import "net/http"

// Layer represents a middleware function that wraps a Service with additional behavior.
// Layers can be composed to form a pipeline of processing steps (e.g., logging, timeout, authentication).
// Each Layer takes a Service and returns a new Service with the middleware applied.
//
// Example:
//
//	loggingLayer := func(next Service[Req, Res]) Service[Req, Res] {
//	    return ServiceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
//	        log.Println("request received")
//	        return next.Call(ctx, req)
//	    })
//	}
type Layer[Req any, Res any] func(Service[Req, Res]) Service[Req, Res]

// FromRequest is the interface that request types must implement to enable automatic
// data extraction from HTTP requests. This follows the Axum pattern where request
// types define how they extract data from the incoming HTTP request.
//
// Built-in Extractors (no need to implement Extract manually):
//
// For most common cases, use the built-in extractor types instead of implementing
// FromRequest yourself. These types work bidirectionally - extracting from requests
// and serializing responses.
//
//	// JSON extraction (most common for APIs)
//	func handler(ctx context.Context, req JSON[CreateUserReq]) (JSON[UserRes], error) {
//	    user := req.Data  // Data contains the decoded CreateUserReq struct
//	    return JSON[UserRes]{Data: UserRes{ID: 1}}, nil
//	}
//
//	// Query parameters
//	func handler(ctx context.Context, req extractor.Query[SearchReq]) (JSON[Results], error) {
//	    params := req.Data  // Data contains decoded query params
//	    return JSON[Results]{Data: results}, nil
//	}
//
//	// Form data
//	func handler(ctx context.Context, req extractor.Form[LoginData]) (JSON[Token], error)
//
//	// Path parameters (router sets these)
//	func handler(ctx context.Context, req extractor.Path[UserReq]) (JSON[User], error)
//
//	// HTTP headers
//	func handler(ctx context.Context, req extractor.Header[AuthReq]) (JSON[Data], error)
//
//	// Raw body bytes
//	func handler(ctx context.Context, req extractor.RawBody) (Status, error) {
//	    body := req.Data  // []byte
//	    return Status(http.StatusNoContent), nil
//	}
//
// Custom Extraction (implement Extract manually):
//
// For complex extraction logic, implement FromRequest manually:
//
//	type CreateUserReq struct {
//	    Name  string `json:"name"`
//	    Email string `json:"email"`
//	    Role  string // From query parameter
//	}
//
//	func (r *CreateUserReq) Extract(req *http.Request) error {
//	    if err := json.NewDecoder(req.Body).Decode(r); err != nil {
//	        return err
//	    }
//	    r.Role = req.URL.Query().Get("role")
//	    return nil
//	}
//
// IMPORTANT: Always use a POINTER receiver for Extract to properly populate the struct.
type FromRequest interface {
	Extract(r *http.Request) error
}

// Resettable is an optional interface that request types can implement to enable
// efficient object pooling with zero-allocation reset. This is used by the handler
// to reuse request objects from a sync.Pool.
//
// Example:
//
//	type CreateUserReq struct {
//	    Name  string `json:"name"`
//	    Email string `json:"email"`
//	}
//
//	func (r *CreateUserReq) Reset() {
//	    r.Name = ""
//	    r.Email = ""
//	}
//
// If Resettable is not implemented, the handler will use reflection to reset
// the request object, which is slower but still functional.
type Resettable interface {
	Reset()
}
