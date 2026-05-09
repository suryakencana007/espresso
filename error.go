package espresso

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
	servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
)

// ============================================
// CircuitBreaker Error Types
// ============================================

// CircuitBreakerError is returned when the circuit breaker is open.
// This custom error type allows users to distinguish between circuit breaker
// errors and other timeout errors.
type CircuitBreakerError struct {
	ServiceName string
	State       servicemiddleware.CircuitState
	Message     string
}

// Error implements the error interface.
func (e *CircuitBreakerError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("circuit breaker open for service %s: %s", e.ServiceName, e.Message)
	}
	return fmt.Sprintf("circuit breaker open for service %s", e.ServiceName)
}

// Unwrap returns nil to indicate this is a leaf error.
func (e *CircuitBreakerError) Unwrap() error {
	return nil
}

// Is allows errors.Is to match CircuitBreakerError.
func (e *CircuitBreakerError) Is(target error) bool {
	_, ok := target.(*CircuitBreakerError)
	return ok
}

// NewCircuitBreakerError creates a new CircuitBreakerError.
func NewCircuitBreakerError(serviceName string, state servicemiddleware.CircuitState, message string) *CircuitBreakerError {
	return &CircuitBreakerError{
		ServiceName: serviceName,
		State:       state,
		Message:     message,
	}
}

// IsCircuitBreakerError checks if an error is a CircuitBreakerError.
func IsCircuitBreakerError(err error) bool {
	var cbErr *CircuitBreakerError
	return errors.As(err, &cbErr)
}

// errorsAs is a helper that mirrors errors.As behavior.
func errorsAs(err error, target any) bool {
	return errors.As(err, target)
}

// ============================================
// Structured Error Type
// ============================================

// Error is Espresso's structured error type for HTTP responses.
// Handlers can return *Error to produce a consistent JSON error response
// with proper HTTP status code, machine-readable code, and optional details.
//
// Use the builder methods (WithCode, WithDetail, Wrap) to populate fields.
//
// Example:
//
//	return espresso.NewError(http.StatusNotFound, "project not found").
//	    WithCode("PROJECT_NOT_FOUND").
//	    WithDetail("projectId", id)
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

	// requestID stores a manually set request ID for backward compatibility.
	// Prefer relying on automatic request ID injection via RequestIDMiddleware.
	requestID string
}

// ErrorResponse is a type alias for Error, provided for backward compatibility.
// New code should prefer using Error directly.
type ErrorResponse = Error

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

// WithRequestID sets the request ID on the error for backward compatibility.
// Prefer relying on automatic request ID injection via RequestIDMiddleware.
func (e *Error) WithRequestID(requestID string) *Error {
	e.requestID = requestID
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

// WriteResponse implements IntoResponse by writing a structured JSON error response.
// The response format is: {"error": {"code": "...", "message": "...", ...}}.
func (e *Error) WriteResponse(w http.ResponseWriter) error {
	writeErrorResponse(w, nil, e)
	return nil
}

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
// If r is non-nil, the request ID is automatically included from context.
func writeErrorResponse(w http.ResponseWriter, r *http.Request, err *Error) {
	body := errorResponse{
		Error: errorBody{
			Code:      err.Code,
			Message:   err.Message,
			Details:   err.Details,
			RequestID: requestIDFromContext(r, err),
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(err.StatusCode)
	_ = json.NewEncoder(w).Encode(body)
}

// requestIDFromContext extracts the request ID from the request context,
// falling back to the error's manually set request ID.
func requestIDFromContext(r *http.Request, err *Error) string {
	if r != nil {
		if id := httpmiddleware.GetRequestID(r.Context()); id != "" {
			return id
		}
	}
	return err.requestID
}

// writeHandlerError writes an appropriate error response for a handler error.
// If the error is a *Error, it's used directly. Otherwise, it's wrapped as a
// 500 Internal Server Error.
func writeHandlerError(w http.ResponseWriter, r *http.Request, err error) {
	var espErr *Error
	if errors.As(err, &espErr) {
		writeErrorResponse(w, r, espErr)
		return
	}
	wrapped := ErrInternal("internal server error").Wrap(err)
	writeErrorResponse(w, r, wrapped)
}

// writeExtractError writes an appropriate error response for a request extraction error.
// If the error is a *Error, it's used directly. Otherwise, it's wrapped as a
// 400 Bad Request.
func writeExtractError(w http.ResponseWriter, r *http.Request, err error) {
	var espErr *Error
	if errors.As(err, &espErr) {
		writeErrorResponse(w, r, espErr)
		return
	}
	writeErrorResponse(w, r, ErrBadRequest(err.Error()))
}

// ============================================
// Common Error Constructors
// ============================================

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

// ErrPreconditionFailed creates a 412 Precondition Failed error.
// Use when a request precondition is not met — for example, missing
// prerequisite infrastructure (a Kubernetes CRD not installed), an
// If-Match header that doesn't match, or a required feature flag
// that is disabled.
func ErrPreconditionFailed(message string) *Error {
	return NewError(http.StatusPreconditionFailed, message).WithCode("PRECONDITION_FAILED")
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

// ============================================
// Backward-Compatible Constructors
// ============================================

// BadRequest creates a 400 Bad Request error.
// Provided for backward compatibility; prefer ErrBadRequest for new code.
func BadRequest(message string, details ...any) *Error {
	err := ErrBadRequest(message)
	if len(details) > 0 {
		if m, ok := details[0].(map[string]any); ok {
			err.Details = m
		}
	}
	return err
}

// Unauthorized creates a 401 Unauthorized error.
// Provided for backward compatibility; prefer ErrUnauthorized for new code.
func Unauthorized(message string, details ...any) *Error {
	err := ErrUnauthorized(message)
	if len(details) > 0 {
		if m, ok := details[0].(map[string]any); ok {
			err.Details = m
		}
	}
	return err
}

// Forbidden creates a 403 Forbidden error.
// Provided for backward compatibility; prefer ErrForbidden for new code.
func Forbidden(message string, details ...any) *Error {
	err := ErrForbidden(message)
	if len(details) > 0 {
		if m, ok := details[0].(map[string]any); ok {
			err.Details = m
		}
	}
	return err
}

// NotFound creates a 404 Not Found error.
// Provided for backward compatibility; prefer ErrNotFound for new code.
func NotFound(message string, details ...any) *Error {
	err := ErrNotFound(message)
	if len(details) > 0 {
		if m, ok := details[0].(map[string]any); ok {
			err.Details = m
		}
	}
	return err
}

// Conflict creates a 409 Conflict error.
// Provided for backward compatibility; prefer ErrConflict for new code.
func Conflict(message string, details ...any) *Error {
	err := ErrConflict(message)
	if len(details) > 0 {
		if m, ok := details[0].(map[string]any); ok {
			err.Details = m
		}
	}
	return err
}

// InternalError creates a 500 Internal Server Error.
// Provided for backward compatibility; prefer ErrInternal for new code.
func InternalError(message string, details ...any) *Error {
	err := ErrInternal(message)
	if len(details) > 0 {
		if m, ok := details[0].(map[string]any); ok {
			err.Details = m
		}
	}
	return err
}

// ServiceUnavailable creates a 503 Service Unavailable error.
// Provided for backward compatibility; prefer ErrServiceUnavailable for new code.
func ServiceUnavailable(message string, details ...any) *Error {
	err := ErrServiceUnavailable(message)
	if len(details) > 0 {
		if m, ok := details[0].(map[string]any); ok {
			err.Details = m
		}
	}
	return err
}

// ============================================
// Validation Errors
// ============================================

// ValidationError represents a single field validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors creates a 400 Bad Request error with validation details.
func ValidationErrors(errs []ValidationError) *Error {
	return ErrBadRequest("One or more fields failed validation").
		WithCode("VALIDATION_ERROR").
		WithDetail("errors", errs)
}

// ============================================
// Field Errors
// ============================================

// FieldError represents a single field validation error with path support.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
	Path    string `json:"path,omitempty"`
}

// Error implements the error interface for FieldError.
func (fe FieldError) Error() string {
	if fe.Path != "" {
		return fe.Path + "." + fe.Field + ": " + fe.Message
	}
	return fe.Field + ": " + fe.Message
}

// FieldErrors is a collection of field validation errors.
type FieldErrors []FieldError

// Error implements the error interface.
func (fe FieldErrors) Error() string {
	if len(fe) == 0 {
		return "validation errors"
	}
	if len(fe) == 1 {
		return fe[0].Message
	}
	return fmt.Sprintf("%d validation errors", len(fe))
}

// ToValidationErrors converts FieldErrors to ValidationError slice.
func (fe FieldErrors) ToValidationErrors() []ValidationError {
	validationErrors := make([]ValidationError, len(fe))
	for i, e := range fe {
		validationErrors[i] = ValidationError{
			Field:   e.Field,
			Message: e.Message,
		}
	}
	return validationErrors
}

// AddFieldError adds a field error to the collection.
func (fe *FieldErrors) AddFieldError(field, message string, value any, path ...string) *FieldError {
	var p string
	if len(path) > 0 {
		p = path[0]
	}
	fieldErr := FieldError{
		Field:   field,
		Message: message,
		Value:   value,
		Path:    p,
	}
	*fe = append(*fe, fieldErr)
	return &fieldErr
}

// NewFieldErrors creates a new FieldErrors collection.
func NewFieldErrors() *FieldErrors {
	return &FieldErrors{}
}

// RequiredFieldError creates a field error for required field missing.
func RequiredFieldError(field string, path ...string) FieldError {
	var p string
	if len(path) > 0 {
		p = path[0]
	}
	return FieldError{
		Field:   field,
		Message: "required field is missing",
		Path:    p,
	}
}

// InvalidTypeError creates a field error for type mismatch.
func InvalidTypeError(field string, expected, actual string, value any, path ...string) FieldError {
	var p string
	if len(path) > 0 {
		p = path[0]
	}
	return FieldError{
		Field:   field,
		Message: fmt.Sprintf("expected %s, got %s", expected, actual),
		Value:   value,
		Path:    p,
	}
}

// RangeError creates a field error for value out of range.
func RangeError(field string, min, max any, value any, path ...string) FieldError {
	var p string
	if len(path) > 0 {
		p = path[0]
	}
	return FieldError{
		Field:   field,
		Message: fmt.Sprintf("value must be between %v and %v", min, max),
		Value:   value,
		Path:    p,
	}
}

// LengthError creates a field error for length constraints.
func LengthError(field string, minLen, maxLen int, value any, path ...string) FieldError {
	var p string
	if len(path) > 0 {
		p = path[0]
	}
	msg := fmt.Sprintf("length must be between %d and %d characters", minLen, maxLen)
	if minLen == maxLen {
		msg = fmt.Sprintf("length must be exactly %d characters", minLen)
	}
	return FieldError{
		Field:   field,
		Message: msg,
		Value:   value,
		Path:    p,
	}
}

// PatternError creates a field error for pattern mismatch.
func PatternError(field, pattern string, value any, path ...string) FieldError {
	var p string
	if len(path) > 0 {
		p = path[0]
	}
	return FieldError{
		Field:   field,
		Message: fmt.Sprintf("must match pattern: %s", pattern),
		Value:   value,
		Path:    p,
	}
}

// CustomValidationError creates a custom validation error.
func CustomValidationError(field, message string, value any, path ...string) FieldError {
	var p string
	if len(path) > 0 {
		p = path[0]
	}
	return FieldError{
		Field:   field,
		Message: message,
		Value:   value,
		Path:    p,
	}
}

// ============================================
// Error Handler Configuration
// ============================================

// ErrorHandlerConfig configures how errors are handled and presented.
type ErrorHandlerConfig struct {
	// IncludeStackTrace includes stack traces in error responses (development mode)
	IncludeStackTrace bool
	// IncludeDetails includes error details in responses
	IncludeDetails bool
	// DefaultMessage is used when error message should not be exposed
	DefaultMessage string
	// OnError is called when an error occurs (for logging/metrics)
	OnError func(err error, statusCode int)
}

// DefaultErrorHandlerConfig provides sensible defaults.
var DefaultErrorHandlerConfig = ErrorHandlerConfig{
	IncludeStackTrace: false,
	IncludeDetails:    true,
	DefaultMessage:    "An error occurred",
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
