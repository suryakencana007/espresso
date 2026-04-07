package extractor

import (
	"bytes"
	"mime/multipart"
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

func TestQueryExtractor_UintTypes(t *testing.T) {
	type TestReq struct {
		ID    uint   `query:"id"`
		Count uint64 `query:"count"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?id=42&count=100", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err != nil {
		t.Errorf("QueryExtractor.Extract() error = %v", err)
	}

	if ext.Data.ID != 42 {
		t.Errorf("expected ID 42, got %d", ext.Data.ID)
	}
	if ext.Data.Count != 100 {
		t.Errorf("expected Count 100, got %d", ext.Data.Count)
	}
}

func TestQueryExtractor_FloatTypes(t *testing.T) {
	type TestReq struct {
		Price float32 `query:"price"`
		Rate  float64 `query:"rate"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?price=19.99&rate=3.14159", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err != nil {
		t.Errorf("QueryExtractor.Extract() error = %v", err)
	}

	if ext.Data.Price != 19.99 {
		t.Errorf("expected Price 19.99, got %f", ext.Data.Price)
	}
	if ext.Data.Rate != 3.14159 {
		t.Errorf("expected Rate 3.14159, got %f", ext.Data.Rate)
	}
}

func TestQueryExtractor_BoolTypes(t *testing.T) {
	type TestReq struct {
		Active  bool `query:"active"`
		Enabled bool `query:"enabled"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?active=true&enabled=false", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err != nil {
		t.Errorf("QueryExtractor.Extract() error = %v", err)
	}

	if !ext.Data.Active {
		t.Errorf("expected Active true, got %v", ext.Data.Active)
	}
	if ext.Data.Enabled {
		t.Errorf("expected Enabled false, got %v", ext.Data.Enabled)
	}
}

func TestQueryExtractor_IntConversionError(t *testing.T) {
	type TestReq struct {
		Age int `query:"age"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?age=invalid", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for invalid integer")
	}

	var typeErr *TypeConversionError
	if err != nil {
		if te, ok := err.(*TypeConversionError); ok {
			typeErr = te
		}
	}
	if typeErr == nil {
		t.Errorf("expected TypeConversionError, got %T", err)
	}
}

func TestQueryExtractor_FloatConversionError(t *testing.T) {
	type TestReq struct {
		Price float64 `query:"price"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?price=invalid", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for invalid float")
	}
}

func TestQueryExtractor_BoolConversionError(t *testing.T) {
	type TestReq struct {
		Active bool `query:"active"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?active=invalid", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for invalid boolean")
	}
}

func TestQueryExtractor_UintConversionError(t *testing.T) {
	type TestReq struct {
		ID uint `query:"id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?id=invalid", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for invalid uint")
	}
}

func TestQueryExtractor_NegativeInt(t *testing.T) {
	type TestReq struct {
		Value int `query:"value"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?value=-42", nil)
	ext := &QueryExtractor[TestReq]{}

	err := ext.Extract(req)
	if err != nil {
		t.Errorf("QueryExtractor.Extract() error = %v", err)
	}

	if ext.Data.Value != -42 {
		t.Errorf("expected Value -42, got %d", ext.Data.Value)
	}
}

func TestFormExtractor_RequiredField(t *testing.T) {
	type TestReq struct {
		Required string `form:"required,required"`
		Optional string `form:"optional"`
	}

	body := "optional=value"
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ext := &FormExtractor[TestReq]{}
	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for missing required field")
	}
}

func TestFormExtractor_InvalidFormData(t *testing.T) {
	type TestReq struct {
		Name string `form:"name"`
	}

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("invalid%form"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ext := &FormExtractor[TestReq]{}
	// This should still work - Go's ParseForm is lenient
	_ = ext.Extract(req)
}

func TestFormExtractor_IntTypes(t *testing.T) {
	type TestReq struct {
		Age   int `form:"age"`
		Count int `form:"count"`
	}

	body := "age=25&count=100"
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ext := &FormExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("FormExtractor.Extract() error = %v", err)
	}

	if ext.Data.Age != 25 {
		t.Errorf("expected Age 25, got %d", ext.Data.Age)
	}
}

func TestFormExtractor_Reset(t *testing.T) {
	type TestReq struct {
		Name string `form:"name"`
	}

	ext := &FormExtractor[TestReq]{Data: TestReq{Name: "test"}}
	ext.Reset()

	if ext.Data.Name != "" {
		t.Errorf("expected empty Name after reset, got '%s'", ext.Data.Name)
	}
}

func TestPathExtractor_RequiredField(t *testing.T) {
	type TestReq struct {
		ID string `path:"id,required"`
	}

	req := httptest.NewRequest(http.MethodGet, "/users/", nil)
	// Don't set path value - should error on required field

	ext := &PathExtractor[TestReq]{}
	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for missing required path field")
	}
}

func TestPathExtractor_IntTypes(t *testing.T) {
	type TestReq struct {
		ID int `path:"id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.SetPathValue("id", "123")

	ext := &PathExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("PathExtractor.Extract() error = %v", err)
	}

	if ext.Data.ID != 123 {
		t.Errorf("expected ID 123, got %d", ext.Data.ID)
	}
}

func TestPathExtractor_Reset(t *testing.T) {
	type TestReq struct {
		ID string `path:"id"`
	}

	ext := &PathExtractor[TestReq]{Data: TestReq{ID: "123"}}
	ext.Reset()

	if ext.Data.ID != "" {
		t.Errorf("expected empty ID after reset, got '%s'", ext.Data.ID)
	}
}

func TestHeaderExtractor_RequiredField(t *testing.T) {
	type TestReq struct {
		Token string `header:"Authorization,required"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Don't set header - should error on required field

	ext := &HeaderExtractor[TestReq]{}
	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for missing required header")
	}
}

func TestHeaderExtractor_IntTypes(t *testing.T) {
	type TestReq struct {
		Count int `header:"X-Count"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Count", "42")

	ext := &HeaderExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("HeaderExtractor.Extract() error = %v", err)
	}

	if ext.Data.Count != 42 {
		t.Errorf("expected Count 42, got %d", ext.Data.Count)
	}
}

func TestHeaderExtractor_Reset(t *testing.T) {
	type TestReq struct {
		Token string `header:"Authorization"`
	}

	ext := &HeaderExtractor[TestReq]{Data: TestReq{Token: "test"}}
	ext.Reset()

	if ext.Data.Token != "" {
		t.Errorf("expected empty Token after reset, got '%s'", ext.Data.Token)
	}
}

func TestRawBodyExtractor_LargeBody(t *testing.T) {
	largeBody := make([]byte, 100*1024) // 100KB
	for i := range largeBody {
		largeBody[i] = 'x'
	}

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/octet-stream")

	ext := &RawBodyExtractor{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("RawBodyExtractor.Extract() error = %v", err)
	}

	if len(ext.Data) != len(largeBody) {
		t.Errorf("expected body length %d, got %d", len(largeBody), len(ext.Data))
	}

	// Test reset of large body (> 64KB threshold)
	ext.Reset()
	if ext.Data != nil {
		t.Errorf("expected nil Data after reset of large body, got len %d", len(ext.Data))
	}
}

func TestXMLExtractor_Reset(t *testing.T) {
	type TestReq struct {
		Name string `xml:"name"`
	}

	ext := &XMLExtractor[TestReq]{Data: TestReq{Name: "test"}}
	ext.Reset()

	if ext.Data.Name != "" {
		t.Errorf("expected empty Name after reset, got '%s'", ext.Data.Name)
	}
}

func TestXMLExtractor_WriteResponse(t *testing.T) {
	type TestReq struct {
		Name string `xml:"name"`
	}

	ext := &XMLExtractor[TestReq]{Data: TestReq{Name: "john"}}

	rec := httptest.NewRecorder()
	err := ext.WriteResponse(rec)
	if err != nil {
		t.Errorf("XMLExtractor.WriteResponse() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "application/xml" {
		t.Errorf("expected Content-Type 'application/xml', got '%s'", rec.Header().Get("Content-Type"))
	}

	body := rec.Body.String()
	if !strings.Contains(body, "john") {
		t.Errorf("expected body to contain 'john', got '%s'", body)
	}
}

func TestXMLExtractor_WriteResponseWithStatus(t *testing.T) {
	type StatusReq struct {
		Name string `xml:"name"`
	}

	ext := &XMLExtractor[StatusReq]{Data: StatusReq{Name: "test"}}

	rec := httptest.NewRecorder()
	err := ext.WriteResponse(rec)
	if err != nil {
		t.Errorf("XMLExtractor.WriteResponse() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestFieldErrors_Empty(t *testing.T) {
	errors := FieldErrors{}

	if errors.Error() != "validation errors" {
		t.Errorf("expected 'validation errors', got '%s'", errors.Error())
	}
}

func TestFieldErrors_Single(t *testing.T) {
	errors := FieldErrors{
		{Field: "name", Message: "required"},
	}

	if errors.Error() != "required" {
		t.Errorf("expected 'required', got '%s'", errors.Error())
	}
}

func TestFieldErrors_AddFieldError(t *testing.T) {
	errors := NewFieldErrors()
	_ = errors.AddFieldError("name", "required", nil)
	_ = errors.AddFieldError("email", "invalid", "test@example.com", "user")

	if len(*errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(*errors))
	}

	if (*errors)[0].Field != "name" {
		t.Errorf("expected field 'name', got '%s'", (*errors)[0].Field)
	}

	if (*errors)[1].Path != "user" {
		t.Errorf("expected path 'user', got '%s'", (*errors)[1].Path)
	}
}

func TestUnsupportedTypeError(t *testing.T) {
	err := &UnsupportedTypeError{
		Field:    "data",
		Expected: "string",
		Actual:   "map",
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}

	fieldErr := err.ToFieldError()
	if fieldErr.Field != "data" {
		t.Errorf("expected Field 'data', got '%s'", fieldErr.Field)
	}
}

func TestUnsupportedTypeError_NoField(t *testing.T) {
	err := &UnsupportedTypeError{
		Expected: "string",
		Actual:   "map",
	}

	if !strings.Contains(err.Error(), "unsupported type") {
		t.Errorf("expected 'unsupported type' in error, got '%s'", err.Error())
	}
}

func TestTypeConversionError_NoField(t *testing.T) {
	err := &TypeConversionError{
		Expected: "int",
		Actual:   "string",
		Value:    "abc",
	}

	if !strings.Contains(err.Error(), "cannot convert") {
		t.Errorf("expected 'cannot convert' in error, got '%s'", err.Error())
	}
}

func TestTypeConversionError_WithField(t *testing.T) {
	err := &TypeConversionError{
		Field:    "age",
		Expected: "int",
		Actual:   "string",
		Value:    "abc",
	}

	if !strings.Contains(err.Error(), "field 'age'") {
		t.Errorf("expected 'field' in error, got '%s'", err.Error())
	}
}

func TestGetValueFromCache(t *testing.T) {
	type TestReq struct {
		Name string `query:"name"`
	}

	// First call - should populate cache
	req1 := httptest.NewRequest(http.MethodGet, "/test?name=test1", nil)
	ext1 := &QueryExtractor[TestReq]{}
	_ = ext1.Extract(req1)

	// Second call - should use cache
	req2 := httptest.NewRequest(http.MethodGet, "/test?name=test2", nil)
	ext2 := &QueryExtractor[TestReq]{}
	err := ext2.Extract(req2)
	if err != nil {
		t.Errorf("QueryExtractor.Extract() error = %v", err)
	}

	if ext2.Data.Name != "test2" {
		t.Errorf("expected Name 'test2', got '%s'", ext2.Data.Name)
	}
}

func TestUnexportedField(t *testing.T) {
	type TestReq struct {
		_    string `query:"-"`
		Name string `query:"name"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?name=test", nil)
	ext := &QueryExtractor[TestReq]{}
	// Should not error, just skip unexported field
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestIgnoredField(t *testing.T) {
	type TestReq struct {
		Name string `query:"-"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test?name=test", nil)
	ext := &QueryExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Field should remain empty (ignored)
	if ext.Data.Name != "" {
		t.Errorf("expected empty Name for ignored field, got '%s'", ext.Data.Name)
	}
}

func TestPathParams_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	retrieved := GetPathParams(req)

	if retrieved == nil {
		t.Error("expected non-nil PathParams")
	}

	if len(retrieved) != 0 {
		t.Errorf("expected empty PathParams, got %d items", len(retrieved))
	}
}

func TestFieldErrors_Multiple(t *testing.T) {
	errors := FieldErrors{
		{Field: "name", Message: "required"},
		{Field: "email", Message: "invalid"},
		{Field: "age", Message: "must be positive"},
	}

	if errors.Error() != "3 validation errors" {
		t.Errorf("expected '3 validation errors', got '%s'", errors.Error())
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

func BenchmarkFormExtractor_Extract(b *testing.B) {
	type TestReq struct {
		Username string `form:"username"`
		Password string `form:"password"`
	}

	body := "username=john&password=secret"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		ext := &FormExtractor[TestReq]{}
		_ = ext.Extract(req)
	}
}

func BenchmarkPathExtractor_Extract(b *testing.B) {
	type TestReq struct {
		ID int `path:"id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.SetPathValue("id", "123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ext := &PathExtractor[TestReq]{}
		_ = ext.Extract(req)
	}
}

func BenchmarkHeaderExtractor_Extract(b *testing.B) {
	type TestReq struct {
		Auth string `header:"Authorization"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer token123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ext := &HeaderExtractor[TestReq]{}
		_ = ext.Extract(req)
	}
}

func BenchmarkCachedFields(b *testing.B) {
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

func TestCookieExtractor_Basic(t *testing.T) {
	type TestReq struct {
		SessionID string `cookie:"session_id"`
		UserID    string `cookie:"user_id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "abc123"})
	req.AddCookie(&http.Cookie{Name: "user_id", Value: "user456"})

	ext := &CookieExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("CookieExtractor.Extract() error = %v", err)
	}

	if ext.Data.SessionID != "abc123" {
		t.Errorf("expected SessionID 'abc123', got '%s'", ext.Data.SessionID)
	}
	if ext.Data.UserID != "user456" {
		t.Errorf("expected UserID 'user456', got '%s'", ext.Data.UserID)
	}
}

func TestCookieExtractor_IntTypes(t *testing.T) {
	type TestReq struct {
		Count int `cookie:"count"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "count", Value: "42"})

	ext := &CookieExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("CookieExtractor.Extract() error = %v", err)
	}

	if ext.Data.Count != 42 {
		t.Errorf("expected Count 42, got %d", ext.Data.Count)
	}
}

func TestCookieExtractor_RequiredField(t *testing.T) {
	type TestReq struct {
		SessionID string `cookie:"session_id,required"`
		Optional  string `cookie:"optional"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Don't add session_id cookie - should error on required field

	ext := &CookieExtractor[TestReq]{}
	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for missing required cookie")
	}
}

func TestCookieExtractor_OptionalField(t *testing.T) {
	type TestReq struct {
		Required string `cookie:"required,required"`
		Optional string `cookie:"optional"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "required", Value: "present"})

	ext := &CookieExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("CookieExtractor.Extract() error = %v", err)
	}

	if ext.Data.Required != "present" {
		t.Errorf("expected Required 'present', got '%s'", ext.Data.Required)
	}
	if ext.Data.Optional != "" {
		t.Errorf("expected empty Optional, got '%s'", ext.Data.Optional)
	}
}

func TestCookieExtractor_Reset(t *testing.T) {
	type TestReq struct {
		SessionID string `cookie:"session_id"`
	}

	ext := &CookieExtractor[TestReq]{Data: TestReq{SessionID: "test"}}
	ext.Reset()

	if ext.Data.SessionID != "" {
		t.Errorf("expected empty SessionID after reset, got '%s'", ext.Data.SessionID)
	}
}

func TestCookieExtractor_BoolTypes(t *testing.T) {
	type TestReq struct {
		Active  bool `cookie:"active"`
		Enabled bool `cookie:"enabled"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "active", Value: "true"})
	req.AddCookie(&http.Cookie{Name: "enabled", Value: "false"})

	ext := &CookieExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("CookieExtractor.Extract() error = %v", err)
	}

	if !ext.Data.Active {
		t.Errorf("expected Active true, got %v", ext.Data.Active)
	}
	if ext.Data.Enabled {
		t.Errorf("expected Enabled false, got %v", ext.Data.Enabled)
	}
}

func TestCookieExtractor_TypeAlias(t *testing.T) {
	type TestReq struct {
		Token string `cookie:"token"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "secret"})

	ext := &Cookie[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("Cookie.Extract() error = %v", err)
	}

	if ext.Data.Token != "secret" {
		t.Errorf("expected Token 'secret', got '%s'", ext.Data.Token)
	}
}

func BenchmarkCookieExtractor_Extract(b *testing.B) {
	type TestReq struct {
		SessionID string `cookie:"session_id"`
		UserID    string `cookie:"user_id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "abc123"})
	req.AddCookie(&http.Cookie{Name: "user_id", Value: "user456"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ext := &CookieExtractor[TestReq]{}
		_ = ext.Extract(req)
	}
}

func TestMultipartExtractor_Basic(t *testing.T) {
	type TestReq struct {
		Name  string `form:"name"`
		Email string `form:"email"`
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "john")
	_ = writer.WriteField("email", "john@example.com")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	ext := &MultipartExtractor[TestReq]{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("MultipartExtractor.Extract() error = %v", err)
	}

	if ext.Data.Name != "john" {
		t.Errorf("expected Name 'john', got '%s'", ext.Data.Name)
	}
	if ext.Data.Email != "john@example.com" {
		t.Errorf("expected Email 'john@example.com', got '%s'", ext.Data.Email)
	}
}

func TestMultipartExtractor_RequiredField(t *testing.T) {
	type TestReq struct {
		Required string `form:"required,required"`
		Optional string `form:"optional"`
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("optional", "value")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	ext := &MultipartExtractor[TestReq]{}
	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for missing required field")
	}
}

func TestMultipartExtractor_Reset(t *testing.T) {
	type TestReq struct {
		Name string `form:"name"`
	}

	ext := &MultipartExtractor[TestReq]{Data: TestReq{Name: "test"}}
	ext.Reset()

	if ext.Data.Name != "" {
		t.Errorf("expected empty Name after reset, got '%s'", ext.Data.Name)
	}
}

func TestFileExtractor_Basic(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	_, _ = part.Write([]byte("hello world"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	ext := &FileExtractor{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("FileExtractor.Extract() error = %v", err)
	}

	if ext.File.Filename != "test.txt" {
		t.Errorf("expected Filename 'test.txt', got '%s'", ext.File.Filename)
	}
	if ext.File.Size != 11 {
		t.Errorf("expected Size 11, got %d", ext.File.Size)
	}
}

func TestFileExtractor_MissingFile(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	ext := &FileExtractor{}
	err := ext.Extract(req)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestFileExtractor_Reset(t *testing.T) {
	ext := &FileExtractor{File: FileInfo{Filename: "test.txt", Size: 100}}
	ext.Reset()

	if ext.File.Filename != "" {
		t.Errorf("expected empty Filename after reset, got '%s'", ext.File.Filename)
	}
}

func TestFilesExtractor_Basic(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part1, _ := writer.CreateFormFile("files", "file1.txt")
	_, _ = part1.Write([]byte("content1"))
	part2, _ := writer.CreateFormFile("files", "file2.txt")
	_, _ = part2.Write([]byte("content2"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	ext := &FilesExtractor{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("FilesExtractor.Extract() error = %v", err)
	}

	if len(ext.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(ext.Files))
	}
}

func TestFilesExtractor_NoFiles(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	ext := &FilesExtractor{}
	err := ext.Extract(req)
	if err != nil {
		t.Errorf("FilesExtractor.Extract() error = %v", err)
	}

	if len(ext.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(ext.Files))
	}
}

func TestFilesExtractor_Reset(t *testing.T) {
	ext := &FilesExtractor{Files: []FileInfo{{Filename: "test.txt"}}}
	ext.Reset()

	if ext.Files != nil {
		t.Errorf("expected nil Files after reset, got %v", ext.Files)
	}
}

func BenchmarkMultipartExtractor_Extract(b *testing.B) {
	type TestReq struct {
		Name  string `form:"name"`
		Email string `form:"email"`
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "john")
	_ = writer.WriteField("email", "john@example.com")
	_ = writer.Close()
	contentType := writer.FormDataContentType()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body.Bytes()))
		req.Header.Set("Content-Type", contentType)
		ext := &MultipartExtractor[TestReq]{}
		_ = ext.Extract(req)
	}
}

func BenchmarkFileExtractor_Extract(b *testing.B) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	_, _ = part.Write([]byte("hello world"))
	_ = writer.Close()
	contentType := writer.FormDataContentType()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body.Bytes()))
		req.Header.Set("Content-Type", contentType)
		ext := &FileExtractor{}
		_ = ext.Extract(req)
	}
}
