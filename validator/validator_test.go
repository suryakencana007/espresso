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

func TestAsDefaultValidator_InvalidReturns400(t *testing.T) {
	type Req struct {
		Name string `json:"name" validate:"required"`
	}
	fn := validator.AsDefaultValidator()
	err := fn(&Req{}) // empty name → required fails
	if err == nil {
		t.Fatal("expected error from empty Name")
	}
	var espErr *espresso.Error
	if !errors.As(err, &espErr) {
		t.Fatalf("expected *espresso.Error, got %T", err)
	}
	if espErr.StatusCode != 400 || espErr.Code != "VALIDATION_ERROR" {
		t.Errorf("status=%d code=%q, want 400/VALIDATION_ERROR", espErr.StatusCode, espErr.Code)
	}
}

func TestAsDefaultValidator_ValidReturnsNil(t *testing.T) {
	type Req struct {
		Name string `json:"name" validate:"required"`
	}
	fn := validator.AsDefaultValidator()
	if err := fn(&Req{Name: "alice"}); err != nil {
		t.Errorf("unexpected error for valid input: %v", err)
	}
}

// TestStruct_PointerFieldsDereference locks the v2.4 task-09 fix. Before the
// fix, walkStruct applied rules to the raw pointer value and only
// dereferenced for the recursion step afterwards. Every rule except
// `required` was unusable on pointer fields: *string with `email` yielded
// "email rule requires string field" and *int with `min=18` yielded
// "min/max not supported for kind ptr", even for valid values. This
// contradicted docs/guide/validation.md's "Nil pointer fields are skipped"
// contract and turned well-formed client requests into 400s.
//
// Post-fix: nil pointers skip non-`required` rules; non-nil pointers have
// their rules applied to the dereferenced element value; `required` still
// operates on the pointer itself so a nil pointer fails `required`.
func TestStruct_PointerString_Email(t *testing.T) {
	type req struct {
		Email *string `json:"email" validate:"email"`
	}

	// Valid email through pointer — pre-fix: "email rule requires string field".
	valid := "user@example.com"
	if err := validator.Struct(req{Email: &valid}); err != nil {
		t.Errorf("valid *string email should pass, got %v", err)
	}

	// Nil pointer — non-required rule skipped, no error.
	if err := validator.Struct(req{}); err != nil {
		t.Errorf("nil *string should skip email rule, got %v", err)
	}

	// Invalid email through pointer — rule sees dereferenced value, fails.
	invalid := "not-an-email"
	if err := validator.Struct(req{Email: &invalid}); err == nil {
		t.Error("expected email validation failure on *string with invalid value")
	}
}

func TestStruct_PointerInt_Min(t *testing.T) {
	type req struct {
		Age *int `json:"age" validate:"min=18"`
	}

	valid := 30
	if err := validator.Struct(req{Age: &valid}); err != nil {
		t.Errorf("valid *int min should pass, got %v", err)
	}

	if err := validator.Struct(req{}); err != nil {
		t.Errorf("nil *int should skip min rule, got %v", err)
	}

	tooYoung := 17
	if err := validator.Struct(req{Age: &tooYoung}); err == nil {
		t.Error("expected min violation on *int with value below bound")
	}
}

func TestStruct_PointerFloat_Max(t *testing.T) {
	type req struct {
		Weight *float64 `json:"weight" validate:"max=200"`
	}

	valid := 75.5
	if err := validator.Struct(req{Weight: &valid}); err != nil {
		t.Errorf("valid *float64 max should pass, got %v", err)
	}

	over := 250.0
	if err := validator.Struct(req{Weight: &over}); err == nil {
		t.Error("expected max violation on *float64 over bound")
	}
}

func TestStruct_PointerString_URL(t *testing.T) {
	type req struct {
		Site *string `json:"site" validate:"url"`
	}

	valid := "https://example.com/path"
	if err := validator.Struct(req{Site: &valid}); err != nil {
		t.Errorf("valid *string url should pass, got %v", err)
	}

	if err := validator.Struct(req{}); err != nil {
		t.Errorf("nil *string url should skip, got %v", err)
	}
}

func TestStruct_PointerString_OneOf(t *testing.T) {
	type req struct {
		Role *string `json:"role" validate:"oneof=admin user guest"`
	}

	valid := "admin"
	if err := validator.Struct(req{Role: &valid}); err != nil {
		t.Errorf("valid *string oneof should pass, got %v", err)
	}

	invalid := "owner"
	if err := validator.Struct(req{Role: &invalid}); err == nil {
		t.Error("expected oneof failure on *string with disallowed value")
	}
}

func TestStruct_PointerString_Regex(t *testing.T) {
	type req struct {
		Code *string `json:"code" validate:"regex=^[A-Z]{3}$"`
	}

	valid := "ABC"
	if err := validator.Struct(req{Code: &valid}); err != nil {
		t.Errorf("valid *string regex should pass, got %v", err)
	}

	invalid := "abc"
	if err := validator.Struct(req{Code: &invalid}); err == nil {
		t.Error("expected regex failure on *string with non-matching value")
	}
}

func TestStruct_PointerRequired(t *testing.T) {
	// `required` on a pointer field operates on the pointer itself: a nil
	// pointer fails, a non-nil pointer to any value (including zero) passes.
	type req struct {
		Name *string `json:"name" validate:"required"`
	}

	// Nil pointer — required fails.
	if err := validator.Struct(req{}); err == nil {
		t.Error("expected required to fail on nil pointer")
	}

	// Non-nil pointer to empty string — required passes (the pointer itself
	// is non-nil). Note: this differs from `required` on a bare string,
	// which fails on the zero value. It matches Go's typical "*T is
	// present if non-nil" semantics.
	empty := ""
	if err := validator.Struct(req{Name: &empty}); err != nil {
		t.Errorf("non-nil *string should satisfy required, got %v", err)
	}
}

func TestStruct_PointerRequiredWithChainedRule(t *testing.T) {
	// `required,email` on a *string: nil pointer fails required; non-nil
	// pointer applies email to the dereferenced value.
	type req struct {
		Email *string `json:"email" validate:"required,email"`
	}

	if err := validator.Struct(req{}); err == nil {
		t.Error("expected required to fail on nil pointer")
	}

	invalid := "not-an-email"
	err := validator.Struct(req{Email: &invalid})
	if err == nil {
		t.Fatal("expected email rule to fail on invalid non-nil *string")
	}
	var fe espresso.FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("expected FieldErrors, got %T", err)
	}
	// required passes (pointer non-nil), email fails: exactly 1 error.
	if len(fe) != 1 {
		t.Errorf("expected 1 error (email only, required satisfied by non-nil pointer), got %d: %v", len(fe), fe)
	}

	valid := "user@example.com"
	if err := validator.Struct(req{Email: &valid}); err != nil {
		t.Errorf("required+email should pass with valid pointer, got %v", err)
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
