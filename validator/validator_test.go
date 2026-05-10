package validator_test

import (
	"errors"
	"testing"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/validator"
)

func TestStruct_NilAndNonStruct(t *testing.T) {
	if err := validator.Struct(nil); err != nil {
		t.Errorf("nil input should pass, got %v", err)
	}

	var ptr *struct{ Name string }
	if err := validator.Struct(ptr); err != nil {
		t.Errorf("nil pointer should pass, got %v", err)
	}

	if err := validator.Struct(42); err == nil {
		t.Error("non-struct input should return an error")
	}
}

func TestStruct_Required(t *testing.T) {
	type req struct {
		Name string `json:"name" validate:"required"`
		Tags []int  `json:"tags" validate:"required"`
	}

	err := validator.Struct(req{})
	if err == nil {
		t.Fatal("expected validation errors")
	}

	var fe espresso.FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("expected FieldErrors, got %T", err)
	}
	if len(fe) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(fe), fe)
	}

	fields := map[string]bool{}
	for _, e := range fe {
		fields[e.Field] = true
	}
	if !fields["name"] || !fields["tags"] {
		t.Errorf("missing expected field errors: %v", fe)
	}
}

func TestStruct_RequiredPass(t *testing.T) {
	type req struct {
		Name string `json:"name" validate:"required"`
	}
	if err := validator.Struct(req{Name: "nanang"}); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestStruct_MinMaxString(t *testing.T) {
	type req struct {
		Name string `json:"name" validate:"min=3,max=10"`
	}

	if err := validator.Struct(req{Name: "hi"}); err == nil {
		t.Error("expected min violation")
	}
	if err := validator.Struct(req{Name: "hello world!!"}); err == nil {
		t.Error("expected max violation")
	}
	if err := validator.Struct(req{Name: "hello"}); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestStruct_MinMaxNumeric(t *testing.T) {
	type req struct {
		Age    int     `json:"age"    validate:"min=0,max=150"`
		Weight float64 `json:"weight" validate:"min=0"`
	}

	if err := validator.Struct(req{Age: -1, Weight: 70}); err == nil {
		t.Error("expected min violation on age")
	}
	if err := validator.Struct(req{Age: 200, Weight: 70}); err == nil {
		t.Error("expected max violation on age")
	}
	if err := validator.Struct(req{Age: 30, Weight: -5}); err == nil {
		t.Error("expected min violation on weight")
	}
	if err := validator.Struct(req{Age: 30, Weight: 70}); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestStruct_MinMaxSlice(t *testing.T) {
	type req struct {
		Tags []string `json:"tags" validate:"min=1,max=3"`
	}
	if err := validator.Struct(req{Tags: nil}); err == nil {
		t.Error("empty slice should fail min=1")
	}
	if err := validator.Struct(req{Tags: []string{"a", "b", "c", "d"}}); err == nil {
		t.Error("over-long slice should fail max=3")
	}
	if err := validator.Struct(req{Tags: []string{"a", "b"}}); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestStruct_Email(t *testing.T) {
	type req struct {
		Email string `json:"email" validate:"email"`
	}
	if err := validator.Struct(req{Email: "not-an-email"}); err == nil {
		t.Error("expected email validation failure")
	}
	if err := validator.Struct(req{Email: "user@example.com"}); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
	// Empty string passes because only `required` enforces presence.
	if err := validator.Struct(req{Email: ""}); err != nil {
		t.Errorf("empty email should pass without required, got %v", err)
	}
}

func TestStruct_URL(t *testing.T) {
	type req struct {
		Site string `json:"site" validate:"url"`
	}
	if err := validator.Struct(req{Site: "not a url"}); err == nil {
		t.Error("expected url validation failure")
	}
	if err := validator.Struct(req{Site: "https://example.com/path"}); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestStruct_Regex(t *testing.T) {
	type req struct {
		Code string `json:"code" validate:"regex=^[A-Z]{3}-\\d{3}$"`
	}
	if err := validator.Struct(req{Code: "abc-123"}); err == nil {
		t.Error("expected regex failure")
	}
	if err := validator.Struct(req{Code: "ABC-123"}); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestStruct_OneOf(t *testing.T) {
	type req struct {
		Role string `json:"role" validate:"oneof=admin user guest"`
	}
	if err := validator.Struct(req{Role: "owner"}); err == nil {
		t.Error("expected oneof failure")
	}
	if err := validator.Struct(req{Role: "admin"}); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestStruct_Nested(t *testing.T) {
	type addr struct {
		City string `json:"city" validate:"required"`
	}
	type req struct {
		Name string `json:"name" validate:"required"`
		Addr addr   `json:"addr"`
	}

	err := validator.Struct(req{Name: "nanang"})
	if err == nil {
		t.Fatal("expected nested field error")
	}
	var fe espresso.FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("expected FieldErrors, got %T", err)
	}
	if len(fe) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(fe), fe)
	}
	if fe[0].Field != "city" || fe[0].Path != "addr" {
		t.Errorf("expected field=city path=addr, got field=%q path=%q", fe[0].Field, fe[0].Path)
	}
}

func TestStruct_NestedPointer(t *testing.T) {
	type inner struct {
		Val string `json:"val" validate:"required"`
	}
	type req struct {
		Inner *inner `json:"inner"`
	}
	// Nil inner pointer: no recursion, no error.
	if err := validator.Struct(req{}); err != nil {
		t.Errorf("nil nested pointer should pass, got %v", err)
	}
	// Present but empty value: should report the nested error.
	err := validator.Struct(req{Inner: &inner{}})
	if err == nil {
		t.Fatal("expected nested error")
	}
	var fe espresso.FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("expected FieldErrors, got %T", err)
	}
	if len(fe) != 1 || fe[0].Path != "inner" {
		t.Errorf("expected 1 error with path=inner, got %v", fe)
	}
}

func TestStruct_NestedSliceOfStructs(t *testing.T) {
	type item struct {
		Name string `json:"name" validate:"required"`
	}
	type req struct {
		Items []item `json:"items"`
	}
	err := validator.Struct(req{Items: []item{{Name: "ok"}, {Name: ""}}})
	if err == nil {
		t.Fatal("expected error on items[1]")
	}
	var fe espresso.FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("expected FieldErrors, got %T", err)
	}
	if len(fe) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(fe), fe)
	}
	if fe[0].Path != "items[1]" {
		t.Errorf("expected path=items[1], got %q", fe[0].Path)
	}
}

func TestStruct_MultipleRulesSingleField(t *testing.T) {
	type req struct {
		Name string `json:"name" validate:"required,min=3"`
	}
	err := validator.Struct(req{Name: ""})
	if err == nil {
		t.Fatal("expected error")
	}
	var fe espresso.FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("expected FieldErrors, got %T", err)
	}
	// Both required and min should fire.
	if len(fe) != 2 {
		t.Errorf("expected 2 errors for empty string against required+min, got %d: %v", len(fe), fe)
	}
}

func TestStruct_JSONFieldName(t *testing.T) {
	type req struct {
		DisplayName string `json:"display_name" validate:"required"`
	}
	err := validator.Struct(req{})
	var fe espresso.FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("expected FieldErrors, got %T", err)
	}
	if len(fe) != 1 || fe[0].Field != "display_name" {
		t.Errorf("expected field=display_name, got %v", fe)
	}
}

func TestStruct_ToValidationErrors(t *testing.T) {
	type req struct {
		Name string `json:"name" validate:"required"`
	}
	err := validator.Struct(req{})
	var fe espresso.FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("expected FieldErrors, got %T", err)
	}
	ves := fe.ToValidationErrors()
	if len(ves) != 1 {
		t.Fatalf("expected 1 validation error, got %d", len(ves))
	}
	structured := espresso.ValidationErrors(ves)
	if structured.Code != "VALIDATION_ERROR" {
		t.Errorf("expected code VALIDATION_ERROR, got %q", structured.Code)
	}
	if structured.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", structured.StatusCode)
	}
}
