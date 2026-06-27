// Package errorenvelope holds the canonical Espresso error wire format and the
// writer that serializes it. It is a stdlib-only leaf package so that both the
// root espresso package and middleware/http can produce the exact same
// {"error":{...}} shape without forming an import cycle.
//
// The dependency direction is root -> middleware/http (error.go imports
// httpmiddleware for GetRequestID), and middleware/http must NOT import root.
// That is why RecoverMiddleware, the auth middlewares, and the rate limiter
// cannot call the root package's writeErrorResponse directly. They import this
// leaf instead, mirroring how internal/validatehook breaks the root <-> extractor
// cycle.
//
// Users do not import this package directly. The user-facing structured error
// type is espresso.Error in the root package, whose writeErrorResponse delegates
// here so every framework-produced error response is byte-identical.
package errorenvelope

import (
	"encoding/json"
	"net/http"
)

// ContentType is the canonical content type for the error envelope. It matches
// exactly what the root package's writeErrorResponse sets, so responses written
// through either path carry an identical Content-Type header.
const ContentType = "application/json; charset=utf-8"

// Body is the inner error object of the canonical envelope. Its fields, JSON
// tags, and field order are kept identical to the root package's errorBody so
// that marshaling Body (wrapped in {"error": ...}) is byte-identical to the
// root package's historical output. Do not reorder fields or change tags
// without updating root's errorBody in lockstep.
type Body struct {
	// Code is a machine-readable error code (e.g. "BAD_REQUEST", "PANIC").
	Code string `json:"code"`
	// Message is a human-readable error message.
	Message string `json:"message"`
	// Details carries additional context. Omitted from the JSON when nil.
	Details map[string]any `json:"details,omitempty"`
	// RequestID echoes the request's correlation ID. Omitted when empty.
	RequestID string `json:"request_id,omitempty"`
}

// envelope is the {"error": Body} wrapper.
type envelope struct {
	Error Body `json:"error"`
}

// Write serializes {"error": body} as JSON with the canonical content type and
// the given status code. It sets Content-Type, writes the status header once,
// then streams the body — the same sequence the root package uses. It uses the
// stdlib encoding/json encoder (not sonic) so the leaf stays dependency-free.
func Write(w http.ResponseWriter, status int, body Body) {
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Error: body})
}
