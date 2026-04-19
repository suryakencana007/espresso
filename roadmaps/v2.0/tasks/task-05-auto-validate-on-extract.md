# Task 5: Optional Auto-Validate on Extract

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 2-3 days
**Dependencies:** Task 4 (typed Validation layer) is preferred but not strictly required

## Context

v1.3 shipped a `validator/` subpackage (`validator.Struct(v any) error`) that reads `validate:"..."` struct tags and returns `espresso.FieldErrors`. To wire it into a route today the user writes:

```go
router.Post("/users", espresso.Doppio(func(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[Res], error) {
    if err := validator.Struct(req.Data); err != nil {
        return espresso.JSON[Res]{}, espresso.ValidationErrors(err.(espresso.FieldErrors).ToValidationErrors())
    }
    // business logic
}))
```

That's four extra lines per handler and exactly the same pattern every time — the kind of boilerplate frameworks are supposed to eliminate. v2.0 adds an opt-in hook that runs validation automatically after extraction, so the handler body becomes:

```go
router.Post("/users", espresso.Doppio(func(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[Res], error) {
    // validation already ran; if req.Data had bad tags the client got a 400
    // business logic
}))
```

## Design Decision — Where Does Validation Run?

Two defensible places:

### Option A — Inside the extractor

`JSON[T].Extract(r)` decodes the body, then calls `validator.Struct(&j.Data)`. Rejection surfaces as an extraction error, which flows through `writeExtractError` → JSON 400.

Pro: natural — "extraction" already means "read and parse", adding validate is a tiny extension.
Con: couples extractors to the validator package (or requires a late-bound hook mechanism).

### Option B — As a default layer

A `ValidateLayer()` that automatically wraps every handler when enabled.

Pro: clean separation — extractor stays minimal, validation is a distinct pipeline step.
Con: every handler registration adds a layer; micro-overhead if the user hasn't tagged anything.

**Recommended:** **Option A**, but with a hook so the validator is pluggable — the espresso core does not import the `validator/` subpackage.

```go
// In core: a package-level, settable function
var DefaultValidator func(v any) error

// In JSON[T].Extract (and Query/Form/Path/etc):
if DefaultValidator != nil {
    if err := DefaultValidator(&j.Data); err != nil {
        return err
    }
}
```

Users opt in by setting:

```go
import "github.com/suryakencana007/espresso/v2/validator"

func init() {
    espresso.DefaultValidator = validator.Struct
}
```

This keeps the core free of the validator dependency while making opt-in ergonomic.

## Acceptance Criteria

- [ ] `espresso.DefaultValidator` package-level variable exists with the signature `func(v any) error`.
- [ ] `JSON[T].Extract`, `Query[T].Extract`, `Path[T].Extract`, `Form[T].Extract`, `Header[T].Extract`, `Cookie[T].Extract` each call `DefaultValidator(&.Data)` when set, passing the decoded-but-not-yet-returned value.
- [ ] If the validator returns a non-nil error, the extractor propagates it — the existing `writeExtractError` path turns it into a structured 400 JSON response.
- [ ] An example in `cmd/example/` shows the opt-in init() wiring.
- [ ] Documentation in `docs/guide/extractors.md` (or equivalent) covers the opt-in.
- [ ] When `DefaultValidator` is nil (the default), extractors behave exactly as today — zero overhead.

## Technical Approach

### Step 5.1 — Add the Hook

In a new top-level file `validate.go` (or inside `core.go`):

```go
// DefaultValidator is called by every extractor after decoding to validate
// the extracted value. When nil (the default), no validation runs and the
// framework behaves identically to v1.x.
//
// Set it once at program startup:
//
//	import "github.com/suryakencana007/espresso/v2/validator"
//	func init() { espresso.DefaultValidator = validator.Struct }
//
// The function must be safe to call concurrently. The framework calls it
// exactly once per successful extraction, passing a pointer to the decoded
// value so the validator can reflect into tagged fields.
var DefaultValidator func(v any) error
```

### Step 5.2 — Wire It into Extractors

Every `Extract` implementation in `extractor/extractor.go` and `response.go` (`JSON[T]`) ends with:

```go
if espresso.DefaultValidator != nil {
    return espresso.DefaultValidator(&e.Data)
}
return nil
```

This is a tight loop; guard the nil check once per `Extract` call, not per field.

### Step 5.3 — Make It Composable With the Existing Validation Layer

The existing `Validation[Req]` layer (Task 4) runs **after** extraction, on the full request object. `DefaultValidator` runs **during** extraction, on the raw decoded payload. They solve different problems:

- `DefaultValidator` for tag-driven, per-field checks — can reject before the handler is called at all.
- `Validation[Req]` for cross-field logic, ctx-dependent checks, database lookups.

Document this; they compose, they don't overlap.

### Step 5.4 — Example

New file `cmd/example/validate/main.go`:

```go
package main

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

func init() {
    espresso.DefaultValidator = validator.Struct
}

func main() {
    espresso.Portafilter().Post("/users", espresso.Doppio(
        func(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.Text, error) {
            return espresso.Text{Body: "created: " + req.Data.Name}, nil
        },
    )).Brew()
}
```

## Tests Required

- Per extractor: tagged struct + invalid input → extractor returns FieldErrors → handler responds with 400 JSON in the documented shape.
- Per extractor: tagged struct + valid input → handler runs normally.
- With `DefaultValidator` nil: extractors produce zero overhead (benchmark diff < 1 ns).
- With `DefaultValidator` set but target has no `validate` tags: no FieldErrors, no panic.

## Breaking Changes

Technically none — `DefaultValidator` defaults to nil, preserving v1.x behavior. But users who set the hook will get new 400 responses on requests that previously passed through — that's a behavioral change they opted into.

## Definition of Done

- Hook + wiring in all extractors
- Bench shows nil-path is free
- Example program runs
- Docs updated
- `go test ./... -race` passes
- `golangci-lint run ./...` clean
- Migration guide entry ("New: opt-in auto-validation via DefaultValidator")
- CHANGELOG `[Unreleased]` entry under `Added`
