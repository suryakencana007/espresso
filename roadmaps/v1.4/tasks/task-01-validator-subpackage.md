# Task 1: Struct-Tag Validator Subpackage

**Priority:** 🔴 P0 — Feature
**Estimated Effort:** 3 days
**Dependencies:** v1.3 `*Error` and `FieldErrors` types

## Context

`TODOS.md` #7 ("Request Validation Improvements") had been open since v1.0. The service-layer surface advertised `Validation(any)` since v1.0, but no concrete validator shipped with the framework — every Espresso user re-imported `go-playground/validator` or rolled their own.

v1.4 closes this gap with a small, dependency-free struct-tag validator that produces `espresso.FieldErrors` so the result flows through the existing structured-error pipeline without a parallel response shape.

This is **not** the auto-validate-on-extract feature (that's `roadmaps/v2.0/tasks/task-05-auto-validate-on-extract.md`). v1.4 ships the validator only; opting it into the request lifecycle remains the user's choice.

## Acceptance Criteria

- [x] New subpackage at `validator/` with `validator.go` and `validator_test.go`.
- [x] Public entry point: `validator.Struct(v any) error`.
- [x] Returns `espresso.FieldErrors` so `(*espresso.Error).WithFieldErrors(...)` and the JSON pipeline work without changes.
- [x] Built-in rules: `required`, `min`, `max`, `email`, `url`, `regex`, `oneof`.
- [x] `min` / `max` interpret as numeric value for numeric kinds and as length for `string` / slice / map kinds.
- [x] Recurses into nested structs, pointers to structs, and slices of structs with field-path tracking.
- [x] 16+ unit tests covering happy path and each rule's error path.
- [x] Documentation at `docs/guide/validation.md` (user guide) and `docs/api/validator.md` (API reference).
- [x] Zero new third-party deps in `go.mod` (validator is stdlib-only).
- [x] `cmd/example/` shows the validator integrated into a handler.

## Technical Approach

### Step 1.1: Package skeleton

Create `validator/validator.go` with:

- `Struct(v any) error` — single public entry point.
- A private rule registry: `var rules = map[string]ruleFn{...}`.
- `ruleFn` signature: `func(fieldValue reflect.Value, arg string) error`.

### Step 1.2: Rule implementations

| Rule | Signature | Notes |
|------|-----------|-------|
| `required` | `validate:"required"` | Non-zero value (empty string, zero number, nil pointer, empty slice/map all fail). |
| `min=N` / `max=N` | numeric on numeric kinds; length on string / slice / map. |
| `email` | RFC-5322 lite via `net/mail.ParseAddress`; no DNS check. |
| `url` | `net/url.ParseRequestURI` + scheme check. |
| `regex=...` | `regexp.MustCompile`; cache compiled regex per tag string. |
| `oneof=a b c` | space-separated allowlist; string compare. |

### Step 1.3: Recursion

Recurse into structs, `*struct`, and `[]struct` / `[]*struct`. Build the field path as `Outer.Inner[2].Field` so error messages are unambiguous.

### Step 1.4: Result shape

```go
type FieldError struct {
    Field   string  // dotted path, e.g. "User.Email"
    Rule    string  // "required", "min", etc.
    Message string  // human-readable
}
type FieldErrors []FieldError  // implements error
```

`FieldErrors` already exists in `error.go` (v1.3). Reuse — do not redefine.

### Step 1.5: Documentation

- `docs/guide/validation.md` — usage walkthrough: tags, nested structs, custom messages, integration with `*espresso.Error`.
- `docs/api/validator.md` — godoc-style reference for each rule.

### Step 1.6: Example

Add a `validate:"..."` tag to the `cmd/example/main.go` `CreateUserReq` and a call to `validator.Struct(req.Data)` in the create-user handler.

## Tests Required

- One test per rule, happy + error path.
- One test for nested struct recursion.
- One test for slice-of-struct recursion with index in path.
- One test for `min`/`max` length-vs-value dispatch on different kinds.
- One test verifying the returned error is assertable to `espresso.FieldErrors` (so callers can attach to `*espresso.Error`).

## Definition of Done

- [x] `go test ./validator/... -race` passes.
- [x] `golangci-lint run ./validator/...` clean.
- [x] Validator referenced from main `README.md` (Validation section).
- [x] CHANGELOG entry under `[Unreleased]` → `Added`.
- [x] No new entries in root `go.mod` `require` block.
