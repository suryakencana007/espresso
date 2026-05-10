---
title: Validator
description: Struct-tag-driven request validation
---

# Validator

The `validator` package provides struct-tag-driven request validation. It reads `validate:"..."` tags via reflection and returns an `espresso.FieldErrors` on failure — which feeds directly into the framework's structured error pipeline.

Import:

```go
import "github.com/suryakencana007/espresso/v2/validator"
```

::: tip Auto-wiring since v2.0
Wire `validator.Struct` once via `espresso.SetDefaultValidator` and every
built-in extractor will validate its decoded value automatically — no
explicit call from the handler body needed. See
[Auto-Validate on Extract](../guide/validation.md#auto-validate-on-extract-since-v20).
:::

## `Struct`

```go
func Struct(v any) error
```

Validates `v` against its `validate` struct tags. Returns `nil` when every field passes, or an `espresso.FieldErrors` (which implements `error`) when one or more rules fail.

`v` may be a struct or a pointer to a struct. A nil pointer returns `nil`. A non-struct input returns a plain error (programmer error, not a validation failure).

Fields that themselves hold structs — or pointers to structs, or slices/arrays of structs — are walked recursively so nested types can carry their own tags. Error paths are tracked so a failure at `items[1].name` reports `Field: "name"` and `Path: "items[1]"`.

### Example

```go
type CreateUserReq struct {
    Name  string `json:"name"  validate:"required,min=3,max=50"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"min=0,max=150"`
    Role  string `json:"role"  validate:"oneof=admin user guest"`
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[Res], error) {
    if err := validator.Struct(req.Data); err != nil {
        // err is espresso.FieldErrors; ValidationErrors turns it into a
        // structured 400 with code "VALIDATION_ERROR" and a "details"
        // array listing every failed field.
        fe := err.(espresso.FieldErrors)
        return espresso.JSON[Res]{}, espresso.ValidationErrors(fe.ToValidationErrors())
    }
    // business logic
}
```

Input `{"name":"","email":"not-an-email","age":200,"role":"owner"}` produces:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "One or more fields failed validation",
    "details": {
      "errors": [
        {"field": "name",  "message": "is required"},
        {"field": "name",  "message": "length must be at least 3"},
        {"field": "email", "message": "must be a valid email address"},
        {"field": "age",   "message": "must be at most 150"},
        {"field": "role",  "message": "must be one of: admin, user, guest"}
      ]
    }
  }
}
```

## Built-in Rules

Rules are comma-separated inside the `validate` tag. Rules that take a parameter use `name=value`.

| Rule | Target kinds | Behavior |
|------|--------------|----------|
| `required` | any | Fails if the field is zero-valued. For slices and maps, nil or empty both fail. |
| `min=N` | string, slice, array, map | Length must be ≥ N. |
| `min=N` | int, uint, float | Value must be ≥ N. |
| `max=N` | string, slice, array, map | Length must be ≤ N. |
| `max=N` | int, uint, float | Value must be ≤ N. |
| `email` | string | Parses via `net/mail.ParseAddress`. Empty strings pass (combine with `required` to enforce presence). |
| `url` | string | Parses via `net/url.Parse` and requires non-empty scheme + host. Empty strings pass. |
| `regex=PATTERN` | string | Must match the compiled regex. Patterns are compiled once and cached. |
| `oneof=a b c` | string | Must equal one of the space-separated values. |

### Field Names in Errors

`FieldError.Field` uses the JSON tag when present (`json:"display_name"` → reports `display_name`) so error payloads line up with the wire format your client sees. Falls back to the Go field name if no JSON tag is set.

### Nested Structures

Recursion walks:

- Embedded structs
- Pointer-to-struct fields (nil pointers are skipped — use `required` on the pointer field if you want to enforce presence)
- Slices/arrays of structs — each element is visited with `Path` set to `parent[i]`

```go
type Address struct {
    City string `json:"city" validate:"required"`
}
type User struct {
    Name    string    `json:"name"    validate:"required"`
    Address Address   `json:"address"`
    Tags    []string  `json:"tags"    validate:"min=1,max=5"`
}

err := validator.Struct(User{Tags: nil})
// Two FieldErrors:
//   {Field: "name", Path: "",        Message: "is required"}
//   {Field: "city", Path: "address", Message: "is required"}
//   {Field: "tags", Path: "",        Message: "length must be at least 1"}
```

## Integration with Service Layers

For handlers that use the service-layer pipeline, pair `validator.Struct` with the existing `Validation` layer:

```go
import (
    "github.com/suryakencana007/espresso/v2"
    servicemiddleware "github.com/suryakencana007/espresso/v2/middleware/service"
    "github.com/suryakencana007/espresso/v2/validator"
)

layers := espresso.Layers(
    espresso.Validation(servicemiddleware.ValidatorFunc[*espresso.JSON[CreateUserReq]](
        func(ctx context.Context, req *espresso.JSON[CreateUserReq]) error {
            return validator.Struct(req.Data)
        },
    )),
)
```

The service-layer `Validation` returns `ErrValidation{Err: ...}` on failure, which `writeHandlerError` detects and converts into the structured 400 JSON response.

## Thread Safety

`validator.Struct` is safe for concurrent use. The internal regex cache is a `sync.Map`; compiled regex patterns are reused across calls.

## See Also

- [Error Handling](/error-handling) — how `FieldErrors` becomes a JSON error response
- [Service Layers](/api/middleware-service) — pipeline-level validation
- [Guide: Validation](/guide/validation) — narrative walkthrough
