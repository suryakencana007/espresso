// Package openapi provides OpenAPI 3.0 specification generation for Espresso.
package openapi

import (
	"context"
	"errors"
	"reflect"
	"strings"
)

// HandlerInfo contains extracted type information from a handler function.
type HandlerInfo struct {
	// RequestTypes are the inner types from extractors (T from Path[T], Query[T], etc.)
	RequestTypes []reflect.Type

	// ResponseType is the inner type from response (T from JSON[T])
	ResponseType reflect.Type

	// ExtractorKinds identify the source of each request type
	ExtractorKinds []ExtractorKind

	// ReturnsError indicates if handler returns error
	ReturnsError bool

	// StatusCode from response (if set)
	StatusCode int
}

// ExtractorKind identifies the source of request data.
type ExtractorKind string

// ExtractorKind constants identify the source of request data.
const (
	KindPath      ExtractorKind = "path"      // Path parameters
	KindQuery     ExtractorKind = "query"     // Query string parameters
	KindJSONBody  ExtractorKind = "json_body" // JSON request body
	KindForm      ExtractorKind = "form"      // Form data
	KindMultipart ExtractorKind = "multipart" // Multipart form data
	KindHeader    ExtractorKind = "header"    // HTTP headers
	KindCookie    ExtractorKind = "cookie"    // HTTP cookies
	KindFile      ExtractorKind = "file"      // Single file upload
	KindFiles     ExtractorKind = "files"     // Multiple file uploads
	KindState     ExtractorKind = "state"     // Application state
	KindUnknown   ExtractorKind = "unknown"   // Unknown extractor type
)

var (
	contextType   = reflect.TypeFor[context.Context]()
	errorType     = reflect.TypeFor[error]()
	fromRequestIf = reflect.TypeFor[interface{ Extract(r any) error }]()
)

// IntrospectError is returned when handler introspection fails.
type IntrospectError struct {
	Message string
}

func (e *IntrospectError) Error() string {
	return e.Message
}

// Introspect analyzes a handler function and extracts type information.
// Works with Ristretto, Solo, Doppio, Lungo handlers.
//
// Supported signatures:
//   - func() Res
//   - func(Req) Res
//   - func(Req) (Res, error)
//   - func(context.Context) Res
//   - func(context.Context) (Res, error)
//   - func(context.Context, Req) Res
//   - func(context.Context, Req) (Res, error)
//   - func(context.Context, Req1, Req2) Res
//   - func(context.Context, Req1, Req2) (Res, error)
func Introspect(handler any) (*HandlerInfo, error) {
	if handler == nil {
		return nil, &IntrospectError{Message: "handler is nil"}
	}

	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		return nil, &IntrospectError{Message: "handler must be a function"}
	}

	info := &HandlerInfo{}
	numIn := t.NumIn()
	numOut := t.NumOut()

	// Validate return types
	if numOut == 0 || numOut > 2 {
		return nil, &IntrospectError{Message: "handler must return 1 or 2 values"}
	}

	// Check error return
	if numOut == 2 {
		if !t.Out(1).Implements(errorType) {
			return nil, &IntrospectError{Message: "second return value must be error"}
		}
		info.ReturnsError = true
	}

	// Extract response type
	responseType := t.Out(0)
	info.ResponseType = extractResponseType(responseType)
	info.StatusCode = extractStatusCode(responseType)

	// Extract request types from function parameters
	for i := 0; i < numIn; i++ {
		paramType := t.In(i)

		// Skip context.Context
		if paramType.Implements(contextType) {
			continue
		}

		// Check if it's an extractor
		kind := getExtractorKind(paramType)
		if kind != KindUnknown {
			info.ExtractorKinds = append(info.ExtractorKinds, kind)
			innerType := extractInnerType(paramType)
			if innerType != nil {
				info.RequestTypes = append(info.RequestTypes, innerType)
			}
		} else if paramType.Implements(fromRequestIf) {
			// Generic FromRequest implementation
			info.ExtractorKinds = append(info.ExtractorKinds, KindUnknown)
			info.RequestTypes = append(info.RequestTypes, paramType)
		}
	}

	return info, nil
}

// getExtractorKind determines the extractor kind from a type.
func getExtractorKind(t reflect.Type) ExtractorKind {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	typeName := getTypeName(t)

	extractorKinds := []struct {
		prefixes []string
		kind     ExtractorKind
	}{
		{[]string{"PathExtractor", "Path"}, KindPath},
		{[]string{"QueryExtractor", "Query"}, KindQuery},
		{[]string{"JSONExtractor", "JSON"}, KindJSONBody},
		{[]string{"FormExtractor", "Form"}, KindForm},
		{[]string{"MultipartExtractor", "Multipart"}, KindMultipart},
		{[]string{"HeaderExtractor", "Header"}, KindHeader},
		{[]string{"CookieExtractor", "Cookie"}, KindCookie},
		{[]string{"FileExtractor", "File"}, KindFile},
		{[]string{"FilesExtractor", "Files"}, KindFiles},
		{[]string{"State"}, KindState},
	}

	for _, ek := range extractorKinds {
		for _, prefix := range ek.prefixes {
			if strings.HasPrefix(typeName, prefix) || typeName == prefix {
				return ek.kind
			}
		}
	}

	return KindUnknown
}

// getTypeName returns the type name, handling generic types.
func getTypeName(t reflect.Type) string {
	name := t.Name()
	if name == "" {
		// For unnamed types, use string representation
		name = t.String()
	}
	return name
}

// extractInnerType extracts the inner type T from extractor types.
// Path[T], Query[T], JSON[T], etc. -> T.
func extractInnerType(t reflect.Type) reflect.Type {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check for struct with Data field (most extractors have Data T)
	if t.Kind() == reflect.Struct {
		if field, ok := t.FieldByName("Data"); ok {
			return field.Type
		}
	}

	// For generic types, try to extract type parameter
	// This works for types like JSON[T], Path[T], etc.
	if t.Kind() == reflect.Struct {
		// Find the Data field
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Name == "Data" {
				return field.Type
			}
		}
	}

	return nil
}

// extractResponseType extracts the inner type from response types.
// JSON[T] -> T.
func extractResponseType(t reflect.Type) reflect.Type {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check for JSON[T]
	typeName := t.Name()
	if typeName == "JSON" || strings.HasPrefix(typeName, "JSON") {
		// Find Data field
		if t.Kind() == reflect.Struct {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				if field.Name == "Data" {
					return field.Type
				}
			}
		}
	}

	// For other types (Text, Status, SSE), return nil
	// They don't have a schema body
	return nil
}

// extractStatusCode extracts status code from response types.
// Returns 0 when status code is dynamic/not determined.
//
//nolint:unparam // Always returns 0 for now, will be extended later
func extractStatusCode(t reflect.Type) int {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Name() == "Status" {
		return 0
	}

	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Name == "StatusCode" && field.Type.Kind() == reflect.Int {
				return 0
			}
		}
	}

	return 0
}

// MustIntrospect is like Introspect but panics on error.
func MustIntrospect(handler any) *HandlerInfo {
	info, err := Introspect(handler)
	if err != nil {
		panic(err)
	}
	return info
}

// IsExtractor checks if a type is a known extractor.
func IsExtractor(t reflect.Type) bool {
	return getExtractorKind(t) != KindUnknown
}

// BuildOperation creates an OpenAPI operation from handler info.
func BuildOperation(info *HandlerInfo, opts ...OperationOption) *Operation {
	op := &Operation{
		Responses: make(map[string]Response),
	}

	// Apply custom options
	for _, opt := range opts {
		opt(op)
	}

	return op
}

// GeneratePathParams generates OpenAPI path parameters from a struct type.
func GeneratePathParams(t reflect.Type) []Parameter {
	if t == nil {
		return nil
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	params := make([]Parameter, 0)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get path tag
		pathTag := field.Tag.Get("path")
		if pathTag == "" || pathTag == "-" {
			continue
		}

		// Parse tag (name,pattern)
		parts := strings.Split(pathTag, ",")
		name := parts[0]

		// Get schema type
		schema := GenerateSchemaFromType(field.Type)

		// Get description
		desc := field.Tag.Get("doc")
		if desc == "" {
			desc = field.Tag.Get("description")
		}

		params = append(params, Parameter{
			Name:        name,
			In:          "path",
			Required:    true,
			Description: desc,
			Schema:      schema,
		})
	}

	return params
}

// GenerateQueryParams generates OpenAPI query parameters from a struct type.
func GenerateQueryParams(t reflect.Type) []Parameter {
	if t == nil {
		return nil
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	params := make([]Parameter, 0)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get query tag
		queryTag := field.Tag.Get("query")
		if queryTag == "" || queryTag == "-" {
			continue
		}

		// Parse tag (name,required)
		parts := strings.Split(queryTag, ",")
		name := parts[0]
		required := len(parts) > 1 && parts[1] == "required"

		// Get schema type
		schema := GenerateSchemaFromType(field.Type)

		// Get description
		desc := field.Tag.Get("doc")
		if desc == "" {
			desc = field.Tag.Get("description")
		}

		params = append(params, Parameter{
			Name:        name,
			In:          "query",
			Required:    required,
			Description: desc,
			Schema:      schema,
		})
	}

	return params
}

// GenerateRequestBody generates OpenAPI request body from a type.
func GenerateRequestBody(t reflect.Type, gen *Generator) *RequestBody {
	if t == nil {
		return nil
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Generate schema
	schema := GenerateSchemaFromType(t)

	// Get schema name and add to components
	schemaName := t.Name()
	if schemaName == "" {
		schemaName = "Anonymous"
	}

	if gen != nil {
		gen.AddSchema(schemaName, schema)
	}

	return &RequestBody{
		Required: true,
		Content: map[string]MediaType{
			"application/json": {
				Schema: &Schema{Ref: "#/components/schemas/" + schemaName},
			},
		},
	}
}

// Error variables for introspection failures.
var (
	ErrNotAFunction   = errors.New("handler must be a function")
	ErrInvalidReturns = errors.New("handler must return 1 or 2 values")
	ErrInvalidError   = errors.New("second return value must be error")
	ErrInvalidParams  = errors.New("invalid parameter types")
)
