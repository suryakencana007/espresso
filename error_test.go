package espresso

import (
	"errors"
	"testing"

	servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
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

func TestErrorResponse_WriteResponse(t *testing.T) {
	t.Run("basic error response", func(t *testing.T) {
		err := BadRequest("invalid parameter", map[string]string{"field": "name"})

		// Should implement IntoResponse
		var _ IntoResponse = err

		// Check fields
		if err.StatusCode != 400 {
			t.Errorf("expected status 400, got %d", err.StatusCode)
		}
		if err.ErrorType != "Bad Request" {
			t.Errorf("expected error type 'Bad Request', got %q", err.ErrorType)
		}
		if err.Message != "invalid parameter" {
			t.Errorf("expected message 'invalid parameter', got %q", err.Message)
		}
	})
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name        string
		constructor func(string, ...any) *ErrorResponse
		message     string
		statusCode  int
		errorType   string
	}{
		{"BadRequest", BadRequest, "bad request", 400, "Bad Request"},
		{"Unauthorized", Unauthorized, "unauthorized", 401, "Unauthorized"},
		{"Forbidden", Forbidden, "forbidden", 403, "Forbidden"},
		{"NotFound", NotFound, "not found", 404, "Not Found"},
		{"Conflict", Conflict, "conflict", 409, "Conflict"},
		{"InternalError", InternalError, "internal error", 500, "Internal Server Error"},
		{"ServiceUnavailable", ServiceUnavailable, "service unavailable", 503, "Service Unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor(tt.message)
			if err.StatusCode != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, err.StatusCode)
			}
			if err.ErrorType != tt.errorType {
				t.Errorf("expected error type %q, got %q", tt.errorType, err.ErrorType)
			}
			if err.Message != tt.message {
				t.Errorf("expected message %q, got %q", tt.message, err.Message)
			}
		})
	}
}

func TestErrorResponse_WithRequestID(t *testing.T) {
	err := BadRequest("test error")
	err = err.WithRequestID("req-123")

	if err.RequestID != "req-123" {
		t.Errorf("expected request ID 'req-123', got %q", err.RequestID)
	}
}

func TestValidationErrors(t *testing.T) {
	validationErrs := []ValidationError{
		{Field: "name", Message: "required"},
		{Field: "email", Message: "invalid format"},
	}

	err := ValidationErrors(validationErrs)

	if err.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", err.StatusCode)
	}
	if err.ErrorType != "Validation Error" {
		t.Errorf("expected error type 'Validation Error', got %q", err.ErrorType)
	}
	if err.Details == nil {
		t.Error("expected details to be set")
	}
}

func TestErrorResponse_ImplementsError(t *testing.T) {
	err := BadRequest("test error")

	// Should implement error interface
	var _ error = err

	if err.Error() != "test error" {
		t.Errorf("expected error message 'test error', got %q", err.Error())
	}
}

func TestErrorResponse_WithDetails(t *testing.T) {
	details := map[string]any{
		"field": "name",
		"code":  "REQUIRED",
	}

	err := BadRequest("validation failed", details)

	if err.Details == nil {
		t.Error("expected details to be set")
	}

	detailsMap, ok := err.Details.(map[string]any)
	if !ok {
		t.Error("expected details to be map[string]any")
		return
	}

	if detailsMap["field"] != "name" {
		t.Error("expected field detail to be 'name'")
	}
}

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
