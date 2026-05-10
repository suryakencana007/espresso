---
title: Validation
description: Validate request payloads with struct tags
---

# Validation

Espresso ships a small, dependency-free validator in the `validator/` subpackage. It reads `validate:"..."` struct tags via reflection and returns errors that plug directly into the framework's structured error pipeline — so a client always sees a consistent JSON error shape, regardless of whether the failure came from parsing, validation, or business logic.

This guide covers the pragmatic usage patterns. See [the API reference](/api/validator) for the exhaustive rule list and type signatures.

## Why a Framework Validator?

You could validate inside every handler manually:

```go
if req.Data.Name == "" {
    return empty, espresso.ErrBadRequest("name is required")
}
if len(req.Data.Name) < 3 {
    return empty, espresso.ErrBadRequest("name must be at least 3 chars")
}
// ... repeat for every field ...
```

That works, but it scales poorly. Tag-driven validation declares rules alongside the type, so the request schema, the wire format, and the validation contract all live in one place:

```go
type CreateUserReq struct {
    Name  string `json:"name"  validate:"required,min=3,max=50"`
    Email string `json:"email" validate:"required,email"`
}
```

## The Basic Pattern

```go
import (
    "context"
    "github.com/suryakencana007/espresso/v2"
    "github.com/suryakencana007/espresso/v2/validator"
)

type CreateUserReq struct {
    Name  string `json:"name"  validate:"required,min=3,max=50"`
    Email string `json:"email" validate:"required,email"`
    Role  string `json:"role"  validate:"oneof=admin user guest"`
}

type CreateUserRes struct {
    ID string `json:"id"`
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[CreateUserRes], error) {
    if err := validator.Struct(req.Data); err != nil {
        fe := err.(espresso.FieldErrors)
        return espresso.JSON[CreateUserRes]{}, espresso.ValidationErrors(fe.ToValidationErrors())
    }
    // business logic — req.Data is now known-good
    return espresso.JSON[CreateUserRes]{Data: CreateUserRes{ID: "usr_123"}}, nil
}
```

`validator.Struct` returns `nil` on success or an `espresso.FieldErrors` on failure. The cast then feeds into `espresso.ValidationErrors(...)`, which returns the standard 400 JSON shape with code `VALIDATION_ERROR` and a `details.errors` array.

## Available Rules

Quick reference — see [the API page](/api/validator#built-in-rules) for full semantics:

| Rule | Example | What it does |
|------|---------|--------------|
| `required` | `validate:"required"` | Fails if the field is zero-valued. |
| `min=N` | `validate:"min=3"` | Strings/slices/maps: length ≥ N. Numbers: value ≥ N. |
| `max=N` | `validate:"max=50"` | Same target set, upper bound. |
| `email` | `validate:"email"` | `net/mail.ParseAddress`. Empty strings pass. |
| `url` | `validate:"url"` | Parsed URL with scheme + host. Empty strings pass. |
| `regex=PAT` | `validate:"regex=^[A-Z]{3}-\\d+$"` | Compiled and cached. |
| `oneof=a b c` | `validate:"oneof=admin user guest"` | Space-separated allow-list. |

Combine multiple rules with commas: `validate:"required,min=3,email"`.

## The `required` Gotcha

Format rules (`email`, `url`, `regex`) deliberately let empty strings pass. The intent is **presence is a separate concern from format**. Combine `required` explicitly when you need both:

```go
type Req struct {
    // Optional email field. If present, must be valid. Empty is OK.
    ContactEmail string `json:"contact_email" validate:"email"`

    // Required email field. Empty fails.
    PrimaryEmail string `json:"primary_email" validate:"required,email"`
}
```

## Nested and Collection Types

The validator recurses automatically:

```go
type Address struct {
    City    string `json:"city"    validate:"required"`
    Country string `json:"country" validate:"required,oneof=US CA UK"`
}

type User struct {
    Name    string    `json:"name"    validate:"required"`
    Address Address   `json:"address"`
    Tags    []string  `json:"tags"    validate:"min=1,max=5"`
    Partners []*User  `json:"partners"`  // recursion into pointer-to-struct slice
}

err := validator.Struct(User{})
// Errors report Path-prefixed fields:
//   {Field: "name",    Path: "",               Message: "is required"}
//   {Field: "city",    Path: "address",        Message: "is required"}
//   {Field: "country", Path: "address",        Message: "is required"}
//   {Field: "tags",    Path: "",               Message: "length must be at least 1"}
```

Nil pointer fields are skipped — if you want to enforce presence of a pointer field, add `required` to the field itself.

## Auto-Validate on Extract (since v2.0)

For the most common case — "validate every JSON/Query/Path/Form/etc. body
on every route" — wire the validator once globally and let extraction take
care of the rest:

```go
import (
    "github.com/suryakencana007/espresso/v2"
    "github.com/suryakencana007/espresso/v2/validator"
)

func init() {
    espresso.SetDefaultValidator(func(v any) error {
        if err := validator.Struct(v); err != nil {
            // Wrap FieldErrors in the standard 400 shape so downstream
            // gets {"error":{"code":"VALIDATION_ERROR",...}} bodies.
            if fe, ok := err.(espresso.FieldErrors); ok {
                return espresso.ValidationErrors(fe.ToValidationErrors())
            }
            return err
        }
        return nil
    })
}

type CreateUserReq struct {
    Name  string `json:"name"  validate:"required,min=3,max=50"`
    Email string `json:"email" validate:"required,email"`
}

// Validation already ran — req.Data is known good.
func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[Res], error) {
    return espresso.JSON[Res]{Data: ...}, nil
}
```

When `SetDefaultValidator` is unset (the default), extractors behave
identically to v1.x — the nil-fast path is one atomic load with zero
allocations (see `BenchmarkRunDefaultValidator_NilHook`).

`RunDefaultValidator(v any) error` is also exported so custom `Extract`
methods can opt into the same hook:

```go
func (r *MyRequest) Extract(req *http.Request) error {
    if err := json.NewDecoder(req.Body).Decode(r); err != nil {
        return err
    }
    return espresso.RunDefaultValidator(r)
}
```

A complete runnable demo lives at [`cmd/example/validate/main.go`](https://github.com/suryakencana007/espresso/v2/blob/main/cmd/example/validate/main.go).

### Auto-validate vs. the `Validation[Req]` layer

Both run validation — at different points in the pipeline:

| | Auto-validate | `Validation[Req]` layer |
|--|--|--|
| When it runs | During extraction | After extraction, before handler |
| Sees | The decoded payload only | The full extracted request value |
| Use it for | Tag-driven, per-field rules (the `validator/` package) | Cross-field, ctx-dependent, I/O-bound checks |
| Costs | One atomic load per Extract (free if unset) | One layer in the service pipeline per route |

They compose. A typical setup uses auto-validate for tag-driven syntax
checks and one or two `Validation[Req]` layers for business rules that
need the surrounding context.

## Pairing with Service Layers

If your handlers use Espresso's service layer pipeline (`espresso.WithLayersTyped`, `espresso.Layers`), move validation out of the handler body and into a layer so it runs before business logic:

```go
import (
    "context"
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
    espresso.Timeout(5 * time.Second),
)

router.Post("/users", espresso.WithLayersTyped[*espresso.JSON[CreateUserReq], espresso.JSON[CreateUserRes]](createUser, layers...))
```

The service-layer `Validation` layer catches the failure before `createUser` is called and emits the same structured 400 response.

## Custom Rules

The built-in rule set is intentionally small. For application-specific rules, compose `validator.Struct` with your own logic:

```go
func validateCreateUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) error {
    // Run tag-driven checks first.
    if err := validator.Struct(req.Data); err != nil {
        return err
    }
    // Then application-specific checks the validator can't express.
    if req.Data.Role == "admin" && !isAllowedAdmin(ctx, req.Data.Email) {
        fe := espresso.NewFieldErrors()
        _ = fe.AddFieldError("role", "this email is not authorized for admin", req.Data.Role)
        return *fe
    }
    return nil
}
```

## When Not to Use It

The validator is a convenience. If your rules are genuinely cross-field (e.g. "end_date must be after start_date"), database-dependent, or need internationalized error messages, write a typed validator by hand — the service-layer `Validator[Req]` interface exists for exactly that.

Rule of thumb: **tag-driven for shape, handwritten for semantics**.

## See Also

- [API: validator](/api/validator) — full rule semantics and type signatures
- [Error Handling](/error-handling) — how `FieldErrors` becomes a 400 JSON response
- [Service Layers](/guide/middleware/service) — pipeline-level validation
