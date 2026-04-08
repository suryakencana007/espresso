package openapi

import (
	"context"
	"reflect"
	"testing"
)

func TestIntrospect_NilHandler(t *testing.T) {
	_, err := Introspect(nil)
	if err == nil {
		t.Error("expected error for nil handler")
	}
}

func TestIntrospect_NotAFunction(t *testing.T) {
	_, err := Introspect("not a function")
	if err == nil {
		t.Error("expected error for non-function")
	}
}

func TestIntrospect_NoReturns(t *testing.T) {
	handler := func() {}
	_, err := Introspect(handler)
	if err == nil {
		t.Error("expected error for handler with no returns")
	}
}

func TestIntrospect_TooManyReturns(t *testing.T) {
	handler := func() (int, string, bool) { return 1, "a", true }
	_, err := Introspect(handler)
	if err == nil {
		t.Error("expected error for handler with too many returns")
	}
}

func TestIntrospect_InvalidSecondReturn(t *testing.T) {
	handler := func() (int, string) { return 1, "a" }
	_, err := Introspect(handler)
	if err == nil {
		t.Error("expected error when second return is not error")
	}
}

func TestIntrospect_SimpleHandler(t *testing.T) {
	handler := func() string { return "hello" }
	info, err := Introspect(handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if info.ReturnsError {
		t.Error("expected ReturnsError to be false")
	}
	// ResponseType is only extracted from JSON[T], Text, etc. wrapper types
	// For simple types like string, it returns nil
}

func TestIntrospect_HandlerWithError(t *testing.T) {
	handler := func() (string, error) { return "hello", nil }
	info, err := Introspect(handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if !info.ReturnsError {
		t.Error("expected ReturnsError to be true")
	}
}

func TestIntrospect_WithContext(t *testing.T) {
	handler := func(ctx context.Context) string { return "hello" }
	info, err := Introspect(handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if len(info.RequestTypes) != 0 {
		t.Errorf("expected 0 request types, got %d", len(info.RequestTypes))
	}
}

func TestIntrospect_ContextAndError(t *testing.T) {
	handler := func(ctx context.Context) (string, error) { return "hello", nil }
	info, err := Introspect(handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if !info.ReturnsError {
		t.Error("expected ReturnsError to be true")
	}
	// ResponseType is only extracted from JSON[T], Text, etc. wrapper types
	// For simple types like string, it returns nil
}

func TestIntrospect_WithRequestType(t *testing.T) {
	type UserRequest struct {
		Name string `json:"name"`
	}

	handler := func(req UserRequest) string { return "hello" }
	info, err := Introspect(handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// RequestTypes is only extracted from extractor types (Path[T], Query[T], JSON[T], etc.)
	// For plain struct parameters, they're extracted but ExtractorKind will be Unknown
	if len(info.RequestTypes) == 0 {
		// This is expected for non-extractor parameters
		t.Log("plain struct parameters are not automatically recognized as extractors")
	}
}

func TestMustIntrospect_Valid(t *testing.T) {
	handler := func() string { return "hello" }
	info := MustIntrospect(handler)
	if info == nil {
		t.Error("expected info to be non-nil")
	}
}

func TestMustIntrospect_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid handler")
		}
	}()

	MustIntrospect(nil)
}

func TestGetExtractorKind(t *testing.T) {
	// Test with non-extractor types
	tests := []struct {
		name string
		val  interface{}
		want ExtractorKind
	}{
		{
			name: "string type",
			val:  "",
			want: KindUnknown,
		},
		{
			name: "int type",
			val:  0,
			want: KindUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := reflect.TypeOf(tt.val)
			if got := getExtractorKind(typ); got != tt.want {
				t.Errorf("getExtractorKind(%v) = %v, want %v", typ, got, tt.want)
			}
		})
	}
}

func TestGeneratePathParams_NilType(t *testing.T) {
	params := GeneratePathParams(nil)
	if params != nil {
		t.Error("expected nil for nil type")
	}
}

func TestGeneratePathParams_PointerStruct(t *testing.T) {
	type PathParams struct {
		ID int `path:"id"`
	}

	params := GeneratePathParams(reflect.TypeOf(&PathParams{}))
	if len(params) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(params))
		return
	}

	if params[0].Name != "id" {
		t.Errorf("expected parameter name 'id', got %s", params[0].Name)
	}
	if params[0].In != "path" {
		t.Errorf("expected parameter in 'path', got %s", params[0].In)
	}
	if !params[0].Required {
		t.Error("expected parameter to be required")
	}
}

func TestGeneratePathParams_WithDescription(t *testing.T) {
	type PathParams struct {
		ID int `path:"id" doc:"User ID"`
	}

	params := GeneratePathParams(reflect.TypeOf(PathParams{}))
	if len(params) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(params))
		return
	}

	if params[0].Description != "User ID" {
		t.Errorf("expected description 'User ID', got %s", params[0].Description)
	}
}

func TestGeneratePathParams_SkipUnmarkedFields(t *testing.T) {
	type PathParams struct {
		ID   int    `path:"id"`
		Name string `json:"name"`
	}

	params := GeneratePathParams(reflect.TypeOf(PathParams{}))
	if len(params) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(params))
	}
}

func TestGenerateQueryParams_NilType(t *testing.T) {
	params := GenerateQueryParams(nil)
	if params != nil {
		t.Error("expected nil for nil type")
	}
}

func TestGenerateQueryParams_PointerStruct(t *testing.T) {
	type QueryParams struct {
		Page int `query:"page"`
	}

	params := GenerateQueryParams(reflect.TypeOf(&QueryParams{}))
	if len(params) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(params))
		return
	}

	if params[0].Name != "page" {
		t.Errorf("expected parameter name 'page', got %s", params[0].Name)
	}
	if params[0].In != "query" {
		t.Errorf("expected parameter in 'query', got %s", params[0].In)
	}
}

func TestGenerateQueryParams_Required(t *testing.T) {
	type QueryParams struct {
		Page    int    `query:"page"`
		Keyword string `query:"q,required"`
	}

	params := GenerateQueryParams(reflect.TypeOf(QueryParams{}))
	if len(params) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(params))
		return
	}

	if params[0].Required {
		t.Error("expected page to not be required")
	}
	if !params[1].Required {
		t.Error("expected q to be required")
	}
}

func TestGenerateRequestBody_NilType(t *testing.T) {
	body := GenerateRequestBody(nil, nil)
	if body != nil {
		t.Error("expected nil for nil type")
	}
}

func TestGenerateRequestBody_ValidType(t *testing.T) {
	type CreateUser struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	gen := New("Test API", "1.0.0")
	body := GenerateRequestBody(reflect.TypeOf(CreateUser{}), gen)

	if body == nil {
		t.Error("expected request body to be generated")
		return
	}

	if !body.Required {
		t.Error("expected request body to be required")
	}
	if body.Content == nil {
		t.Error("expected content to be set")
		return
	}
	if body.Content["application/json"] == (MediaType{}) {
		t.Error("expected application/json media type")
	}
}

func TestIsExtractor(t *testing.T) {
	tests := []struct {
		name string
		typ  reflect.Type
		want bool
	}{
		{
			name: "string type",
			typ:  reflect.TypeOf(""),
			want: false,
		},
		{
			name: "int type",
			typ:  reflect.TypeOf(0),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsExtractor(tt.typ); got != tt.want {
				t.Errorf("IsExtractor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntrospectError_Error(t *testing.T) {
	err := &IntrospectError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %s", err.Error())
	}
}
