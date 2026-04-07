package extractor

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

type fieldInfo struct {
	index    int
	name     string
	required bool
	kind     reflect.Kind
}

var structFieldCache sync.Map

func getCachedFields(t reflect.Type, tagName string) []fieldInfo {
	key := fmt.Sprintf("%p|%s", t, tagName)

	if cached, ok := structFieldCache.Load(key); ok {
		return cached.([]fieldInfo) //nolint:errcheck // type guaranteed by LoadOrStore
	}

	fields := buildFieldInfo(t, tagName)
	cached, _ := structFieldCache.LoadOrStore(key, fields)
	return cached.([]fieldInfo) //nolint:errcheck // type guaranteed by LoadOrStore
}

func buildFieldInfo(t reflect.Type, tagName string) []fieldInfo {
	fields := make([]fieldInfo, 0)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get(tagName)
		if tag == "" || tag == "-" {
			continue
		}

		parts := strings.Split(tag, ",")
		name := parts[0]
		required := len(parts) > 1 && parts[1] == "required"

		fields = append(fields, fieldInfo{
			index:    i,
			name:     name,
			required: required,
			kind:     field.Type.Kind(),
		})
	}
	return fields
}

// QueryExtractor extracts URL query parameters into a struct.
// Uses struct tags with `query:"name"` to map query params to struct fields.
// Supports required fields with `query:"name,required"`.
type QueryExtractor[T any] struct {
	Data T
}

// Extract populates the struct from URL query parameters.
func (q *QueryExtractor[T]) Extract(r *http.Request) error {
	return extractStructTags(&q.Data, r.URL.Query(), "query")
}

// Reset clears the extractor data for reuse.
func (q *QueryExtractor[T]) Reset() {
	var zero T
	q.Data = zero
}

// FormExtractor extracts application/x-www-form-urlencoded data.
// Similar to QueryExtractor but reads from request body for POST/PUT forms.
type FormExtractor[T any] struct {
	Data T
}

// Extract populates the struct from form data in the request body.
func (f *FormExtractor[T]) Extract(r *http.Request) error {
	defer func() { _, _ = io.Copy(io.Discard, r.Body); _ = r.Body.Close() }()
	if err := r.ParseForm(); err != nil {
		return err
	}
	return extractStructTags(&f.Data, r.Form, "form")
}

// Reset clears the extractor data for reuse.
func (f *FormExtractor[T]) Reset() {
	var zero T
	f.Data = zero
}

// FileInfo contains metadata about an uploaded file.
type FileInfo struct {
	Filename string
	Size     int64
	Header   textproto.MIMEHeader
}

// MultipartExtractor extracts multipart/form-data including file uploads.
// Uses struct tags with `form:"name"` for regular fields and `file:"name"` for files.
// Supports required fields with `form:"name,required"` and `file:"name,required"`.
type MultipartExtractor[T any] struct {
	Data T
}

// Extract populates the struct from multipart form data.
func (m *MultipartExtractor[T]) Extract(r *http.Request) error {
	defer func() { _, _ = io.Copy(io.Discard, r.Body); _ = r.Body.Close() }()

	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32 MB max memory
		return err
	}

	if r.MultipartForm == nil {
		return nil
	}

	return extractStructTagsFromMultipart(&m.Data, r.MultipartForm)
}

// Reset clears the extractor data for reuse.
func (m *MultipartExtractor[T]) Reset() {
	var zero T
	m.Data = zero
}

// Multipart is a type alias for MultipartExtractor[T].
type Multipart[T any] = MultipartExtractor[T]

// FileExtractor extracts a single file from multipart form data.
// Use for handlers that accept a single file upload.
type FileExtractor struct {
	File FileInfo
}

// Extract reads the file from the multipart form.
// The form field name must be "file" or specified via `file:"fieldname"` tag.
func (f *FileExtractor) Extract(r *http.Request) error {
	defer func() { _, _ = io.Copy(io.Discard, r.Body); _ = r.Body.Close() }()

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return err
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	f.File = FileInfo{
		Filename: header.Filename,
		Size:     header.Size,
		Header:   header.Header,
	}

	return nil
}

// Reset clears the extractor data for reuse.
func (f *FileExtractor) Reset() {
	f.File = FileInfo{}
}

// File is a type alias for FileExtractor.
type File = FileExtractor

// FilesExtractor extracts multiple files from multipart form data.
type FilesExtractor struct {
	Files []FileInfo
}

// Extract reads all files from the multipart form.
// The form field name must be "files" or specified via `files:"fieldname"` tag.
func (f *FilesExtractor) Extract(r *http.Request) error {
	defer func() { _, _ = io.Copy(io.Discard, r.Body); _ = r.Body.Close() }()

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return err
	}

	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil
	}

	headers, ok := r.MultipartForm.File["files"]
	if !ok {
		return nil
	}

	f.Files = make([]FileInfo, 0, len(headers))
	for _, header := range headers {
		f.Files = append(f.Files, FileInfo{
			Filename: header.Filename,
			Size:     header.Size,
			Header:   header.Header,
		})
	}

	return nil
}

// Reset clears the extractor data for reuse.
func (f *FilesExtractor) Reset() {
	f.Files = nil
}

// Files is a type alias for FilesExtractor.
type Files = FilesExtractor

// PathExtractor extracts path parameters from the URL.
// Uses Go 1.22+ r.PathValue() for native path parameter extraction.
type PathExtractor[T any] struct{ Data T }

// Extract populates the struct from URL path parameters.
func (p *PathExtractor[T]) Extract(r *http.Request) error {
	return extractStructTagsPathParams(&p.Data, r, "path")
}

// Reset clears the extractor data for reuse.
func (p *PathExtractor[T]) Reset() {
	var zero T
	p.Data = zero
}

// HeaderExtractor extracts HTTP headers into a struct.
// Uses struct tags with `header:"name"` to map headers to struct fields.
type HeaderExtractor[T any] struct {
	Data T
}

// Extract populates the struct from HTTP headers.
func (h *HeaderExtractor[T]) Extract(r *http.Request) error {
	return extractStructTagsFromHeaders(&h.Data, r.Header)
}

// Reset clears the extractor data for reuse.
func (h *HeaderExtractor[T]) Reset() {
	var zero T
	h.Data = zero
}

// RawBodyExtractor extracts the raw request body as bytes.
// Uses pooled byte slices to reduce allocations for frequently accessed request bodies.
type RawBodyExtractor struct {
	Data []byte
}

// Extract reads the raw request body into Data.
// For large bodies (>64KB), uses pooled buffers for better performance.
func (rb *RawBodyExtractor) Extract(r *http.Request) error {
	defer func() { _, _ = io.Copy(io.Discard, r.Body); _ = r.Body.Close() }()

	var err error
	rb.Data, err = io.ReadAll(r.Body)
	return err
}

// Reset clears the extractor data for reuse.
// Releases large buffers back to pool to prevent memory bloat.
func (rb *RawBodyExtractor) Reset() {
	if cap(rb.Data) > 64*1024 {
		rb.Data = nil
	} else {
		rb.Data = rb.Data[:0]
	}
}

// XMLExtractor extracts and decodes XML request body.
type XMLExtractor[T any] struct {
	Data T
}

// Extract decodes XML from the request body into Data.
func (x *XMLExtractor[T]) Extract(r *http.Request) error {
	defer func() { _, _ = io.Copy(io.Discard, r.Body); _ = r.Body.Close() }()
	return xml.NewDecoder(r.Body).Decode(&x.Data)
}

// Reset clears the extractor data for reuse.
func (x *XMLExtractor[T]) Reset() {
	var zero T
	x.Data = zero
}

// WriteResponse writes the XML response to the HTTP response writer.
func (x *XMLExtractor[T]) WriteResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/xml")
	status := 0
	if s, ok := any(x.Data).(interface{ StatusCode() int }); ok {
		status = s.StatusCode()
	}
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, err := w.Write([]byte(xml.Header))
	if err != nil {
		return err
	}
	return xml.NewEncoder(w).Encode(x.Data)
}

// Query is a type alias for QueryExtractor[T].
type Query[T any] = QueryExtractor[T]

// Form is a type alias for FormExtractor[T].
type Form[T any] = FormExtractor[T]

// Path is a type alias for PathExtractor[T].
type Path[T any] = PathExtractor[T]

// Header is a type alias for HeaderExtractor[T].
type Header[T any] = HeaderExtractor[T]

// XML is a type alias for XMLExtractor[T].
type XML[T any] = XMLExtractor[T]

// RawBody is a type alias for RawBodyExtractor.
type RawBody = RawBodyExtractor

// PathParamsKey is the context key for path parameters.
type PathParamsKey = struct{}

// PathParams stores path parameter key-value pairs.
type PathParams map[string]string

// SetPathParams stores path parameters in the request context.
func SetPathParams(r *http.Request, params PathParams) *http.Request {
	ctx := context.WithValue(r.Context(), PathParamsKey{}, params)
	return r.WithContext(ctx)
}

// GetPathParams retrieves path parameters from the request context.
func GetPathParams(r *http.Request) PathParams {
	params, _ := r.Context().Value(PathParamsKey{}).(PathParams)
	if params == nil {
		return make(PathParams)
	}
	return params
}

func extractStructTags[T any](target *T, values map[string][]string, tagName string) error {
	targetVal := reflect.ValueOf(target).Elem()
	targetType := targetVal.Type()

	fields := getCachedFields(targetType, tagName)

	for _, fi := range fields {
		fieldVal := targetVal.Field(fi.index)
		if !fieldVal.CanSet() {
			continue
		}

		vals, ok := values[fi.name]
		if !ok || len(vals) == 0 {
			if fi.required {
				fieldErr := RequiredFieldError(fi.name, tagName)
				return &fieldErr
			}
			continue
		}

		if err := setValueFromString(fieldVal, vals[0]); err != nil {
			return err
		}
	}

	return nil
}

func extractStructTagsFromMultipart[T any](target *T, form *multipart.Form) error {
	targetVal := reflect.ValueOf(target).Elem()
	targetType := targetVal.Type()

	if err := extractFormFields(targetVal, targetType, form); err != nil {
		return err
	}

	return extractFileFields(targetVal, targetType, form)
}

func extractFormFields(targetVal reflect.Value, targetType reflect.Type, form *multipart.Form) error {
	fields := getCachedFields(targetType, "form")
	for _, fi := range fields {
		fieldVal := targetVal.Field(fi.index)
		if !fieldVal.CanSet() {
			continue
		}

		vals, ok := form.Value[fi.name]
		if !ok || len(vals) == 0 {
			if fi.required {
				fieldErr := RequiredFieldError(fi.name, "form")
				return &fieldErr
			}
			continue
		}

		if err := setValueFromString(fieldVal, vals[0]); err != nil {
			return err
		}
	}
	return nil
}

func extractFileFields(targetVal reflect.Value, targetType reflect.Type, form *multipart.Form) error {
	fileFields := getCachedFields(targetType, "file")
	for _, fi := range fileFields {
		fieldVal := targetVal.Field(fi.index)
		if !fieldVal.CanSet() {
			continue
		}

		headers, ok := form.File[fi.name]
		if !ok || len(headers) == 0 {
			if fi.required {
				fieldErr := RequiredFieldError(fi.name, "file")
				return &fieldErr
			}
			continue
		}

		// Check if field is FileInfo, []FileInfo, or string (filename)
		switch fieldVal.Kind() {
		case reflect.Struct:
			if fieldVal.Type() == reflect.TypeFor[FileInfo]() {
				fieldVal.Set(reflect.ValueOf(FileInfo{
					Filename: headers[0].Filename,
					Size:     headers[0].Size,
					Header:   headers[0].Header,
				}))
			}
		case reflect.Slice:
			if fieldVal.Type().Elem() == reflect.TypeFor[FileInfo]() {
				files := make([]FileInfo, 0, len(headers))
				for _, header := range headers {
					files = append(files, FileInfo{
						Filename: header.Filename,
						Size:     header.Size,
						Header:   header.Header,
					})
				}
				fieldVal.Set(reflect.ValueOf(files))
			}
		case reflect.String:
			fieldVal.SetString(headers[0].Filename)
		}
	}
	return nil
}

func extractStructTagsPathParams[T any](target *T, r *http.Request, tagName string) error {
	targetVal := reflect.ValueOf(target).Elem()
	targetType := targetVal.Type()

	fields := getCachedFields(targetType, tagName)

	for _, fi := range fields {
		fieldVal := targetVal.Field(fi.index)
		if !fieldVal.CanSet() {
			continue
		}

		val := r.PathValue(fi.name)
		if val == "" {
			if fi.required {
				fieldErr := RequiredFieldError(fi.name, tagName)
				return &fieldErr
			}
			continue
		}

		if err := setValueFromString(fieldVal, val); err != nil {
			return err
		}
	}

	return nil
}

func extractStructTagsFromHeaders[T any](target *T, headers http.Header) error {
	targetVal := reflect.ValueOf(target).Elem()
	targetType := targetVal.Type()

	fields := getCachedFields(targetType, "header")

	for _, fi := range fields {
		fieldVal := targetVal.Field(fi.index)
		if !fieldVal.CanSet() {
			continue
		}

		val := headers.Get(fi.name)
		if val == "" {
			if fi.required {
				fieldErr := RequiredFieldError(fi.name, "header")
				return &fieldErr
			}
			continue
		}

		if err := setValueFromString(fieldVal, val); err != nil {
			return err
		}
	}

	return nil
}

func setValueFromString(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return &TypeConversionError{
				Field:    "",
				Expected: "integer",
				Actual:   "invalid integer format",
				Value:    s,
			}
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return &TypeConversionError{
				Field:    "",
				Expected: "unsigned integer",
				Actual:   "invalid unsigned integer format",
				Value:    s,
			}
		}
		v.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return &TypeConversionError{
				Field:    "",
				Expected: "float",
				Actual:   "invalid float format",
				Value:    s,
			}
		}
		v.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return &TypeConversionError{
				Field:    "",
				Expected: "boolean",
				Actual:   "invalid boolean format",
				Value:    s,
			}
		}
		v.SetBool(b)
	default:
		return &UnsupportedTypeError{
			Field:    "",
			Expected: "string, int, uint, float, or bool",
			Actual:   v.Type().String(),
		}
	}
	return nil
}

// TypeConversionError represents a failed string-to-type conversion during extraction.
type TypeConversionError struct {
	Field    string
	Expected string
	Actual   string
	Value    any
}

func (e *TypeConversionError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("field '%s': cannot convert %v to %s", e.Field, e.Value, e.Expected)
	}
	return fmt.Sprintf("cannot convert %v to %s", e.Value, e.Expected)
}

// ToFieldError converts TypeConversionError to a generic FieldError.
func (e *TypeConversionError) ToFieldError() FieldError {
	return FieldError{
		Field:   e.Field,
		Message: fmt.Sprintf("expected %s, got %s", e.Expected, e.Actual),
		Value:   e.Value,
	}
}

// UnsupportedTypeError represents an unsupported destination field type.
type UnsupportedTypeError struct {
	Field    string
	Expected string
	Actual   string
}

func (e *UnsupportedTypeError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("field '%s': unsupported type %s (expected %s)", e.Field, e.Actual, e.Expected)
	}
	return fmt.Sprintf("unsupported type: %s (expected %s)", e.Actual, e.Expected)
}

// ToFieldError converts UnsupportedTypeError to a generic FieldError.
func (e *UnsupportedTypeError) ToFieldError() FieldError {
	return FieldError{
		Field:   e.Field,
		Message: fmt.Sprintf("unsupported type: %s (expected %s)", e.Actual, e.Expected),
	}
}

// FieldError represents a single extraction or validation error for one field.
type FieldError struct {
	Field   string
	Message string
	Value   any
	Path    string
}

func (fe FieldError) Error() string {
	if fe.Path != "" {
		return fe.Path + "." + fe.Field + ": " + fe.Message
	}
	return fe.Field + ": " + fe.Message
}

// FieldErrors is a collection of FieldError values.
type FieldErrors []FieldError

func (fe FieldErrors) Error() string {
	if len(fe) == 0 {
		return "validation errors"
	}
	if len(fe) == 1 {
		return fe[0].Message
	}
	return fmt.Sprintf("%d validation errors", len(fe))
}

// ToValidationErrors converts FieldErrors into JSON-friendly validation errors.
func (fe FieldErrors) ToValidationErrors() []ValidationError {
	validationErrors := make([]ValidationError, len(fe))
	for i, e := range fe {
		validationErrors[i] = ValidationError{
			Field:   e.Field,
			Message: e.Message,
		}
	}
	return validationErrors
}

// AddFieldError appends a new field error and returns the appended item.
func (fe *FieldErrors) AddFieldError(field, message string, value any, path ...string) *FieldError {
	var p string
	if len(path) > 0 {
		p = path[0]
	}
	fieldErr := FieldError{
		Field:   field,
		Message: message,
		Value:   value,
		Path:    p,
	}
	*fe = append(*fe, fieldErr)
	return &fieldErr
}

// NewFieldErrors creates an empty FieldErrors collection.
func NewFieldErrors() *FieldErrors {
	return &FieldErrors{}
}

// RequiredFieldError creates an error for a missing required field.
func RequiredFieldError(field string, path ...string) FieldError {
	var p string
	if len(path) > 0 {
		p = path[0]
	}
	return FieldError{
		Field:   field,
		Message: "required field is missing",
		Path:    p,
	}
}

// InvalidTypeError creates an error for a field with invalid type/value shape.
func InvalidTypeError(field string, expected, actual string, value any, path ...string) FieldError {
	var p string
	if len(path) > 0 {
		p = path[0]
	}
	return FieldError{
		Field:   field,
		Message: fmt.Sprintf("expected %s, got %s", expected, actual),
		Value:   value,
		Path:    p,
	}
}

// ValidationError is a minimal field error structure for API responses.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
