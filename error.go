package espresso

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

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
// Structured Error Responses
// ============================================

// ErrorResponse represents a structured error response sent to clients.
// It implements the IntoResponse interface for automatic serialization.
type ErrorResponse struct {
	StatusCode int    `json:"status_code"`
	ErrorType  string `json:"error"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id,omitempty"`
	Details    any    `json:"details,omitempty"`
}

// WriteResponse implements IntoResponse by writing JSON error response.
func (e *ErrorResponse) WriteResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	return json.NewEncoder(w).Encode(e)
}

// Error implements the error interface.
func (e *ErrorResponse) Error() string {
	return e.Message
}

// BadRequest creates a 400 Bad Request error.
func BadRequest(message string, details ...any) *ErrorResponse {
	err := &ErrorResponse{
		StatusCode: http.StatusBadRequest,
		ErrorType:  "Bad Request",
		Message:    message,
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// Unauthorized creates a 401 Unauthorized error.
func Unauthorized(message string, details ...any) *ErrorResponse {
	err := &ErrorResponse{
		StatusCode: http.StatusUnauthorized,
		ErrorType:  "Unauthorized",
		Message:    message,
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// Forbidden creates a 403 Forbidden error.
func Forbidden(message string, details ...any) *ErrorResponse {
	err := &ErrorResponse{
		StatusCode: http.StatusForbidden,
		ErrorType:  "Forbidden",
		Message:    message,
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// NotFound creates a 404 Not Found error.
func NotFound(message string, details ...any) *ErrorResponse {
	err := &ErrorResponse{
		StatusCode: http.StatusNotFound,
		ErrorType:  "Not Found",
		Message:    message,
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// Conflict creates a 409 Conflict error.
func Conflict(message string, details ...any) *ErrorResponse {
	err := &ErrorResponse{
		StatusCode: http.StatusConflict,
		ErrorType:  "Conflict",
		Message:    message,
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// InternalError creates a 500 Internal Server Error.
func InternalError(message string, details ...any) *ErrorResponse {
	err := &ErrorResponse{
		StatusCode: http.StatusInternalServerError,
		ErrorType:  "Internal Server Error",
		Message:    message,
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// ServiceUnavailable creates a 503 Service Unavailable error.
func ServiceUnavailable(message string, details ...any) *ErrorResponse {
	err := &ErrorResponse{
		StatusCode: http.StatusServiceUnavailable,
		ErrorType:  "Service Unavailable",
		Message:    message,
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// ValidationError represents a validation error with field-specific details.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors creates a 400 Bad Request error with validation details.
func ValidationErrors(errors []ValidationError) *ErrorResponse {
	return &ErrorResponse{
		StatusCode: http.StatusBadRequest,
		ErrorType:  "Validation Error",
		Message:    "One or more fields failed validation",
		Details:    errors,
	}
}

// ============================================
// Enhanced Validation Errors
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

// WithRequestID adds a request ID to the error response.
func (e *ErrorResponse) WithRequestID(requestID string) *ErrorResponse {
	e.RequestID = requestID
	return e
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
