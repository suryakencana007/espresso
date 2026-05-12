// Package validator provides struct-tag-driven validation using the `validate`
// tag. It returns espresso.FieldErrors on failure so that validation results
// compose with the rest of the framework's structured error pipeline.
//
// Example:
//
//	type CreateUserReq struct {
//	    Name  string `json:"name"  validate:"required,min=3,max=50"`
//	    Email string `json:"email" validate:"required,email"`
//	    Age   int    `json:"age"   validate:"min=0,max=150"`
//	    Role  string `json:"role"  validate:"oneof=admin user guest"`
//	}
//
//	func handler(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[Res], error) {
//	    if err := validator.Struct(req.Data); err != nil {
//	        return espresso.JSON[Res]{}, espresso.ValidationErrors(err.(espresso.FieldErrors).ToValidationErrors())
//	    }
//	    // ... business logic
//	}
package validator

import (
	"fmt"
	"net/mail"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/suryakencana007/espresso/v2"
)

// AsDefaultValidator returns a function suitable for
// espresso.SetDefaultValidator that runs Struct(v) and converts the
// FieldErrors result into the framework's standard ValidationError shape.
//
// Wire it once in init():
//
//	func init() {
//	    espresso.SetDefaultValidator(validator.AsDefaultValidator())
//	}
//
// Users who need a custom error mapper (different code, extra context,
// etc.) should write their own closure instead — this helper is the
// most-common-case shortcut, not a configuration surface.
func AsDefaultValidator() func(v any) error {
	return func(v any) error {
		if err := Struct(v); err != nil {
			if fe, ok := err.(espresso.FieldErrors); ok {
				return espresso.ValidationErrors(fe.ToValidationErrors())
			}
			return err
		}
		return nil
	}
}

// Struct validates v against its `validate` struct tags. It returns
// espresso.FieldErrors on failure (implements error) or nil when every
// field passes.
//
// v may be a struct or a pointer to a struct; nil pointers return nil.
// Non-struct inputs return a plain error.
//
// Struct fields that themselves hold structs (or pointers to structs,
// or slices/arrays of structs) are walked recursively so nested types
// can carry their own tags.
func Struct(v any) error {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("validator: expected struct, got %s", rv.Kind())
	}

	var errs espresso.FieldErrors
	walkStruct(rv, "", &errs)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func walkStruct(rv reflect.Value, path string, errs *espresso.FieldErrors) {
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldVal := rv.Field(i)
		name := jsonName(field)

		if tag := field.Tag.Get("validate"); tag != "" {
			for _, rule := range parseRules(tag) {
				if msg, ok := applyRule(fieldVal, rule); !ok {
					_ = errs.AddFieldError(name, msg, safeInterface(fieldVal), path)
				}
			}
		}

		// Recurse into nested structures
		inner := fieldVal
		if inner.Kind() == reflect.Pointer {
			if inner.IsNil() {
				continue
			}
			inner = inner.Elem()
		}

		nextPath := name
		if path != "" {
			nextPath = path + "." + name
		}

		switch inner.Kind() {
		case reflect.Struct:
			walkStruct(inner, nextPath, errs)
		case reflect.Slice, reflect.Array:
			for j := 0; j < inner.Len(); j++ {
				elem := inner.Index(j)
				if elem.Kind() == reflect.Pointer {
					if elem.IsNil() {
						continue
					}
					elem = elem.Elem()
				}
				if elem.Kind() == reflect.Struct {
					walkStruct(elem, fmt.Sprintf("%s[%d]", nextPath, j), errs)
				}
			}
		}
	}
}

// rule represents a single parsed validation rule.
type rule struct {
	name  string
	param string
}

// parseRules splits "required,min=3,max=20" into rule{name,param} entries.
// Commas inside an `oneof=` parameter (space-separated list) are not expected;
// if a rule needs commas in its value, users should use regex or a custom
// encoding and split on space instead.
func parseRules(tag string) []rule {
	parts := strings.Split(tag, ",")
	rules := make([]rule, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if eq := strings.IndexByte(p, '='); eq >= 0 {
			rules = append(rules, rule{name: p[:eq], param: p[eq+1:]})
		} else {
			rules = append(rules, rule{name: p})
		}
	}
	return rules
}

// ruleFunc executes a single rule against a value.
type ruleFunc func(v reflect.Value, param string) (string, bool)

var builtinRules = map[string]ruleFunc{
	"required": ruleRequired,
	"min":      ruleMin,
	"max":      ruleMax,
	"email":    ruleEmail,
	"url":      ruleURL,
	"regex":    ruleRegex,
	"oneof":    ruleOneOf,
}

// applyRule dispatches to the appropriate builtin rule.
func applyRule(v reflect.Value, r rule) (string, bool) {
	fn, ok := builtinRules[r.name]
	if !ok {
		return "unknown rule: " + r.name, false
	}
	return fn(v, r.param)
}

func ruleRequired(v reflect.Value, _ string) (string, bool) {
	if isZero(v) {
		return "is required", false
	}
	return "", true
}

func ruleMin(v reflect.Value, param string) (string, bool) {
	return compareBound(v, param, true)
}

func ruleMax(v reflect.Value, param string) (string, bool) {
	return compareBound(v, param, false)
}

func ruleEmail(v reflect.Value, _ string) (string, bool) {
	if v.Kind() != reflect.String {
		return "email rule requires string field", false
	}
	s := v.String()
	if s == "" {
		return "", true
	}
	if _, err := mail.ParseAddress(s); err != nil {
		return "must be a valid email address", false
	}
	return "", true
}

func ruleURL(v reflect.Value, _ string) (string, bool) {
	if v.Kind() != reflect.String {
		return "url rule requires string field", false
	}
	s := v.String()
	if s == "" {
		return "", true
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "must be a valid URL", false
	}
	return "", true
}

func ruleRegex(v reflect.Value, param string) (string, bool) {
	if v.Kind() != reflect.String {
		return "regex rule requires string field", false
	}
	re, err := regexCached(param)
	if err != nil {
		return "invalid regex pattern: " + err.Error(), false
	}
	if v.String() != "" && !re.MatchString(v.String()) {
		return "must match pattern " + param, false
	}
	return "", true
}

func ruleOneOf(v reflect.Value, param string) (string, bool) {
	if v.Kind() != reflect.String {
		return "oneof rule requires string field", false
	}
	s := v.String()
	if s == "" {
		return "", true
	}
	options := strings.Fields(param)
	for _, opt := range options {
		if s == opt {
			return "", true
		}
	}
	return "must be one of: " + strings.Join(options, ", "), false
}

// compareBound parses param and dispatches to the right bound check.
func compareBound(v reflect.Value, param string, isMin bool) (string, bool) {
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		label := "max"
		if isMin {
			label = "min"
		}
		return "invalid " + label + " param: " + param, false
	}

	switch v.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		return checkLengthBound(float64(v.Len()), n, param, isMin)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return checkValueBound(float64(v.Int()), n, param, isMin)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return checkValueBound(float64(v.Uint()), n, param, isMin)
	case reflect.Float32, reflect.Float64:
		return checkValueBound(v.Float(), n, param, isMin)
	default:
		return "min/max not supported for kind " + v.Kind().String(), false
	}
}

func checkLengthBound(length, bound float64, param string, isMin bool) (string, bool) {
	if isMin && length < bound {
		return fmt.Sprintf("length must be at least %s", param), false
	}
	if !isMin && length > bound {
		return fmt.Sprintf("length must be at most %s", param), false
	}
	return "", true
}

func checkValueBound(val, bound float64, param string, isMin bool) (string, bool) {
	if isMin && val < bound {
		return fmt.Sprintf("must be at least %s", param), false
	}
	if !isMin && val > bound {
		return fmt.Sprintf("must be at most %s", param), false
	}
	return "", true
}

func isZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	case reflect.Array:
		return v.Len() == 0
	default:
		return v.IsZero()
	}
}

// safeInterface returns fieldVal.Interface() when the value is addressable
// or exported, else nil. This guards against reflect panics on unexported
// fields even though walkStruct already skips them.
func safeInterface(v reflect.Value) any {
	if !v.IsValid() || !v.CanInterface() {
		return nil
	}
	return v.Interface()
}

// jsonName returns the field name to report in errors. It prefers the `json`
// tag (first comma-separated element) so errors line up with the wire format;
// falls back to the Go field name otherwise.
func jsonName(f reflect.StructField) string {
	if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			return tag[:idx]
		}
		return tag
	}
	return f.Name
}

var regexCache sync.Map

func regexCached(pattern string) (*regexp.Regexp, error) {
	if v, ok := regexCache.Load(pattern); ok {
		if re, ok := v.(*regexp.Regexp); ok {
			return re, nil
		}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	actual, _ := regexCache.LoadOrStore(pattern, re)
	if cached, ok := actual.(*regexp.Regexp); ok {
		return cached, nil
	}
	return re, nil
}
