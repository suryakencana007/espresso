// Package openapi provides OpenAPI 3.0 specification generation for Espresso.
package openapi

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/suryakencana007/espresso/v2/internal/errorenvelope"
)

// Spec represents the OpenAPI 3.0 specification.
type Spec struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Servers    []Server            `json:"servers,omitempty"`
	Paths      map[string]PathItem `json:"paths"`
	Components map[string]any      `json:"components,omitempty"`
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
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required"`
	Schema      *Schema `json:"schema,omitempty"`
	Example     any     `json:"example,omitempty"`
}

// RequestBody represents an OpenAPI request body.
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

// MediaType represents an OpenAPI media type.
type MediaType struct {
	Schema  *Schema `json:"schema,omitempty"`
	Example any     `json:"example,omitempty"`
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
	AdditionalProperties any                `json:"additionalProperties,omitempty"`
	Example              any                `json:"example,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
}

// SecurityScheme mirrors the OpenAPI 3.0 Security Scheme Object (minimal subset).
//
// Only the fields needed by the two common schemes are modeled. OAuth2 flows and
// OpenID Connect are intentionally out of scope for v2.3. Use the constructors
// for the documented cases:
//
//	BearerScheme("JWT")             // HTTP bearer / JWT: {type:"http", scheme:"bearer", bearerFormat:"JWT"}
//	APIKeyHeaderScheme("X-API-Key") // apiKey in header:  {type:"apiKey", in:"header", name:"X-API-Key"}
type SecurityScheme struct {
	Type         string `json:"type"`                   // "http", "apiKey", ...
	Scheme       string `json:"scheme,omitempty"`       // "bearer" for http
	BearerFormat string `json:"bearerFormat,omitempty"` // "JWT"
	In           string `json:"in,omitempty"`           // "header" for apiKey
	Name         string `json:"name,omitempty"`         // header name for apiKey
	Description  string `json:"description,omitempty"`
}

// BearerScheme returns an HTTP bearer security scheme. bearerFormat is optional
// (commonly "JWT") and is omitted from the spec when empty.
func BearerScheme(bearerFormat string) SecurityScheme {
	return SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: bearerFormat}
}

// APIKeyHeaderScheme returns an apiKey security scheme carried in a request
// header. headerName is the header the key is read from (e.g. "X-API-Key").
func APIKeyHeaderScheme(headerName string) SecurityScheme {
	return SecurityScheme{Type: "apiKey", In: "header", Name: headerName}
}

// Generator generates OpenAPI specs from routes.
//
// The marshaled spec is cached: Handler serves the bytes produced by the first
// successful marshal and reuses them on every subsequent request, since the spec
// is immutable once route registration completes. Any mutation method
// (Server/AddServer, AddPath, AddSchema, Schema, AddSecurityScheme, Description/
// SetDescription) invalidates the cache so a later marshal reflects the change —
// the cache is never stale. Reads and the mutation/invalidation path are guarded
// by mu, so Handler is safe to serve concurrently while registration is still in
// flight (verified under -race).
type Generator struct {
	spec *Spec

	// mu guards specBytes/specErr and the invalidation flag below. The cached
	// bytes are computed lazily on the first marshal and dropped on any mutation.
	mu         sync.Mutex
	specBytes  []byte
	specErr    error
	specCached bool
}

// invalidateCache drops any cached marshaled spec. It must be called by every
// method that mutates g.spec so a subsequent marshal does not serve stale bytes.
func (g *Generator) invalidateCache() {
	g.mu.Lock()
	g.specBytes = nil
	g.specErr = nil
	g.specCached = false
	g.mu.Unlock()
}

// cachedJSON returns the marshaled spec, computing and caching it on first call
// and serving the cached slice (and any marshal error) thereafter. It is safe
// for concurrent use. The cache is dropped by invalidateCache on mutation.
func (g *Generator) cachedJSON() ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.specCached {
		g.specBytes, g.specErr = g.ToJSON()
		g.specCached = true
	}
	return g.specBytes, g.specErr
}

// New creates a new OpenAPI generator with the given title and version.
// This is the primary entry point for OpenAPI spec generation.
//
// Example:
//
//	gen := openapi.New("My API", "1.0.0").
//	    Description("REST API for user management").
//	    Server("http://localhost:8080", "Development")
func New(title, version string) *Generator {
	return &Generator{
		spec: &Spec{
			OpenAPI: "3.0.3",
			Info: Info{
				Title:   title,
				Version: version,
			},
			Paths: make(map[string]PathItem),
			Components: map[string]any{
				"schemas":         make(map[string]*Schema),
				"securitySchemes": make(map[string]SecurityScheme),
			},
		},
	}
}

// NewGenerator creates a new OpenAPI generator.
//
// Deprecated: Use New() instead.
func NewGenerator(title, version string) *Generator {
	return New(title, version)
}

// Description sets the API description.
func (g *Generator) Description(desc string) *Generator {
	g.spec.Info.Description = desc
	g.invalidateCache()
	return g
}

// SetDescription sets the API description.
//
// Deprecated: Use Description() instead.
func (g *Generator) SetDescription(desc string) *Generator {
	return g.Description(desc)
}

// Server adds a server to the spec.
func (g *Generator) Server(url, description string) *Generator {
	g.spec.Servers = append(g.spec.Servers, Server{
		URL:         url,
		Description: description,
	})
	g.invalidateCache()
	return g
}

// AddServer adds a server to the spec.
//
// Deprecated: Use Server() instead.
func (g *Generator) AddServer(url, description string) *Generator {
	return g.Server(url, description)
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
	g.invalidateCache()
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
	g.invalidateCache()
	return g
}

// AddSecurityScheme registers a named security scheme under
// components.securitySchemes. Operations reference it by name via the
// Security("name") option; registering the scheme here is what turns that
// reference into a resolvable, strict-validation-clean spec instead of a
// dangling reference.
//
// Use the constructors for the two documented cases:
//
//	gen.AddSecurityScheme("bearerAuth", openapi.BearerScheme("JWT"))
//	gen.AddSecurityScheme("apiKeyAuth", openapi.APIKeyHeaderScheme("X-API-Key"))
//
// OAuth2 flows and OpenID Connect are out of scope for v2.3.
func (g *Generator) AddSecurityScheme(name string, scheme SecurityScheme) *Generator {
	schemes, ok := g.spec.Components["securitySchemes"].(map[string]SecurityScheme)
	if !ok {
		schemes = make(map[string]SecurityScheme)
		g.spec.Components["securitySchemes"] = schemes
	}
	schemes[name] = scheme
	g.invalidateCache()
	return g
}

// UnresolvedSecurityRefs returns, sorted and de-duplicated, every scheme name
// referenced by any operation's Security requirement that has no matching entry
// in components.securitySchemes. An empty result means every reference resolves.
//
// It surfaces dangling references explicitly rather than emitting them silently,
// so callers (and the verification matrix) can flag a Security("name") that names
// a scheme nobody registered.
func (g *Generator) UnresolvedSecurityRefs() []string {
	defined, _ := g.spec.Components["securitySchemes"].(map[string]SecurityScheme)

	seen := make(map[string]struct{})
	var missing []string
	for _, item := range g.spec.Paths {
		for _, op := range []*Operation{item.Get, item.Post, item.Put, item.Delete, item.Patch, item.Options, item.Head} {
			if op == nil {
				continue
			}
			for _, req := range op.Security {
				for name := range req {
					if _, ok := defined[name]; ok {
						continue
					}
					if _, dup := seen[name]; dup {
						continue
					}
					seen[name] = struct{}{}
					missing = append(missing, name)
				}
			}
		}
	}
	sort.Strings(missing)
	return missing
}

// Schema generates a schema from type and adds it to components.
// Returns the generator for chaining.
//
// Example:
//
//	gen.Schema("User", reflect.TypeOf(User{}))
func (g *Generator) Schema(name string, t reflect.Type) *Generator {
	return g.AddSchema(name, GenerateSchemaFromType(t))
}

// JSON returns the spec as JSON.
func (g *Generator) JSON() ([]byte, error) {
	return g.ToJSON()
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
//
// The spec is marshaled once (lazily, on the first request) and served from the
// cache thereafter; a mutation to the generator invalidates the cache so the
// next request re-marshals. On a marshal failure the handler emits the canonical
// {"error":{...}} envelope with Content-Type application/json (via the
// stdlib-only internal/errorenvelope leaf), not a text/plain http.Error body, so
// it matches every other framework failure path.
func (g *Generator) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data, err := g.cachedJSON()
		if err != nil {
			errorenvelope.Write(w, http.StatusInternalServerError, errorenvelope.Body{
				Code:    "INTERNAL",
				Message: "failed to generate OpenAPI specification",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})
}
