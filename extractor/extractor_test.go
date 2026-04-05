package extractor

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQueryExtractor_Basic(t *testing.T) {
	type TestReq struct {
		Name  string `query:"name"`
		Email string `query:"email"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?name=john&email=john@example.com", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err != nil {
		t.Errorf("QueryExtractor.Extract() error = %v", err)
	}

	if ext.Data.Name != "john" {
		t.Errorf("expected Name 'john', got '%s'", ext.Data.Name)
	}
	if ext.Data.Email != "john@example.com" {
		t.Errorf("expected Email 'john@example.com', got '%s'", ext.Data.Email)
	}
}

func TestQueryExtractor_IntTypes(t *testing.T) {
	type TestReq struct {
		Page  int `query:"page"`
		Limit int `query:"limit"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?page=1&limit=100", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err != nil {
		t.Errorf("QueryExtractor.Extract() error = %v", err)
	}

	if ext.Data.Page != 1 {
		t.Errorf("expected Page 1, got %d", ext.Data.Page)
	}
	if ext.Data.Limit != 100 {
		t.Errorf("expected Limit 100, got %d", ext.Data.Limit)
	}
}

func TestQueryExtractor_RequiredField(t *testing.T) {
	type TestReq struct {
		Required string `query:"required,required"`
		Optional string `query:"optional"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?optional=value", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for missing required field")
	}
}

func TestQueryExtractor_Reset(t *testing.T) {
	type TestReq struct {
		Name string `query:"name"`
	}

	ext := &QueryExtractor[TestReq]{Data: TestReq{Name: "test"}}
	ext.Reset()

	if ext.Data.Name != "" {
		t.Errorf("expected empty Name after reset, got '%s'", ext.Data.Name)
	}
}

func TestFormExtractor_Basic(t *testing.T) {
	type TestReq struct {
		Username string `form:"username"`
		Password string `form:"password"`
	}

	body := "username=john&password=secret"
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ext := &FormExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("FormExtractor.Extract() error = %v", err)
	}

	if ext.Data.Username != "john" {
		t.Errorf("expected Username 'john', got '%s'", ext.Data.Username)
	}
	if ext.Data.Password != "secret" {
		t.Errorf("expected Password 'secret', got '%s'", ext.Data.Password)
	}
}

func TestPathExtractor_Basic(t *testing.T) {
	type TestReq struct {
		ID string `path:"id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.SetPathValue("id", "123")

	ext := &PathExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("PathExtractor.Extract() error = %v", err)
	}

	if ext.Data.ID != "123" {
		t.Errorf("expected ID '123', got '%s'", ext.Data.ID)
	}
}

func TestHeaderExtractor_Basic(t *testing.T) {
	type TestReq struct {
		Auth string `header:"Authorization"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer token123")

	ext := &HeaderExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("HeaderExtractor.Extract() error = %v", err)
	}

	if ext.Data.Auth != "Bearer token123" {
		t.Errorf("expected Auth 'Bearer token123', got '%s'", ext.Data.Auth)
	}
}

func TestRawBodyExtractor_Basic(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte("test body")))
	req.Header.Set("Content-Type", "text/plain")

	ext := &RawBodyExtractor{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("RawBodyExtractor.Extract() error = %v", err)
	}

	if string(ext.Data) != "test body" {
		t.Errorf("expected body 'test body', got '%s'", string(ext.Data))
	}
}

func TestRawBodyExtractor_Reset(t *testing.T) {
	ext := &RawBodyExtractor{Data: []byte("test data")}
	ext.Reset()

	if len(ext.Data) != 0 {
		t.Errorf("expected empty Data after reset, got '%s'", string(ext.Data))
	}
}

func TestXMLExtractor_Basic(t *testing.T) {
	type TestReq struct {
		Name string `xml:"name"`
	}

	xmlBody := `<TestReq><name>john</name></TestReq>`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(xmlBody))
	req.Header.Set("Content-Type", "application/xml")

	ext := &XMLExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("XMLExtractor.Extract() error = %v", err)
	}

	if ext.Data.Name != "john" {
		t.Errorf("expected Name 'john', got '%s'", ext.Data.Name)
	}
}

func TestPathParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	params := PathParams{"id": "123", "name": "test"}

	req = SetPathParams(req, params)

	retrieved := GetPathParams(req)
	if retrieved["id"] != "123" {
		t.Errorf("expected id '123', got '%s'", retrieved["id"])
	}
	if retrieved["name"] != "test" {
		t.Errorf("expected name 'test', got '%s'", retrieved["name"])
	}
}

func TestTypeConversionError(t *testing.T) {
	err := &TypeConversionError{
		Field:    "age",
		Expected: "int",
		Actual:   "string",
		Value:    "abc",
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}

	fieldErr := err.ToFieldError()
	if fieldErr.Field != "age" {
		t.Errorf("expected Field 'age', got '%s'", fieldErr.Field)
	}
}

func TestFieldErrors(t *testing.T) {
	errors := FieldErrors{
		{Field: "name", Message: "required"},
		{Field: "email", Message: "invalid"},
	}

	if errors.Error() != "2 validation errors" {
		t.Errorf("expected '2 validation errors', got '%s'", errors.Error())
	}

	validationErrors := errors.ToValidationErrors()
	if len(validationErrors) != 2 {
		t.Errorf("expected 2 validation errors, got %d", len(validationErrors))
	}
}

func TestNewFieldErrors(t *testing.T) {
	errors := NewFieldErrors()
	if errors == nil {
		t.Error("expected non-nil FieldErrors")
	}
}

func TestRequiredFieldError(t *testing.T) {
	err := RequiredFieldError("name", "user")
	if err.Field != "name" {
		t.Errorf("expected Field 'name', got '%s'", err.Field)
	}
	if err.Message != "required field is missing" {
		t.Errorf("expected 'required field is missing', got '%s'", err.Message)
	}
}

func TestInvalidTypeError(t *testing.T) {
	err := InvalidTypeError("age", "int", "string", "abc")
	if err.Field != "age" {
		t.Errorf("expected Field 'age', got '%s'", err.Field)
	}
}

func BenchmarkQueryExtractor_Extract(b *testing.B) {
	type TestReq struct {
		Name  string `query:"name"`
		Email string `query:"email"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?name=john&email=john@example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ext := &QueryExtractor[TestReq]{}
		_ = ext.Extract(req)
	}
}
