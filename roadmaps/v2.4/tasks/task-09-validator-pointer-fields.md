# Task 9: Validator — Pointer-Field Dereference

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 0.5 day
**Dependencies:** None

## Context

Validator rules never dereference pointer fields — audit-confirmed by reproduction:

```go
type Req struct {
    ContactEmail *string `validate:"email"`
    Age          *int    `validate:"min=18"`
}
// r := Req{ContactEmail: strPtr("a@b.com"), Age: intPtr(30)} — VALID inputs
// Post-validate: 2 errors:
//   "email rule requires string field"
//   "min/max not supported for kind ptr"
```

`walkStruct` (`validator/validator.go:104-110`) applies rules to the raw field value; only dereferences pointers for the recursion step afterwards (`validator.go:113-119`). Every rule except `required` is unusable on the canonical Go pattern for optional JSON fields (`*string`, `*int`, `*float`, `*bool` with rules).

This directly contradicts `docs/guide/validation.md:121` ("Nil pointer fields are skipped") and turns well-formed client requests into 400s once such a tag exists on any pointer field.

## Acceptance Criteria

- [ ] `*string` with `validate:"email"` and a valid value like `strPtr("a@b.com")` passes validation.
- [ ] `*int` with `validate:"min=18"` and a valid value like `intPtr(30)` passes validation.
- [ ] `*string` with `validate:"required,min=3"`, when nil, fails on `required` only (not on `min`). When non-nil short value ("ab"), fails on `min`.
- [ ] Nil pointer fields skip all non-`required` rules (matches the doc contract at `docs/guide/validation.md:121`).
- [ ] Non-nil pointer fields have their rules applied to the dereferenced element value.
- [ ] Existing non-pointer validator tests continue to pass unchanged.
- [ ] `required` continues to operate on the pointer itself (i.e. `*string` is "required" iff non-nil, not iff non-empty).

## Technical Approach

### Step 9.1 — Reproduce the failure

```go
func TestValidator_PointerStringEmail_Valid(t *testing.T) {
    type R struct{ Email *string `validate:"email"` }
    v := validator.NewValidator()
    s := "user@example.com"
    err := v.Struct(&R{Email: &s})
    if err != nil {
        t.Fatalf("expected valid, got: %v", err) // pre-fix: fails with "email rule requires string field"
    }
}

func TestValidator_PointerIntMin_Valid(t *testing.T) {
    type R struct{ Age *int `validate:"min=18"` }
    v := validator.NewValidator()
    n := 30
    err := v.Struct(&R{Age: &n})
    if err != nil {
        t.Fatalf("expected valid, got: %v", err) // pre-fix: fails with "min/max not supported for kind ptr"
    }
}

func TestValidator_PointerNilSkipsNonRequired(t *testing.T) {
    type R struct{ Email *string `validate:"email"` }
    v := validator.NewValidator()
    if err := v.Struct(&R{Email: nil}); err != nil {
        t.Fatalf("expected nil pointer to skip non-required rules, got: %v", err)
    }
}

func TestValidator_PointerNilFailsRequired(t *testing.T) {
    type R struct{ Email *string `validate:"required,email"` }
    v := validator.NewValidator()
    if err := v.Struct(&R{Email: nil}); err == nil {
        t.Fatal("expected required to fail on nil pointer")
    }
}
```

Confirm the first three fail on pre-fix code, the fourth already passes.

### Step 9.2 — Fix walkStruct

In `walkStruct` (`validator/validator.go:104-119`), before applying rules to a pointer field:

```go
for _, rule := range rules {
    // Special-case required — operates on the pointer.
    if rule.name == "required" {
        if err := ruleRequired(field, ...); err != nil { errs = append(errs, err) }
        continue
    }
    // Non-required rule on a pointer: skip if nil, deref if not.
    if field.Kind() == reflect.Ptr {
        if field.IsNil() { continue } // documented: nil pointers skip non-required
        field = field.Elem() // rules now see the underlying value
    }
    if err := applyRule(rule, field, ...); err != nil { errs = append(errs, err) }
}
```

Preserve the existing recursion into pointer-to-struct fields for nested validation (`validator.go:113-119`).

### Step 9.3 — Verify against docs/guide/validation.md

Re-read `docs/guide/validation.md:121` after the fix. The current text says "Nil pointer fields are skipped" — with the fix, this becomes true. Update to specifically call out that `required` on a pointer still applies (this is Task 10's turf but a one-line clarification here is fine to avoid a follow-up docs PR).

## Tests Required

- `TestValidator_PointerStringEmail_Valid`, `_Nil`, `_Invalid`.
- `TestValidator_PointerIntMin_Valid`, `_Nil`, `_Invalid`.
- `TestValidator_PointerFloatMin_Valid`, `_Nil`, `_Invalid`.
- `TestValidator_PointerBool_Nil`.
- `TestValidator_PointerRequired`: verify `required` still fires on nil pointer.
- `TestValidator_NestedPointerStruct`: pointer-to-struct fields still recurse for nested validation.
- Run with `-race`.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -race ./validator/... -count=2` clean.
- [ ] `go test -race ./... -count=2` clean (regression across the module).
- [ ] `golangci-lint run ./...` clean.
- [ ] CI's `Test (race)` job green on the PR.
- [ ] CHANGELOG `[Unreleased]` entry under `Fixed`: validator rules now dereference non-nil pointer fields; nil pointer fields skip non-`required` rules as documented.
- [ ] No public API signature changed.
