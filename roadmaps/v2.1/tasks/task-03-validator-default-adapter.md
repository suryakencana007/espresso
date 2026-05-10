# Task 3: `validator.AsDefaultValidator()` Adapter

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 0.5 day
**Dependencies:** None

## Context

v2.0 task-05 added `espresso.SetDefaultValidator(fn func(any) error)` for opt-in auto-validation on extract. The wiring requires users to wrap `validator.Struct` in a closure that converts `FieldErrors` → `ValidationErrors` so the response body has the framework's standard 400 shape:

```go
// cmd/example/validate/main.go (current)
espresso.SetDefaultValidator(func(v any) error {
    if err := validator.Struct(v); err != nil {
        if fe, ok := err.(espresso.FieldErrors); ok {
            return espresso.ValidationErrors(fe.ToValidationErrors())
        }
        return err
    }
    return nil
})
```

Every user who wires the bundled validator copies this exact closure. Ship a helper.

## Acceptance Criteria

- [ ] `validator/` package exposes `AsDefaultValidator() func(v any) error` (or a similarly-named factory).
- [ ] The returned closure is the canonical `validator.Struct` + `FieldErrors → ValidationErrors` adapter.
- [ ] `cmd/example/validate/main.go` updated to use the helper.
- [ ] Unit test asserts: invalid input → non-nil `*espresso.Error` with status 400 and code `VALIDATION_ERROR`; valid input → nil.
- [ ] `docs/guide/validation.md` "Auto-Validate on Extract" section updated to show the helper as the recommended path; the inline closure preserved as the customization-point pattern.
- [ ] CHANGELOG `[Unreleased]` → `Added`.

## Technical Approach

### Step 3.1 — Add the Helper

In `validator/validator.go` (or a new `validator/default.go`):

```go
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
```

The body is exactly the closure from the v2.0 example. ~10 LOC including the godoc.

### Step 3.2 — Update the Example

`cmd/example/validate/main.go`:

```go
// Before
func init() {
    espresso.SetDefaultValidator(structAdapter)
}

func structAdapter(v any) error {
    if err := validator.Struct(v); err != nil {
        if fe, ok := err.(espresso.FieldErrors); ok {
            return espresso.ValidationErrors(fe.ToValidationErrors())
        }
        return err
    }
    return nil
}

// After
func init() {
    espresso.SetDefaultValidator(validator.AsDefaultValidator())
}
```

### Step 3.3 — Test

```go
func TestAsDefaultValidator_InvalidReturns400(t *testing.T) {
    type Req struct {
        Name string `validate:"required"`
    }
    fn := AsDefaultValidator()
    err := fn(&Req{}) // empty name → required fails
    if err == nil {
        t.Fatal("expected error from empty Name")
    }
    var espErr *espresso.Error
    if !errors.As(err, &espErr) {
        t.Fatalf("expected *espresso.Error, got %T", err)
    }
    if espErr.StatusCode != 400 || espErr.Code != "VALIDATION_ERROR" {
        t.Errorf("status=%d code=%q, want 400/VALIDATION_ERROR", espErr.StatusCode, espErr.Code)
    }
}

func TestAsDefaultValidator_ValidReturnsNil(t *testing.T) {
    type Req struct {
        Name string `validate:"required"`
    }
    fn := AsDefaultValidator()
    if err := fn(&Req{Name: "alice"}); err != nil {
        t.Errorf("unexpected error for valid input: %v", err)
    }
}
```

### Step 3.4 — Docs

`docs/guide/validation.md`:

- "Auto-Validate on Extract" section: replace the closure-heavy first example with the helper-based one.
- Keep a "Custom error mapping" subsection showing the inline-closure form for users who need different code/details.

## Tests Required

See Step 3.3 above. Plus a sanity check that the helper path produces the same JSON 400 body as the inline-closure path (by shape, not by reference equality).

## Breaking Changes

None. Pure addition.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -race ./validator/...` clean.
- [ ] `golangci-lint run ./...` clean.
- [ ] CHANGELOG `[Unreleased]` → `Added` entry.
- [ ] PR description includes a one-line before/after demonstrating the LOC reduction in user code (~10 lines down to 1).
