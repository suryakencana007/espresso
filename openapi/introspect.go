// Package openapi provides OpenAPI 3.0 specification generation for Espresso.
package openapi

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/suryakencana007/espresso/v2/extractor"
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
	fromRequestIf = reflect.TypeFor[interface{ Extract(*http.Request) error }]()
)

// rootPkgPath is the import path of the root espresso package. The JSON[T] and
// State[T] extractors live there, so the openapi package cannot reference them
// concretely (it would form an import cycle: espresso imports openapi). They are
// matched by package path + base name instead. The extractor-package types below
// are referenced concretely so a rename surfaces as a compile error.
const rootPkgPath = "github.com/suryakencana007/espresso/v2"

// extractorKey identifies a generic extractor by its package path and base name
// (the name with the type-parameter instantiation stripped), so PathExtractor[Any]
// and PathExtractor[Other] map to the same kind.
type extractorKey struct {
	pkgPath  string
	baseName string
}

// extractorKindByType classifies extractors off their actual base types rather
// than a type-name prefix. Referencing the concrete extractor types means a
// rename becomes a compile error instead of a silent mis-classification, and
// FileExtractor vs FilesExtractor are distinct base types (no prefix ambiguity).
var extractorKindByType = func() map[extractorKey]ExtractorKind {
	m := map[extractorKey]ExtractorKind{
		keyOf(reflect.TypeFor[extractor.PathExtractor[struct{}]]()):      KindPath,
		keyOf(reflect.TypeFor[extractor.QueryExtractor[struct{}]]()):     KindQuery,
		keyOf(reflect.TypeFor[extractor.FormExtractor[struct{}]]()):      KindForm,
		keyOf(reflect.TypeFor[extractor.MultipartExtractor[struct{}]]()): KindMultipart,
		keyOf(reflect.TypeFor[extractor.HeaderExtractor[struct{}]]()):    KindHeader,
		keyOf(reflect.TypeFor[extractor.CookieExtractor[struct{}]]()):    KindCookie,
		keyOf(reflect.TypeFor[extractor.FileExtractor]()):                KindFile,
		keyOf(reflect.TypeFor[extractor.FilesExtractor]()):               KindFiles,
		// Root-package extractors (cannot be referenced concretely — cycle).
		{pkgPath: rootPkgPath, baseName: "JSON"}:  KindJSONBody,
		{pkgPath: rootPkgPath, baseName: "State"}: KindState,
	}
	return m
}()

// keyOf builds an extractorKey from a reflect.Type, stripping the generic
// type-parameter instantiation from the name.
func keyOf(t reflect.Type) extractorKey {
	return extractorKey{pkgPath: t.PkgPath(), baseName: baseName(t.Name())}
}

// baseName strips the bracketed generic instantiation from a reflect type name,
// e.g. "PathExtractor[struct {...}]" -> "PathExtractor".
func baseName(name string) string {
	if i := strings.IndexByte(name, '['); i >= 0 {
		return name[:i]
	}
	return name
}

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

// getExtractorKind determines the extractor kind from a type by matching its
// actual base type, not a type-name prefix.
func getExtractorKind(t reflect.Type) ExtractorKind {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if kind, ok := extractorKindByType[keyOf(t)]; ok {
		return kind
	}

	return KindUnknown
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

// statusCoder lets a response type declare the HTTP status it documents to,
// so a non-200 success status (e.g. 201 Created) is derivable from the type
// alone. The built-in JSON[T]/Text/Status responses carry their status on the
// instance (not the type), so they document as 200 by default; callers that
// need a different success code either implement this interface or pass
// openapi.Status(...) at registration time.
type statusCoder interface {
	OpenAPIStatusCode() int
}

var statusCoderIf = reflect.TypeFor[statusCoder]()

// extractStatusCode derives the documented HTTP status code from a response
// type. A type implementing statusCoder reports its own status; every other
// response type documents the 200 default (their concrete status is an instance
// field, not statically knowable from the type).
func extractStatusCode(t reflect.Type) int {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Implements(statusCoderIf) {
		if v, ok := reflect.Zero(t).Interface().(statusCoder); ok {
			if code := v.OpenAPIStatusCode(); code != 0 {
				return code
			}
		}
	}

	return http.StatusOK
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

// BuildPathOperation builds a complete OpenAPI operation from handler info:
// it applies options, defaults the tag, wires path/query parameters and the
// JSON request body, and attaches the response-body schema under the handler's
// documented status code. It is the single source of truth shared by both
// registration paths (the fluent Get/Post/… chain and RegisterHandler) so they
// can no longer drift — previously only the fluent path attached the response
// schema. gen may be nil, in which case schemas are not added to components.
func BuildPathOperation(gen *Generator, info *HandlerInfo, opts ...OperationOption) *Operation {
	op := BuildOperation(info, opts...)

	if len(op.Tags) == 0 {
		op.Tags = []string{"default"}
	}

	for i, reqType := range info.RequestTypes {
		if i >= len(info.ExtractorKinds) {
			continue
		}

		switch info.ExtractorKinds[i] {
		case KindPath:
			op.Parameters = append(op.Parameters, GeneratePathParams(reqType)...)
		case KindQuery:
			op.Parameters = append(op.Parameters, GenerateQueryParams(reqType)...)
		case KindJSONBody:
			if op.RequestBody == nil {
				op.RequestBody = GenerateRequestBody(reqType, gen)
			}
		}
	}

	attachResponse(gen, op, info)

	return op
}

// attachResponse seeds the success response under the handler's documented
// status code and attaches the response-body schema (if any). When the options
// already declared a 2xx success response (e.g. via openapi.Status("201", …)),
// the schema is attached there rather than forcing a 200.
func attachResponse(gen *Generator, op *Operation, info *HandlerInfo) {
	statusKey := strconv.Itoa(info.StatusCode)
	if k, ok := existingSuccessKey(op.Responses); ok {
		statusKey = k
	}

	resp, ok := op.Responses[statusKey]
	if !ok {
		resp = Response{Description: "Success"}
	}

	if info.ResponseType != nil {
		schema := GenerateSchemaFromType(info.ResponseType)
		if name := info.ResponseType.Name(); name != "" && gen != nil {
			gen.Schema(name, info.ResponseType)
		}
		resp.Content = map[string]MediaType{
			"application/json": {Schema: schema},
		}
	}

	if op.Responses == nil {
		op.Responses = make(map[string]Response)
	}
	op.Responses[statusKey] = resp
}

// existingSuccessKey returns the first 2xx response key already present in the
// response map, so an explicit success status from options wins over the
// reflection default.
func existingSuccessKey(responses map[string]Response) (string, bool) {
	for code := range responses {
		if len(code) == 3 && code[0] == '2' {
			return code, true
		}
	}
	return "", false
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
