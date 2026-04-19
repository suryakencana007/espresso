# Task 3: Structured Error Response

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 2-3 days
**Dependencies:** None (can be done in parallel with Tasks 1 and 2)

## Context

Barista will have dozens of endpoints that return errors. The frontend needs a consistent JSON format for error handling, so it can:

- Display user-friendly error messages
- Route based on error codes (e.g., show "Not Found" page for `PROJECT_NOT_FOUND`)
- Show structured details (e.g., which field failed validation)
- Include request IDs for support/debugging

Espresso currently has an `error.go` file but lacks a clear, opinionated format for error responses. This task establishes that format as first-class.

### Why a Custom Format Instead of RFC 7807?

RFC 7807 (Problem Details) is a reasonable standard, but we're choosing a custom Espresso format because:

- We want full control over field names and shape
- We want built-in integration with request ID tracing
- We want structured `details` that can be any JSON value, not a flat map
- We want clear separation between user-facing fields and internal-only fields

Users who prefer RFC 7807 can map `*espresso.Error` to that format in their own code.

## Acceptance Criteria

- [ ] `espresso.Error` type with a fluent builder API
- [ ] Fields: `StatusCode`, `Code`, `Message`, `Details`, `Internal` (not exposed in JSON)
- [ ] Auto-serializes to a consistent JSON format
- [ ] Request ID auto-included if `RequestIDMiddleware` is active
- [ ] Handlers can return `*espresso.Error` directly; the framework handles serialization and status code
- [ ] Panic recovery wraps to `*espresso.Error` with code `INTERNAL`
- [ ] Existing handlers returning plain `error` still work (fallback to generic 500 error)
- [ ] Common error constructors provided: `ErrNotFound`, `ErrBadRequest`, `ErrUnauthorized`, etc.

## Technical Approach

### Step 3.1: Refactor error.go

Update `error.go` with the structured Error type:

```go
package espresso

import (
    "encoding/json"
    "fmt"
    "net/http"
)

// Error is Espresso's structured error type for HTTP responses.
// Handlers can return *Error to produce a consistent JSON error response
// with proper HTTP status code.
//
// Use the builder methods (WithCode, WithDetail, Wrap) to populate fields.
type Error struct {
    // StatusCode is the HTTP status code to return. Required.
    StatusCode int `json:"-"`

    // Code is a machine-readable error code (e.g., "PROJECT_NOT_FOUND").
    // Intended for programmatic error handling by clients.
    Code string `json:"code"`

    // Message is a human-readable error message.
    Message string `json:"message"`

    // Details contains additional error context (e.g., field validation errors).
    // Rendered as JSON object. Omitted if nil.
    Details map[string]any `json:"details,omitempty"`

    // Internal is an error wrapped by this Error for logging purposes.
    // Never exposed in the JSON response. Access via Unwrap().
    Internal error `json:"-"`
}

// NewError creates a new Error with the given HTTP status code and message.
// Use builder methods to add code, details, and internal errors.
//
// Example:
//
//	return espresso.NewError(http.StatusNotFound, "project not found").
//	    WithCode("PROJECT_NOT_FOUND").
//	    WithDetail("projectId", id)
func NewError(statusCode int, message string) *Error {
    return &Error{
        StatusCode: statusCode,
        Code:       defaultCodeForStatus(statusCode),
        Message:    message,
    }
}

// WithCode sets the error code and returns the Error for chaining.
func (e *Error) WithCode(code string) *Error {
    e.Code = code
    return e
}

// WithDetail adds a key-value pair to the Details map and returns the Error.
func (e *Error) WithDetail(key string, value any) *Error {
    if e.Details == nil {
        e.Details = make(map[string]any)
    }
    e.Details[key] = value
    return e
}

// WithDetails replaces the Details map and returns the Error.
func (e *Error) WithDetails(details map[string]any) *Error {
    e.Details = details
    return e
}

// Wrap attaches an internal error that will be logged but not exposed to
// the client. Use for wrapping underlying errors (e.g., database errors).
func (e *Error) Wrap(err error) *Error {
    e.Internal = err
    return e
}

// Error implements the error interface.
// Returns a string representation suitable for logging, including the
// internal error if present.
func (e *Error) Error() string {
    if e.Internal != nil {
        return fmt.Sprintf("%d %s: %s (internal: %v)", e.StatusCode, e.Code, e.Message, e.Internal)
    }
    return fmt.Sprintf("%d %s: %s", e.StatusCode, e.Code, e.Message)
}

// Unwrap returns the internal wrapped error, if any.
// Supports errors.Is and errors.As.
func (e *Error) Unwrap() error {
    return e.Internal
}

// Common error constructors.
// These set both the StatusCode and a default Code that can be overridden.

// ErrBadRequest creates a 400 Bad Request error.
func ErrBadRequest(message string) *Error {
    return NewError(http.StatusBadRequest, message).WithCode("BAD_REQUEST")
}

// ErrUnauthorized creates a 401 Unauthorized error.
func ErrUnauthorized(message string) *Error {
    return NewError(http.StatusUnauthorized, message).WithCode("UNAUTHORIZED")
}

// ErrForbidden creates a 403 Forbidden error.
func ErrForbidden(message string) *Error {
    return NewError(http.StatusForbidden, message).WithCode("FORBIDDEN")
}

// ErrNotFound creates a 404 Not Found error.
func ErrNotFound(message string) *Error {
    return NewError(http.StatusNotFound, message).WithCode("NOT_FOUND")
}

// ErrConflict creates a 409 Conflict error.
func ErrConflict(message string) *Error {
    return NewError(http.StatusConflict, message).WithCode("CONFLICT")
}

// ErrUnprocessableEntity creates a 422 Unprocessable Entity error.
// Typically used for validation failures.
func ErrUnprocessableEntity(message string) *Error {
    return NewError(http.StatusUnprocessableEntity, message).WithCode("UNPROCESSABLE_ENTITY")
}

// ErrTooManyRequests creates a 429 Too Many Requests error.
func ErrTooManyRequests(message string) *Error {
    return NewError(http.StatusTooManyRequests, message).WithCode("TOO_MANY_REQUESTS")
}

// ErrInternal creates a 500 Internal Server Error.
func ErrInternal(message string) *Error {
    return NewError(http.StatusInternalServerError, message).WithCode("INTERNAL")
}

// ErrServiceUnavailable creates a 503 Service Unavailable error.
func ErrServiceUnavailable(message string) *Error {
    return NewError(http.StatusServiceUnavailable, message).WithCode("SERVICE_UNAVAILABLE")
}

// defaultCodeForStatus returns a default error code for an HTTP status.
// Used when NewError is called directly without a code.
func defaultCodeForStatus(status int) string {
    switch status {
    case http.StatusBadRequest:
        return "BAD_REQUEST"
    case http.StatusUnauthorized:
        return "UNAUTHORIZED"
    case http.StatusForbidden:
        return "FORBIDDEN"
    case http.StatusNotFound:
        return "NOT_FOUND"
    case http.StatusConflict:
        return "CONFLICT"
    case http.StatusUnprocessableEntity:
        return "UNPROCESSABLE_ENTITY"
    case http.StatusTooManyRequests:
        return "TOO_MANY_REQUESTS"
    case http.StatusInternalServerError:
        return "INTERNAL"
    case http.StatusServiceUnavailable:
        return "SERVICE_UNAVAILABLE"
    default:
        return "ERROR"
    }
}
```

### Step 3.2: JSON Response Format

The standard error response format:

```json
{
  "error": {
    "code": "PROJECT_NOT_FOUND",
    "message": "project with id 'abc123' does not exist",
    "details": {
      "projectId": "abc123"
    },
    "request_id": "req_01H8XYZ..."
  }
}
```

Notes:

- Top-level object always has a single `error` key (distinguishes from success responses)
- `code` and `message` are always present
- `details` is omitted if empty
- `request_id` is included if available from context (set by `RequestIDMiddleware`)

Create an internal `errorResponse` struct for serialization:

```go
// errorResponse is the JSON wrapper for error responses.
type errorResponse struct {
    Error errorBody `json:"error"`
}

type errorBody struct {
    Code      string         `json:"code"`
    Message   string         `json:"message"`
    Details   map[string]any `json:"details,omitempty"`
    RequestID string         `json:"request_id,omitempty"`
}

// writeErrorResponse writes the error as a JSON response.
// Intended to be called by handler wrappers, not user code.
func writeErrorResponse(w http.ResponseWriter, r *http.Request, err *Error) {
    body := errorResponse{
        Error: errorBody{
            Code:      err.Code,
            Message:   err.Message,
            Details:   err.Details,
            RequestID: getRequestID(r.Context()), // helper from request ID middleware
        },
    }

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(err.StatusCode)
    _ = json.NewEncoder(w).Encode(body)
}
```

### Step 3.3: Handler Integration

Modify the existing handler wrappers (`Ristretto`, `Solo`, `Doppio`) to handle `*Error` return values:

```go
// Example modification to the Doppio wrapper's error handling path.
// The exact location depends on current implementation; find where handler
// errors are currently converted to HTTP responses.

func handleHandlerError(w http.ResponseWriter, r *http.Request, err error) {
    var espErr *Error
    if errors.As(err, &espErr) {
        // It's already a structured error — use it directly
        writeErrorResponse(w, r, espErr)
        return
    }

    // Plain error — wrap it as an internal server error
    wrapped := ErrInternal("internal server error").Wrap(err)
    writeErrorResponse(w, r, wrapped)

    // Log the internal error for debugging (don't expose to client)
    logInternalError(r.Context(), wrapped)
}
```

Use `errors.As` (not type assertion) so wrapped errors are handled correctly:

```go
// This works correctly even if the error is wrapped:
// return fmt.Errorf("failed: %w", ErrNotFound("project"))
```

### Step 3.4: Panic Recovery Integration

Update `RecoverMiddleware` (or the equivalent panic recovery point) to wrap panics as structured errors:

```go
func RecoverMiddleware() Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    // Capture stack trace for logging
                    stack := debug.Stack()

                    var panicErr error
                    switch v := rec.(type) {
                    case error:
                        panicErr = v
                    default:
                        panicErr = fmt.Errorf("panic: %v", v)
                    }

                    espErr := ErrInternal("internal server error").
                        WithCode("PANIC").
                        Wrap(panicErr)

                    // Log panic with stack trace
                    logPanic(r.Context(), espErr, stack)

                    writeErrorResponse(w, r, espErr)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}
```

## File Structure

### Modified Files

- `error.go` — Refactor to include structured `Error` type and builders
- `handler.go` — Update handler wrappers to detect and serialize `*Error`
- `middleware/http/recover.go` (or similar) — Update panic recovery
- `README.md` — Add "Error Handling" section
- `CHANGELOG.md` — Add entry

### New Files

- `error_test.go` (if not already exists) — Tests for Error builder and serialization

## Tests Required

```go
// Tests the fluent builder API chains correctly.
func TestError_Builder(t *testing.T)

// Tests JSON serialization format matches spec.
func TestError_Serialization(t *testing.T)

// Tests that request ID is included when middleware is active.
func TestError_RequestIDIncluded(t *testing.T)

// Tests that request ID is omitted when not available.
func TestError_RequestIDOmitted(t *testing.T)

// Tests that internal errors are wrapped and accessible via Unwrap.
func TestError_WrapInternal(t *testing.T)

// Tests errors.Is and errors.As work correctly.
func TestError_ErrorsIs(t *testing.T)
func TestError_ErrorsAs(t *testing.T)

// Tests handler returning *Error produces correct HTTP response.
func TestError_HandlerReturnsError(t *testing.T)

// Tests handler returning plain error produces 500 response.
func TestError_HandlerReturnsPlainError(t *testing.T)

// Tests handler returning wrapped *Error (via fmt.Errorf %w) is unwrapped.
func TestError_HandlerReturnsWrappedError(t *testing.T)

// Tests panic in handler produces 500 response with PANIC code.
func TestError_PanicRecovered(t *testing.T)

// Tests that panic with error value preserves the error.
func TestError_PanicWithError(t *testing.T)

// Tests that StatusCode is set correctly for common errors.
func TestError_StatusCodeSet(t *testing.T)

// Tests all Err* constructors produce correct status + code.
func TestError_Constructors(t *testing.T)

// Tests Details field serializes correctly when populated.
func TestError_WithDetails(t *testing.T)

// Tests that internal error is NOT exposed in JSON response.
func TestError_InternalNotExposed(t *testing.T)
```

Aim for ≥90% coverage on `error.go`.

## Example Usage

Add to README in the "Error Handling" section:

````markdown
## Error Handling

Return `*espresso.Error` from handlers for consistent error responses:

```go
func getProject(ctx context.Context, req *extractor.Path[GetProjectReq]) (espresso.JSON[Project], error) {
    state := espresso.MustGetState[AppState](ctx)

    project, err := state.DB.FindProject(req.Data.ID)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            return espresso.JSON[Project]{},
                espresso.ErrNotFound("project not found").
                    WithCode("PROJECT_NOT_FOUND").
                    WithDetail("projectId", req.Data.ID)
        }
        return espresso.JSON[Project]{},
            espresso.ErrInternal("database error").Wrap(err)
    }

    return espresso.JSON[Project]{Data: project}, nil
}
```

Response body:

```json
{
  "error": {
    "code": "PROJECT_NOT_FOUND",
    "message": "project not found",
    "details": {
      "projectId": "abc123"
    },
    "request_id": "req_01H8XYZ..."
  }
}
```

### Common Constructors

| Function | Status | Default Code |
|----------|--------|--------------|
| `ErrBadRequest` | 400 | `BAD_REQUEST` |
| `ErrUnauthorized` | 401 | `UNAUTHORIZED` |
| `ErrForbidden` | 403 | `FORBIDDEN` |
| `ErrNotFound` | 404 | `NOT_FOUND` |
| `ErrConflict` | 409 | `CONFLICT` |
| `ErrUnprocessableEntity` | 422 | `UNPROCESSABLE_ENTITY` |
| `ErrTooManyRequests` | 429 | `TOO_MANY_REQUESTS` |
| `ErrInternal` | 500 | `INTERNAL` |
| `ErrServiceUnavailable` | 503 | `SERVICE_UNAVAILABLE` |

### Wrapping Internal Errors

Use `Wrap()` to attach errors that shouldn't be exposed to clients:

```go
return espresso.ErrInternal("failed to save").Wrap(dbErr)
```

The wrapped error is logged but not sent to the client.
````

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked
- [ ] All tests pass with `go test -race`, coverage ≥90% on `error.go`
- [ ] Existing handlers still work without modification (backward compatibility verified)
- [ ] Example added to README
- [ ] Godoc comments with examples on all public API
- [ ] Panic recovery tested with both string panics and error panics
- [ ] `CHANGELOG.md` entry under `[Unreleased]`
- [ ] `golangci-lint run ./...` passes
- [ ] PR description explains any changes to existing behavior

## Potential Follow-Up Issues

Out of scope:

- Error middleware that transforms errors before response (e.g., sanitizing details)
- i18n support for error messages
- Custom JSON field names (e.g., snake_case vs camelCase configuration)
- gRPC-style error mapping
