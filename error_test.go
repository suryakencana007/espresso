package espresso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpmiddleware "github.com/suryakencana007/espresso/v2/middleware/http"
	servicemiddleware "github.com/suryakencana007/espresso/v2/middleware/service"
)

func TestCircuitBreakerError_Error(t *testing.T) {
	t.Run("error with message", func(t *testing.T) {
		err := NewCircuitBreakerError("my-service", servicemiddleware.StateOpen, "too many failures")
		expected := "circuit breaker open for service my-service: too many failures"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("error without message", func(t *testing.T) {
		err := NewCircuitBreakerError("my-service", servicemiddleware.StateOpen, "")
		expected := "circuit breaker open for service my-service"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})
}

func TestCircuitBreakerError_Is(t *testing.T) {
	t.Run("matches CircuitBreakerError", func(t *testing.T) {
		err := NewCircuitBreakerError("service", servicemiddleware.StateOpen, "test")
		var cbErr *CircuitBreakerError
		if !errorsAs(err, &cbErr) {
			t.Error("expected error to be CircuitBreakerError")
		}
	})

	t.Run("errors.Is works", func(t *testing.T) {
		err := NewCircuitBreakerError("service", servicemiddleware.StateOpen, "test")
		if !errors.Is(err, &CircuitBreakerError{}) {
			t.Error("expected errors.Is to match CircuitBreakerError")
		}
	})
}

func TestIsCircuitBreakerError(t *testing.T) {
	t.Run("circuit breaker error", func(t *testing.T) {
		err := NewCircuitBreakerError("service", servicemiddleware.StateOpen, "test")
		if !IsCircuitBreakerError(err) {
			t.Error("expected IsCircuitBreakerError to return true")
		}
	})

	t.Run("other error", func(t *testing.T) {
		err := errors.New("some error")
		if IsCircuitBreakerError(err) {
			t.Error("expected IsCircuitBreakerError to return false")
		}
	})
}

// ============================================
// New Error type tests
// ============================================

func TestError_Builder(t *testing.T) {
	err := NewError(http.StatusNotFound, "project not found").
		WithCode("PROJECT_NOT_FOUND").
		WithDetail("projectId", "abc123").
		Wrap(errors.New("db error"))

	if err.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", err.StatusCode)
	}
	if err.Code != "PROJECT_NOT_FOUND" {
		t.Errorf("expected code PROJECT_NOT_FOUND, got %q", err.Code)
	}
	if err.Message != "project not found" {
		t.Errorf("expected message 'project not found', got %q", err.Message)
	}
	if err.Details["projectId"] != "abc123" {
		t.Errorf("expected detail projectId=abc123, got %v", err.Details["projectId"])
	}
	if err.Internal == nil {
		t.Error("expected internal error to be set")
	}
	if err.Internal.Error() != "db error" {
		t.Errorf("expected internal error 'db error', got %q", err.Internal.Error())
	}
}

func TestError_Serialization(t *testing.T) {
	err := NewError(http.StatusNotFound, "project not found").
		WithCode("PROJECT_NOT_FOUND").
		WithDetail("projectId", "abc123")

	w := httptest.NewRecorder()
	writeErrorResponse(w, nil, err)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json content type, got %q", ct)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "PROJECT_NOT_FOUND" {
		t.Errorf("expected code PROJECT_NOT_FOUND, got %q", resp.Error.Code)
	}
	if resp.Error.Message != "project not found" {
		t.Errorf("expected message 'project not found', got %q", resp.Error.Message)
	}
	if resp.Error.Details["projectId"] != "abc123" {
		t.Errorf("expected detail projectId=abc123, got %v", resp.Error.Details["projectId"])
	}
}

func TestError_RequestIDIncluded(t *testing.T) {
	err := ErrBadRequest("test error")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	r = r.WithContext(setRequestID(r.Context(), "req-123"))

	writeErrorResponse(w, r, err)

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.RequestID != "req-123" {
		t.Errorf("expected request_id 'req-123', got %q", resp.Error.RequestID)
	}
}

func TestError_RequestIDOmitted(t *testing.T) {
	err := ErrBadRequest("test error")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	writeErrorResponse(w, r, err)

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.RequestID != "" {
		t.Errorf("expected empty request_id, got %q", resp.Error.RequestID)
	}
}

func TestError_InternalNotExposed(t *testing.T) {
	err := ErrInternal("something failed").Wrap(errors.New("database connection refused"))

	w := httptest.NewRecorder()
	writeErrorResponse(w, nil, err)

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	body := w.Body.String()
	if strings.Contains(body, "database connection refused") {
		t.Error("internal error should not be exposed in JSON response")
	}
	if strings.Contains(body, "internal") {
		// Check it's the "code" field, not the internal error message
		if strings.Contains(body, `"internal"`) {
			t.Error("internal field should not appear in JSON response")
		}
	}
}

func TestError_WrapInternal(t *testing.T) {
	innerErr := errors.New("connection refused")
	err := ErrInternal("something failed").Wrap(innerErr)

	if err.Internal != innerErr {
		t.Error("expected internal error to be wrapped")
	}
	if err.Unwrap() != innerErr {
		t.Error("expected Unwrap to return inner error")
	}
}

func TestError_ErrorsIs(t *testing.T) {
	innerErr := errors.New("connection refused")
	err := ErrNotFound("not found").Wrap(innerErr)

	if !errors.Is(err, innerErr) {
		t.Error("expected errors.Is to match wrapped inner error")
	}
}

func TestError_ErrorsAs(t *testing.T) {
	err := ErrNotFound("not found")

	var espErr *Error
	if !errors.As(err, &espErr) {
		t.Error("expected errors.As to match *Error")
	}
	if espErr.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %q", espErr.Code)
	}

	// Test with wrapped *Error
	wrapped := fmt.Errorf("failed: %w", err)
	if !errors.As(wrapped, &espErr) {
		t.Error("expected errors.As to match *Error through fmt.Errorf wrapping")
	}
}

func TestError_HandlerReturnsError(t *testing.T) {
	err := ErrNotFound("resource not found").WithDetail("id", "123")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	writeHandlerError(w, r, err)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %q", resp.Error.Code)
	}
}

func TestError_HandlerReturnsPlainError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	writeHandlerError(w, r, errors.New("something broke"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "INTERNAL" {
		t.Errorf("expected code INTERNAL, got %q", resp.Error.Code)
	}
	if resp.Error.Message != "internal server error" {
		t.Errorf("expected message 'internal server error', got %q", resp.Error.Message)
	}
}

func TestError_HandlerReturnsWrappedError(t *testing.T) {
	inner := ErrNotFound("project not found")
	wrapped := fmt.Errorf("failed: %w", inner)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	writeHandlerError(w, r, wrapped)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %q", resp.Error.Code)
	}
}

func TestError_Constructors(t *testing.T) {
	tests := []struct {
		name       string
		err        *Error
		statusCode int
		code       string
		message    string
	}{
		{"ErrBadRequest", ErrBadRequest("bad"), 400, "BAD_REQUEST", "bad"},
		{"ErrUnauthorized", ErrUnauthorized("unauth"), 401, "UNAUTHORIZED", "unauth"},
		{"ErrForbidden", ErrForbidden("forbidden"), 403, "FORBIDDEN", "forbidden"},
		{"ErrNotFound", ErrNotFound("not found"), 404, "NOT_FOUND", "not found"},
		{"ErrConflict", ErrConflict("conflict"), 409, "CONFLICT", "conflict"},
		{"ErrPreconditionFailed", ErrPreconditionFailed("precondition"), 412, "PRECONDITION_FAILED", "precondition"},
		{"ErrUnprocessableEntity", ErrUnprocessableEntity("unprocessable"), 422, "UNPROCESSABLE_ENTITY", "unprocessable"},
		{"ErrTooManyRequests", ErrTooManyRequests("rate limited"), 429, "TOO_MANY_REQUESTS", "rate limited"},
		{"ErrInternal", ErrInternal("internal"), 500, "INTERNAL", "internal"},
		{"ErrServiceUnavailable", ErrServiceUnavailable("unavailable"), 503, "SERVICE_UNAVAILABLE", "unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.StatusCode != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, tt.err.StatusCode)
			}
			if tt.err.Code != tt.code {
				t.Errorf("expected code %q, got %q", tt.code, tt.err.Code)
			}
			if tt.err.Message != tt.message {
				t.Errorf("expected message %q, got %q", tt.message, tt.err.Message)
			}
		})
	}
}

func TestError_DefaultCodeForStatus(t *testing.T) {
	tests := []struct {
		status       int
		expectedCode string
	}{
		{400, "BAD_REQUEST"},
		{401, "UNAUTHORIZED"},
		{403, "FORBIDDEN"},
		{404, "NOT_FOUND"},
		{409, "CONFLICT"},
		{422, "UNPROCESSABLE_ENTITY"},
		{429, "TOO_MANY_REQUESTS"},
		{500, "INTERNAL"},
		{503, "SERVICE_UNAVAILABLE"},
		{418, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.status), func(t *testing.T) {
			err := NewError(tt.status, "test")
			if err.Code != tt.expectedCode {
				t.Errorf("expected code %q, got %q", tt.expectedCode, err.Code)
			}
		})
	}
}

func TestError_WithDetails(t *testing.T) {
	details := map[string]any{
		"field": "name",
		"code":  "REQUIRED",
	}

	err := ErrBadRequest("validation failed").WithDetails(details)

	if err.Details == nil {
		t.Error("expected details to be set")
	}
	if err.Details["field"] != "name" {
		t.Error("expected field detail to be 'name'")
	}
}

func TestError_WithRequestID(t *testing.T) {
	err := ErrBadRequest("test").WithRequestID("req-456")

	// requestID is unexported; verify via serialization
	w := httptest.NewRecorder()
	writeErrorResponse(w, nil, err)

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.RequestID != "req-456" {
		t.Errorf("expected request_id 'req-456', got %q", resp.Error.RequestID)
	}
}

func TestError_ErrorString(t *testing.T) {
	t.Run("without internal", func(t *testing.T) {
		err := ErrNotFound("resource not found")
		expected := "404 NOT_FOUND: resource not found"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("with internal", func(t *testing.T) {
		err := ErrInternal("database error").Wrap(errors.New("connection refused"))
		expected := "500 INTERNAL: database error (internal: connection refused)"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})
}

func TestValidationErrors_Constructor(t *testing.T) {
	validationErrs := []ValidationError{
		{Field: "name", Message: "required"},
		{Field: "email", Message: "invalid format"},
	}

	err := ValidationErrors(validationErrs)

	if err.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", err.StatusCode)
	}
	if err.Code != "VALIDATION_ERROR" {
		t.Errorf("expected code VALIDATION_ERROR, got %q", err.Code)
	}
	if err.Details == nil {
		t.Error("expected details to be set")
	}
}

func TestError_IntoResponseAssertion(t *testing.T) {
	var _ IntoResponse = ErrBadRequest("test error")
}

func TestError_WithRequestID_WriteResponse(t *testing.T) {
	err := ErrBadRequest("test").WithRequestID("req-123")
	w := httptest.NewRecorder()
	writeErrorResponse(w, nil, err)

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.RequestID != "req-123" {
		t.Errorf("expected request_id 'req-123', got %q", resp.Error.RequestID)
	}
}

// ============================================
// Field Error Tests (unchanged from original)
// ============================================

func TestFieldError_Error(t *testing.T) {
	t.Run("error with path", func(t *testing.T) {
		fieldErr := FieldError{
			Field:   "name",
			Message: "required field is missing",
			Path:    "query",
		}
		expected := "query.name: required field is missing"
		if fieldErr.Error() != expected {
			t.Errorf("expected %q, got %q", expected, fieldErr.Error())
		}
	})

	t.Run("error without path", func(t *testing.T) {
		fieldErr := FieldError{
			Field:   "email",
			Message: "invalid format",
		}
		expected := "email: invalid format"
		if fieldErr.Error() != expected {
			t.Errorf("expected %q, got %q", expected, fieldErr.Error())
		}
	})

	t.Run("error with value", func(t *testing.T) {
		fieldErr := FieldError{
			Field:   "age",
			Message: "must be positive",
			Value:   -1,
		}
		if fieldErr.Value != -1 {
			t.Errorf("expected value -1, got %v", fieldErr.Value)
		}
	})
}

func TestFieldErrors_Error(t *testing.T) {
	t.Run("empty errors", func(t *testing.T) {
		var errs FieldErrors
		if errs.Error() != "validation errors" {
			t.Errorf("expected 'validation errors', got %q", errs.Error())
		}
	})

	t.Run("single error", func(t *testing.T) {
		errs := FieldErrors{
			{Field: "name", Message: "required"},
		}
		if errs.Error() != "required" {
			t.Errorf("expected 'required', got %q", errs.Error())
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		errs := FieldErrors{
			{Field: "name", Message: "required"},
			{Field: "email", Message: "invalid"},
		}
		if errs.Error() != "2 validation errors" {
			t.Errorf("expected '2 validation errors', got %q", errs.Error())
		}
	})
}

func TestFieldErrors_ToValidationErrors(t *testing.T) {
	errs := FieldErrors{
		{Field: "name", Message: "required"},
		{Field: "email", Message: "invalid format"},
	}

	validationErrs := errs.ToValidationErrors()

	if len(validationErrs) != 2 {
		t.Errorf("expected 2 validation errors, got %d", len(validationErrs))
	}
	if validationErrs[0].Field != "name" {
		t.Errorf("expected field 'name', got %q", validationErrs[0].Field)
	}
	if validationErrs[1].Field != "email" {
		t.Errorf("expected field 'email', got %q", validationErrs[1].Field)
	}
}

func TestFieldErrors_AddFieldError(t *testing.T) {
	errs := NewFieldErrors()

	fieldErr := errs.AddFieldError("name", "required field", nil, "query")

	if len(*errs) != 1 {
		t.Error("expected 1 error in collection")
	}
	if fieldErr.Field != "name" {
		t.Errorf("expected field 'name', got %q", fieldErr.Field)
	}
	if fieldErr.Path != "query" {
		t.Errorf("expected path 'query', got %q", fieldErr.Path)
	}
}

func TestRequiredFieldError_Function(t *testing.T) {
	fieldErr := RequiredFieldError("name", "query")

	if fieldErr.Field != "name" {
		t.Errorf("expected field 'name', got %q", fieldErr.Field)
	}
	if fieldErr.Message != "required field is missing" {
		t.Errorf("expected 'required field is missing', got %q", fieldErr.Message)
	}
	if fieldErr.Path != "query" {
		t.Errorf("expected path 'query', got %q", fieldErr.Path)
	}
}

func TestInvalidTypeError(t *testing.T) {
	fieldErr := InvalidTypeError("age", "int", "string", "abc")

	if fieldErr.Field != "age" {
		t.Errorf("expected field 'age', got %q", fieldErr.Field)
	}
	if fieldErr.Message != "expected int, got string" {
		t.Errorf("expected 'expected int, got string', got %q", fieldErr.Message)
	}
	if fieldErr.Value != "abc" {
		t.Errorf("expected value 'abc', got %v", fieldErr.Value)
	}
}

func TestRangeError(t *testing.T) {
	fieldErr := RangeError("rating", 1, 5, 6)

	if fieldErr.Field != "rating" {
		t.Errorf("expected field 'rating', got %q", fieldErr.Field)
	}
	if fieldErr.Message != "value must be between 1 and 5" {
		t.Errorf("expected 'value must be between 1 and 5', got %q", fieldErr.Message)
	}
	if fieldErr.Value != 6 {
		t.Errorf("expected value 6, got %v", fieldErr.Value)
	}
}

func TestLengthError(t *testing.T) {
	t.Run("range constraint", func(t *testing.T) {
		fieldErr := LengthError("username", 3, 20, "ab")

		if fieldErr.Field != "username" {
			t.Errorf("expected field 'username', got %q", fieldErr.Field)
		}
		if fieldErr.Message != "length must be between 3 and 20 characters" {
			t.Errorf("expected range message, got %q", fieldErr.Message)
		}
	})

	t.Run("exact length constraint", func(t *testing.T) {
		fieldErr := LengthError("code", 6, 6, "123")

		if fieldErr.Message != "length must be exactly 6 characters" {
			t.Errorf("expected exact message, got %q", fieldErr.Message)
		}
	})
}

func TestPatternError(t *testing.T) {
	fieldErr := PatternError("email", `^[a-z]+@[a-z]+\.[a-z]+$`, "invalid")

	if fieldErr.Field != "email" {
		t.Errorf("expected field 'email', got %q", fieldErr.Field)
	}
	expectedMsg := "must match pattern: ^[a-z]+@[a-z]+\\.[a-z]+$"
	if fieldErr.Message != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, fieldErr.Message)
	}
}

func TestCustomValidationError(t *testing.T) {
	fieldErr := CustomValidationError("password", "must contain uppercase", "pass", "body")

	if fieldErr.Field != "password" {
		t.Errorf("expected field 'password', got %q", fieldErr.Field)
	}
	if fieldErr.Message != "must contain uppercase" {
		t.Errorf("expected 'must contain uppercase', got %q", fieldErr.Message)
	}
	if fieldErr.Value != "pass" {
		t.Errorf("expected value 'pass', got %v", fieldErr.Value)
	}
	if fieldErr.Path != "body" {
		t.Errorf("expected path 'body', got %q", fieldErr.Path)
	}
}

// ============================================
// Helper for setting request ID in context
// ============================================

func setRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, httpmiddleware.RequestIDKey{}, id)
}

func TestError_WriteResponse(t *testing.T) {
	err := ErrNotFound("resource gone").WithDetail("id", "42")

	w := httptest.NewRecorder()
	if writeErr := err.WriteResponse(w); writeErr != nil {
		t.Fatalf("WriteResponse failed: %v", writeErr)
	}

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json, got %q", ct)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Error.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %q", resp.Error.Code)
	}
}

func TestError_PanicRecovered(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	// Simulate what RecoverMiddleware does
	defer func() {
		if rec := recover(); rec != nil {
			writeHandlerError(w, r, ErrInternal("internal server error").WithCode("PANIC"))
		}
	}()
	panic("test panic")
}

func TestError_PanicRecoveryInMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	middleware := httpmiddleware.RecoverMiddleware()
	server := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Error.Code != "PANIC" {
		t.Errorf("expected code PANIC, got %q", resp.Error.Code)
	}
}

func TestError_ExtractErrorViaHandler(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	// Test plain error extraction
	writeExtractError(w, r, errors.New("bad input"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %q", resp.Error.Code)
	}
}

func TestError_ExtractErrorStructured(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	// Test structured error extraction
	extractErr := ErrUnprocessableEntity("invalid JSON")
	writeExtractError(w, r, extractErr)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", w.Code)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Error.Code != "UNPROCESSABLE_ENTITY" {
		t.Errorf("expected UNPROCESSABLE_ENTITY, got %q", resp.Error.Code)
	}
}
