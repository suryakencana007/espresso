package espresso

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/suryakencana007/espresso/openapi"
)

func TestOpenAPIRouter_Basic(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0").
		Description("Test Description").
		Server("http://localhost:8080", "Development")

	router := OpenAPI(gen)

	if router == nil {
		t.Error("expected router to be non-nil")
		return
	}
	if router.gen == nil {
		t.Error("expected generator to be initialized")
	}
	if router.router == nil {
		t.Error("expected underlying router to be initialized")
	}
}

func TestOpenAPIRouter_Get(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	handler := func() Text {
		return Text{Body: "OK"}
	}

	router.Get("/health", handler, openapi.Tags("health"))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	spec := gen.Spec()
	pathItem, exists := spec.Paths["/health"]
	if !exists {
		t.Error("expected /health path to be registered")
	}
	if pathItem.Get == nil {
		t.Error("expected GET /health to be registered")
		return
	}
	// Check that tags were applied
	if len(pathItem.Get.Tags) == 0 || pathItem.Get.Tags[0] != "health" {
		t.Errorf("expected tags ['health'], got %v", pathItem.Get.Tags)
	}
}

func TestOpenAPIRouter_Post(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	type CreateUser struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	handler := func(ctx context.Context, req *JSON[CreateUser]) (JSON[CreateUser], error) {
		return JSON[CreateUser]{Data: req.Data}, nil
	}

	router.Post("/users", handler, openapi.Summary("Create user"), openapi.Tags("users"))

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	spec := gen.Spec()
	pathItem, exists := spec.Paths["/users"]
	if !exists {
		t.Error("expected /users path to be registered")
	}
	if pathItem.Post == nil {
		t.Error("expected POST /users to be registered")
		return
	}

	op := pathItem.Post
	if op.Summary != "Create user" {
		t.Errorf("expected summary 'Create user', got %s", op.Summary)
	}
	if len(op.Tags) == 0 || op.Tags[0] != "users" {
		t.Errorf("expected tags ['users'], got %v", op.Tags)
	}
}

func TestOpenAPIRouter_Put(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	handler := func() Text {
		return Text{Body: "Updated"}
	}

	router.Put("/users/123", handler, openapi.Tags("users"))

	spec := gen.Spec()
	pathItem, exists := spec.Paths["/users/123"]
	if !exists {
		t.Error("expected /users/123 path to be registered")
	}
	if pathItem.Put == nil {
		t.Error("expected PUT /users/123 to be registered")
	}
}

func TestOpenAPIRouter_Delete(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	handler := func() Status {
		return http.StatusNoContent
	}

	router.Delete("/users/123", handler, openapi.Tags("users"))

	spec := gen.Spec()
	pathItem, exists := spec.Paths["/users/123"]
	if !exists {
		t.Error("expected /users/123 path to be registered")
	}
	if pathItem.Delete == nil {
		t.Error("expected DELETE /users/123 to be registered")
	}
}

func TestOpenAPIRouter_Patch(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	handler := func() Text {
		return Text{Body: "Patched"}
	}

	router.Patch("/users/123", handler, openapi.Tags("users"))

	spec := gen.Spec()
	pathItem, exists := spec.Paths["/users/123"]
	if !exists {
		t.Error("expected /users/123 path to be registered")
	}
	if pathItem.Patch == nil {
		t.Error("expected PATCH /users/123 to be registered")
	}
}

func TestOpenAPIRouter_WithOptions(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	router.WithState("test-state")

	if router.router == nil {
		t.Error("expected router to be initialized")
	}
}

func TestOpenAPIRouter_Chaining(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")

	healthHandler := func() Text {
		return Text{Body: "OK"}
	}

	userHandler := func() Text {
		return Text{Body: "users"}
	}

	OpenAPI(gen).
		Get("/health", healthHandler, openapi.Tags("health")).
		Post("/users", userHandler, openapi.Tags("users")).
		Put("/users/{id}", userHandler, openapi.Tags("users")).
		Delete("/users/{id}", userHandler, openapi.Tags("users"))

	spec := gen.Spec()

	if len(spec.Paths) != 3 {
		t.Errorf("expected 3 paths, got %d", len(spec.Paths))
	}

	if _, exists := spec.Paths["/health"]; !exists {
		t.Error("expected /health path to be registered")
	}
	if _, exists := spec.Paths["/users"]; !exists {
		t.Error("expected /users path to be registered")
	}
	if _, exists := spec.Paths["/users/{id}"]; !exists {
		t.Error("expected /users/{id} path to be registered")
	}
}

func TestInferTypeFromStruct(t *testing.T) {
	type User struct {
		ID    int    `json:"id" doc:"User ID"`
		Name  string `json:"name" doc:"User name"`
		Email string `json:"email,omitempty" doc:"User email"`
	}

	gen := openapi.New("Test API", "1.0.0")
	InferTypeFromStruct(gen, "User", User{})

	spec := gen.Spec()
	schemas, ok := spec.Components["schemas"].(map[string]*openapi.Schema)
	if !ok {
		t.Error("expected schemas to be initialized")
		return
	}

	userSchema, exists := schemas["User"]
	if !exists {
		t.Error("expected User schema to be added")
		return
	}

	if userSchema.Type != "object" {
		t.Errorf("expected User type 'object', got %s", userSchema.Type)
	}

	if len(userSchema.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(userSchema.Properties))
	}
}

func TestInferTypeFromStruct_Pointer(t *testing.T) {
	type Post struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}

	gen := openapi.New("Test API", "1.0.0")
	InferTypeFromStruct(gen, "Post", &Post{})

	spec := gen.Spec()
	schemas, ok := spec.Components["schemas"].(map[string]*openapi.Schema)
	if !ok {
		t.Error("expected schemas to be initialized")
		return
	}

	if _, exists := schemas["Post"]; !exists {
		t.Error("expected Post schema to be added")
	}
}

func TestRegisterHandler(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")

	handler := func() string {
		return "hello"
	}

	err := RegisterHandler(gen, "GET", "/test", handler,
		openapi.Tags("test"),
		openapi.Summary("Test endpoint"),
		openapi.Status("200", openapi.Response{
			Description: "Success",
		}),
	)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	spec := gen.Spec()
	pathItem, exists := spec.Paths["/test"]
	if !exists {
		t.Error("expected /test path to be registered")
		return
	}

	op := pathItem.Get
	if op == nil {
		t.Error("expected GET operation")
		return
	}

	if len(op.Tags) != 1 || op.Tags[0] != "test" {
		t.Errorf("expected tags ['test'], got %v", op.Tags)
	}
	if op.Summary != "Test endpoint" {
		t.Errorf("expected summary 'Test endpoint', got %s", op.Summary)
	}
	if op.Responses["200"].Description != "Success" {
		t.Errorf("expected '200' response description 'Success', got %s", op.Responses["200"].Description)
	}
}

func TestOpenAPIRouter_Generator(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	if router.Generator() != gen {
		t.Error("expected Generator() to return the generator")
	}
}

func TestOpenAPIRouter_Router(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	if router.Router() == nil {
		t.Error("expected Router() to return the underlying router")
	}
}

func TestOpenAPIRouter_ServeOpenAPI(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	handler := func() Text {
		return Text{Body: "OK"}
	}

	router.Get("/api/test", handler, openapi.Tags("test")).
		ServeOpenAPI("/openapi.json")

	// Test that spec is accessible
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify it's valid JSON
	var spec map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Errorf("expected valid JSON, got error: %v", err)
	}

	if spec["openapi"] != "3.0.3" {
		t.Errorf("expected OpenAPI version 3.0.3, got %v", spec["openapi"])
	}
}

func TestOpenAPIRouter_ServeDocs(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	handler := func() Text {
		return Text{Body: "OK"}
	}

	router.Get("/api/test", handler, openapi.Tags("test")).
		ServeOpenAPI("/openapi.json").
		ServeDocs("/docs", "/openapi.json")

	// Test that docs endpoint is accessible
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify it's HTML (Scalar UI)
	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("expected Content-Type 'text/html; charset=utf-8', got %v", contentType)
	}
}

func TestOpenAPIRouter_ServeOpenAPIWithDocs(t *testing.T) {
	gen := openapi.New("Test API", "1.0.0")
	router := OpenAPI(gen)

	handler := func() Text {
		return Text{Body: "OK"}
	}

	router.Get("/api/test", handler).
		ServeOpenAPIWithDocs("/openapi.json", "/docs")

	// Test spec endpoint
	req1 := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("spec endpoint: expected status %d, got %d", http.StatusOK, rec1.Code)
	}

	// Test docs endpoint
	req2 := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("docs endpoint: expected status %d, got %d", http.StatusOK, rec2.Code)
	}
}
