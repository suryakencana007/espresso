package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestNewGenerator(t *testing.T) {
	g := NewGenerator("Test API", "1.0.0")

	if g.spec.OpenAPI != "3.0.3" {
		t.Errorf("expected OpenAPI version 3.0.3, got %s", g.spec.OpenAPI)
	}
	if g.spec.Info.Title != "Test API" {
		t.Errorf("expected title 'Test API', got %s", g.spec.Info.Title)
	}
	if g.spec.Info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", g.spec.Info.Version)
	}
}

func TestSetDescription(t *testing.T) {
	g := NewGenerator("Test API", "1.0.0")
	g.SetDescription("Test API Description")

	if g.spec.Info.Description != "Test API Description" {
		t.Errorf("expected description 'Test API Description', got %s", g.spec.Info.Description)
	}
}

func TestAddServer(t *testing.T) {
	g := NewGenerator("Test API", "1.0.0")
	g.AddServer("http://localhost:8080", "Local development")

	if len(g.spec.Servers) != 1 {
		t.Errorf("expected 1 server, got %d", len(g.spec.Servers))
	}
	if g.spec.Servers[0].URL != "http://localhost:8080" {
		t.Errorf("expected URL 'http://localhost:8080', got %s", g.spec.Servers[0].URL)
	}
}

func TestAddPath(t *testing.T) {
	g := NewGenerator("Test API", "1.0.0")

	op := Operation{
		Summary: "Get users",
		Responses: map[string]Response{
			"200": {
				Description: "Success",
			},
		},
	}

	g.AddPath("GET", "/users", op)

	if g.spec.Paths["/users"].Get == nil {
		t.Error("expected GET path to be set")
	}
	if g.spec.Paths["/users"].Get.Summary != "Get users" {
		t.Errorf("expected summary 'Get users', got %s", g.spec.Paths["/users"].Get.Summary)
	}
}

func TestAddPathMultiple(t *testing.T) {
	g := NewGenerator("Test API", "1.0.0")

	op1 := Operation{Summary: "Get users"}
	op2 := Operation{Summary: "Create user"}

	g.AddPath("GET", "/users", op1)
	g.AddPath("POST", "/users", op2)

	if g.spec.Paths["/users"].Get == nil {
		t.Error("expected GET to be set")
	}
	if g.spec.Paths["/users"].Post == nil {
		t.Error("expected POST to be set")
	}
}

func TestAddSchema(t *testing.T) {
	g := NewGenerator("Test API", "1.0.0")

	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"id":   {Type: "integer"},
			"name": {Type: "string"},
		},
	}

	g.AddSchema("User", schema)

	schemas, ok := g.spec.Components["schemas"].(map[string]*Schema)
	if !ok {
		t.Error("expected schemas to be map[string]*Schema")
		return
	}
	if schemas["User"] == nil {
		t.Error("expected User schema to be added")
	}
	if schemas["User"].Type != "object" {
		t.Errorf("expected type 'object', got %s", schemas["User"].Type)
	}
}

func TestToJSON(t *testing.T) {
	g := NewGenerator("Test API", "1.0.0")
	g.SetDescription("Test Description")
	g.AddPath("GET", "/users", Operation{Summary: "Get users"})

	data, err := g.ToJSON()
	if err != nil {
		t.Errorf("ToJSON() error = %v", err)
	}

	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Errorf("json.Unmarshal() error = %v", err)
	}

	if spec.Info.Title != "Test API" {
		t.Errorf("expected title 'Test API', got %s", spec.Info.Title)
	}
}

func TestGenerateSchemaFromType_String(t *testing.T) {
	schema := GenerateSchemaFromType(reflect.TypeOf(""))

	if schema.Type != "string" {
		t.Errorf("expected type 'string', got %s", schema.Type)
	}
}

func TestGenerateSchemaFromType_Int(t *testing.T) {
	schema := GenerateSchemaFromType(reflect.TypeOf(0))

	if schema.Type != "integer" {
		t.Errorf("expected type 'integer', got %s", schema.Type)
	}
}

func TestGenerateSchemaFromType_Bool(t *testing.T) {
	schema := GenerateSchemaFromType(reflect.TypeOf(false))

	if schema.Type != "boolean" {
		t.Errorf("expected type 'boolean', got %s", schema.Type)
	}
}

func TestGenerateSchemaFromType_Float(t *testing.T) {
	schema := GenerateSchemaFromType(reflect.TypeOf(0.0))

	if schema.Type != "number" {
		t.Errorf("expected type 'number', got %s", schema.Type)
	}
	if schema.Format != "double" {
		t.Errorf("expected format 'double', got %s", schema.Format)
	}
}

func TestGenerateSchemaFromType_Slice(t *testing.T) {
	schema := GenerateSchemaFromType(reflect.TypeOf([]string{}))

	if schema.Type != "array" {
		t.Errorf("expected type 'array', got %s", schema.Type)
	}
	if schema.Items == nil {
		t.Error("expected items to be set")
	}
	if schema.Items.Type != "string" {
		t.Errorf("expected items type 'string', got %s", schema.Items.Type)
	}
}

func TestGenerateSchemaFromType_Map(t *testing.T) {
	schema := GenerateSchemaFromType(reflect.TypeOf(map[string]string{}))

	if schema.Type != "object" {
		t.Errorf("expected type 'object', got %s", schema.Type)
	}
}

func TestGenerateSchemaFromType_Struct(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	schema := GenerateSchemaFromType(reflect.TypeOf(User{}))

	if schema.Type != "object" {
		t.Errorf("expected type 'object', got %s", schema.Type)
	}
	if len(schema.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(schema.Properties))
	}
	if schema.Properties["id"].Type != "integer" {
		t.Errorf("expected id type 'integer', got %s", schema.Properties["id"].Type)
	}
	if schema.Properties["name"].Type != "string" {
		t.Errorf("expected name type 'string', got %s", schema.Properties["name"].Type)
	}
}

func TestGenerateSchemaFromType_StructWithTags(t *testing.T) {
	type User struct {
		ID    int    `json:"id"`
		Name  string `json:"name" doc:"User name"`
		Email string `json:"email,omitempty"`
	}

	schema := GenerateSchemaFromType(reflect.TypeOf(User{}))

	if schema.Properties["name"].Description != "User name" {
		t.Errorf("expected description 'User name', got %s", schema.Properties["name"].Description)
	}
	// Check required fields - ID should be required (no omitempty)
	found := false
	for _, r := range schema.Required {
		if r == "id" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'id' to be required")
	}
}

func TestHandler(t *testing.T) {
	g := NewGenerator("Test API", "1.0.0")
	g.AddPath("GET", "/users", Operation{Summary: "Get users"})

	handler := g.Handler()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %s", rec.Header().Get("Content-Type"))
	}

	var spec Spec
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Errorf("json.Unmarshal() error = %v", err)
	}
	if spec.Info.Title != "Test API" {
		t.Errorf("expected title 'Test API', got %s", spec.Info.Title)
	}
}

func TestSpec(t *testing.T) {
	g := NewGenerator("Test API", "1.0.0")
	spec := g.Spec()

	if spec == nil {
		t.Fatal("expected spec to be non-nil")
	}
	if spec.Paths == nil {
		t.Error("expected Paths to be initialized")
	}
}
