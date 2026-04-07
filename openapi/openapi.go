// Package openapi provides OpenAPI 3.0 specification generation for Espresso.
package openapi

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
)

// Spec represents the OpenAPI 3.0 specification.
type Spec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       Info                   `json:"info"`
	Servers    []Server               `json:"servers,omitempty"`
	Paths      map[string]PathItem    `json:"paths"`
	Components map[string]interface{} `json:"components,omitempty"`
}

// Info represents the OpenAPI info section.
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

// Server represents an OpenAPI server.
type Server struct {
	URL         string              `json:"url"`
	Description string              `json:"description,omitempty"`
	Variables   map[string]Variable `json:"variables,omitempty"`
}

// Variable represents a server variable.
type Variable struct {
	Default     string   `json:"default"`
	Enum        []string `json:"enum,omitempty"`
	Description string   `json:"description,omitempty"`
}

// PathItem represents an OpenAPI path item.
type PathItem struct {
	Get     *Operation `json:"get,omitempty"`
	Post    *Operation `json:"post,omitempty"`
	Put     *Operation `json:"put,omitempty"`
	Delete  *Operation `json:"delete,omitempty"`
	Patch   *Operation `json:"patch,omitempty"`
	Options *Operation `json:"options,omitempty"`
	Head    *Operation `json:"head,omitempty"`
}

// Operation represents an OpenAPI operation.
type Operation struct {
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []map[string][]string `json:"security,omitempty"`
}

// Parameter represents an OpenAPI parameter.
type Parameter struct {
	Name        string      `json:"name"`
	In          string      `json:"in"`
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required"`
	Schema      *Schema     `json:"schema,omitempty"`
	Example     interface{} `json:"example,omitempty"`
}

// RequestBody represents an OpenAPI request body.
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

// MediaType represents an OpenAPI media type.
type MediaType struct {
	Schema  *Schema     `json:"schema,omitempty"`
	Example interface{} `json:"example,omitempty"`
}

// Response represents an OpenAPI response.
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// Schema represents an OpenAPI schema.
type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Description          string             `json:"description,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	AdditionalProperties interface{}        `json:"additionalProperties,omitempty"`
	Example              interface{}        `json:"example,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
}

// Generator generates OpenAPI specs from routes.
type Generator struct {
	spec *Spec
}

// NewGenerator creates a new OpenAPI generator.
func NewGenerator(title, version string) *Generator {
	return &Generator{
		spec: &Spec{
			OpenAPI: "3.0.3",
			Info: Info{
				Title:   title,
				Version: version,
			},
			Paths: make(map[string]PathItem),
			Components: map[string]interface{}{
				"schemas":         make(map[string]*Schema),
				"securitySchemes": make(map[string]interface{}),
			},
		},
	}
}

// SetDescription sets the API description.
func (g *Generator) SetDescription(desc string) *Generator {
	g.spec.Info.Description = desc
	return g
}

// AddServer adds a server to the spec.
func (g *Generator) AddServer(url, description string) *Generator {
	g.spec.Servers = append(g.spec.Servers, Server{
		URL:         url,
		Description: description,
	})
	return g
}

// AddPath adds a path to the spec.
func (g *Generator) AddPath(method, path string, op Operation) *Generator {
	pathItem, ok := g.spec.Paths[path]
	if !ok {
		pathItem = PathItem{}
	}

	switch strings.ToUpper(method) {
	case http.MethodGet:
		pathItem.Get = &op
	case http.MethodPost:
		pathItem.Post = &op
	case http.MethodPut:
		pathItem.Put = &op
	case http.MethodDelete:
		pathItem.Delete = &op
	case http.MethodPatch:
		pathItem.Patch = &op
	case http.MethodOptions:
		pathItem.Options = &op
	case http.MethodHead:
		pathItem.Head = &op
	}

	g.spec.Paths[path] = pathItem
	return g
}

// AddSchema adds a schema to components.
func (g *Generator) AddSchema(name string, schema *Schema) *Generator {
	schemas, ok := g.spec.Components["schemas"].(map[string]*Schema)
	if !ok {
		schemas = make(map[string]*Schema)
		g.spec.Components["schemas"] = schemas
	}
	schemas[name] = schema
	return g
}

// ToJSON returns the spec as JSON.
func (g *Generator) ToJSON() ([]byte, error) {
	return json.MarshalIndent(g.spec, "", "  ")
}

// Spec returns the spec.
func (g *Generator) Spec() *Spec {
	return g.spec
}

// GenerateSchemaFromType generates an OpenAPI schema from a Go type.
func GenerateSchemaFromType(t reflect.Type) *Schema {
	if t == nil {
		return &Schema{Type: "object"}
	}

	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &Schema{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number", Format: "double"}
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Slice, reflect.Array:
		items := GenerateSchemaFromType(t.Elem())
		return &Schema{Type: "array", Items: items}
	case reflect.Map:
		return &Schema{
			Type:                 "object",
			AdditionalProperties: GenerateSchemaFromType(t.Elem()),
		}
	case reflect.Struct:
		return generateSchemaFromStruct(t)
	case reflect.Ptr:
		return GenerateSchemaFromType(t.Elem())
	default:
		return &Schema{Type: "object"}
	}
}

func generateSchemaFromStruct(t reflect.Type) *Schema {
	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}

	required := make([]string, 0)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		name := strings.Split(jsonTag, ",")[0]
		if name == "" {
			name = field.Name
		}

		// Check if required
		if !strings.Contains(jsonTag, "omitempty") {
			required = append(required, name)
		}

		// Get description from doc tag
		desc := field.Tag.Get("doc")
		if desc == "" {
			desc = field.Tag.Get("description")
		}

		propSchema := GenerateSchemaFromType(field.Type)
		propSchema.Description = desc

		schema.Properties[name] = propSchema
	}

	if len(required) > 0 {
		schema.Required = required
	}

	return schema
}

// Handler returns an http.Handler that serves the OpenAPI spec.
func (g *Generator) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := g.ToJSON()
		if err != nil {
			http.Error(w, "Failed to generate OpenAPI spec", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})
}
