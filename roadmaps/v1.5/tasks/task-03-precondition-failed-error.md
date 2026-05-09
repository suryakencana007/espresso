# Task 3: `ErrPreconditionFailed` (412)

**Priority:** 🔴 P0 — Feature
**Estimated Effort:** 0.25 day
**Dependencies:** None
**Closes:** F-07 in `roadmaps/USAGE_ESPRESSO.md`

## Context

Espresso v1.4.0 ships nine `*espresso.Error` constructors:

```
ErrBadRequest, ErrUnauthorized, ErrForbidden, ErrNotFound,
ErrConflict, ErrUnprocessableEntity, ErrTooManyRequests,
ErrInternal, ErrServiceUnavailable
```

Missing: 412 Precondition Failed. Barista's canary handler hits this when the Flagger CRD isn't installed (`CANARY_PROVIDER_MISSING` is semantically a 412, not a 409 or 422). The current workaround is `espresso.NewError(http.StatusPreconditionFailed, msg).WithCode(...)` — one extra line per occurrence, inconsistent with the rest of the API surface.

This task adds the missing constructor for symmetry. Smallest task on the v1.5 roadmap.

## Acceptance Criteria

- [ ] `ErrPreconditionFailed(message string) *Error` added to `error.go` next to `ErrConflict` / `ErrUnprocessableEntity`.
- [ ] Constructor returns an `*Error` with `Status: 412` and a default code `PRECONDITION_FAILED` (mirroring the convention of the other constructors).
- [ ] One unit test asserts status code, default error code, and JSON envelope shape.
- [ ] Constructor listed in:
  - Main `README.md` "Error Handling" section.
  - `docs/error-handling.md` constructor list.
  - `error.go` package-level doc comment if it enumerates constructors.

## Technical Approach

### Step 3.1: Add the constructor

Find the existing constructors in `error.go`. They follow a pattern:

```go
func ErrConflict(message string) *Error {
    return &Error{
        Status:  http.StatusConflict,
        Code:    "CONFLICT",
        Message: message,
    }
}
```

Add the matching one between `ErrConflict` and `ErrUnprocessableEntity` (status order: 409, 412, 422):

```go
// ErrPreconditionFailed creates a 412 Precondition Failed error.
//
// Use when a precondition the request asserts (or the system requires)
// is not met — for example, missing prerequisite infrastructure
// (CRD not installed), an If-Match header that doesn't match, or
// a feature flag that's required to be enabled.
func ErrPreconditionFailed(message string) *Error {
    return &Error{
        Status:  http.StatusPreconditionFailed,
        Code:    "PRECONDITION_FAILED",
        Message: message,
    }
}
```

### Step 3.2: Test

```go
func TestErrPreconditionFailed(t *testing.T) {
    err := ErrPreconditionFailed("CRD not installed")
    if err.Status != http.StatusPreconditionFailed {
        t.Errorf("status = %d, want 412", err.Status)
    }
    if err.Code != "PRECONDITION_FAILED" {
        t.Errorf("code = %q, want PRECONDITION_FAILED", err.Code)
    }
    if err.Message != "CRD not installed" {
        t.Errorf("message = %q", err.Message)
    }
}
```

Mirror the existing test pattern for the other constructors — don't invent a new shape.

### Step 3.3: Documentation

- `README.md` "Error Handling" — add `ErrPreconditionFailed` to the constructor list (the inline `Constructors:` line that already enumerates them).
- `docs/error-handling.md` — add the constructor to its enumeration with an example use case ("missing prerequisite infrastructure such as a Kubernetes CRD").

## Tests Required

- `TestErrPreconditionFailed` — status, code, message wired correctly.
- (Optional) `TestErrPreconditionFailed_JSONEnvelope` — full HTTP round-trip via `httptest` asserting the standard `{"error":{...}}` envelope.

## Definition of Done

- [ ] Constructor present, tested, documented.
- [ ] `go test ./... -race` clean.
- [ ] `golangci-lint run ./...` clean.
- [ ] CHANGELOG entry under `[Unreleased]` → `Added`: `ErrPreconditionFailed`.
- [ ] PR description references F-07 in `roadmaps/USAGE_ESPRESSO.md`.
