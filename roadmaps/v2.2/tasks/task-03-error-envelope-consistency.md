# Task 3: Structured-JSON Envelope on Every Error Path

**Priority:** 🟡 P1 — Should Have (hardening)
**Estimated Effort:** 1.5 days
**Dependencies:** Task 2 (shares `error.go`)

> **Status: ✅ Shipped 2026-06-28 (v2.2.0).** Delivered via #43 — `internal/errorenvelope` leaf; auth/rate-limit/panic unified.

## Context

Espresso advertises a single error shape for every failure it produces:

```json
{"error":{"code":"...","message":"...","details":{...},"request_id":"..."}}
```

`TestWithLayers_ExtractorErrorReturnsStructuredJSON` (`withlayers_test.go` ~line 420) locks that shape for the extractor, handler, SSE, and WebSocket paths — all of which route through `writeErrorResponse` / `writeHandlerError` / `writeExtractError` in `error.go` and serialize the `errorResponse{errorBody{Code,Message,Details,RequestID}}` wrapper (`error.go` ~178-205). A post-v2.1 analysis on 2026-06-27 found **two framework error paths that bypass that envelope**:

1. **`RecoverMiddleware`** (`middleware/http/middleware.go` ~69-106) emits a **hand-rolled anonymous-struct JSON** with code `"PANIC"` that **omits the `details` key entirely** (its inline struct has only `Code`, `Message`, `RequestID` — see ~87-96). Close to canonical, but not byte-identical: a client that switches on the presence of `details` sees a different shape from a 500 produced by `writeHandlerError`.

2. **Auth middleware and the rate limiter** emit `http.Error(...)` `text/plain`:
   - `JWTMiddleware` / `BasicAuthMiddleware` / `APIKeyMiddleware` (`middleware/http/auth.go` ~57, 62, 68, 107, 114, 120, 164, 170, 175) → `http.Error(w, "Unauthorized…", 401)`.
   - `RateLimitMiddleware` (`middleware/http/middleware.go` ~251) → `http.Error(w, "Too Many Requests", 429)`.
   A caller hitting a 401 or 429 gets `text/plain` with no `code`, no `request_id`, no envelope — the opposite of what a 400 extractor failure returns.

**Why this hasn't already been fixed — the load-bearing constraint.** The dependency direction is **root → `httpmiddleware`**: `error.go` imports `httpmiddleware` (for `GetRequestID`, see `error.go` line 9), and `middleware/http` does **not** import the root package. That is deliberate and must stay that way — the root package's test files import `httpmiddleware`, so a back-edge would form an import cycle. This is precisely **why `RecoverMiddleware` hand-rolls its JSON today**: it physically cannot call the root package's `writeErrorResponse` / construct an `*espresso.Error` without creating a cycle. Any fix for this task must respect that direction. The codebase already contains the template for breaking exactly this kind of cycle: `internal/validatehook` is a stdlib-only leaf package that both root and `extractor` import precisely because `extractor` cannot import root.

Finally — this finding was **surfaced by analysis, not yet independently locked by a test** (unlike F-1 and F-2, which have runnable reproductions). So the first move here is to *characterize* the current behavior with a test, not to assume it. This task is cross-package and cycle-sensitive, which is why it is **P1, not P0**.

## Acceptance Criteria

- [x] **(First.)** A characterization test confirms the current behavior before any change: the panic path emits JSON **without** a `details` key with code `"PANIC"`; auth 401 and rate-limit 429 emit `text/plain` (not the JSON envelope). This pins the "before" state so the unification is provably a fix, not a guess.
- [x] A cycle-safe shared writer for the canonical envelope exists, importable by **both** the root package and `httpmiddleware` without forming an import cycle (root → httpmiddleware stays intact).
- [x] `RecoverMiddleware` emits the canonical envelope including the `details` key (present-but-empty/omitted per the canonical `omitempty` rule, matching `writeErrorResponse`) and `request_id`.
- [x] Auth failures (JWT / BasicAuth / APIKey, 401) emit the canonical JSON envelope with code `UNAUTHORIZED` and `request_id` instead of `text/plain`.
- [x] Rate-limit rejection (429) emits the canonical JSON envelope with code `TOO_MANY_REQUESTS` and `request_id` instead of `text/plain`.
- [x] `TestWithLayers_ExtractorErrorReturnsStructuredJSON` still passes unchanged — the extractor/handler/SSE/WS paths are not regressed.
- [x] The shared writer is stdlib-only (no dependency on `sonic`, the root package, or the validator) so the leaf stays importable by everyone.

## Technical Approach

### Step 3.1 — Characterize Current Behavior (do this first)

Before touching any production code, lock the "before" state. Add tests that drive each suspect path through an `httptest.ResponseRecorder` and assert today's actual output:

```go
// Panic path: today emits {"error":{"code":"PANIC","message":...,"request_id":...}}
// with NO "details" key.
func TestRecoverMiddleware_CurrentShape(t *testing.T) { ... }

// Auth + rate-limit: today emit text/plain, NOT the JSON envelope.
func TestAuthUnauthorized_CurrentShape(t *testing.T) { ... }
func TestRateLimit_CurrentShape(t *testing.T)        { ... }
```

These prove the finding (analysis-surfaced) and become the diff that the PR review reads against. They will be *flipped* (not deleted) in Step 3.4 once the paths are unified.

### Step 3.2 — Extract a Cycle-Safe Envelope Leaf

The constraint (root → httpmiddleware, never the reverse) rules out `httpmiddleware` calling `writeErrorResponse`. Mirror the `internal/validatehook` cycle-break: introduce a stdlib-only leaf package that owns the envelope **shape and writer**, and have **both** root and `httpmiddleware` import it.

Sketch (`internal/errorenvelope/errorenvelope.go`):

```go
// Package errorenvelope holds the canonical Espresso error wire format and
// the writer that serializes it. It is a stdlib-only leaf package so that
// both the root espresso package and middleware/http can produce the exact
// same {"error":{...}} shape without forming an import cycle (root imports
// middleware/http, so middleware/http must not import root).
package errorenvelope

type Body struct {
    Code      string         `json:"code"`
    Message   string         `json:"message"`
    Details   map[string]any `json:"details,omitempty"`
    RequestID string         `json:"request_id,omitempty"`
}

// Write serializes {"error": body} with the canonical content type and status.
func Write(w http.ResponseWriter, status int, body Body) { ... }
```

Then:

- **Root (`error.go`):** re-point `errorResponse` / `errorBody` (lines 178-205) at the leaf — either type-alias `errorBody = errorenvelope.Body` or have `writeErrorResponse` delegate to `errorenvelope.Write`. The externally observable JSON must not change; `TestWithLayers_ExtractorErrorReturnsStructuredJSON` is the guard.
- **`httpmiddleware`:** import the leaf directly. `RequestID` is already locally available via `GetRequestID(r.Context())`, so the middleware can build a `errorenvelope.Body` with no root dependency.

> Decide and record in the PR whether the leaf lives under `internal/` (matching `validatehook`, hidden from users) or is exported. Default to `internal/` unless a downstream (Barista) needs to write the envelope itself.

### Step 3.3 — Route the Three Stray Paths Through the Leaf

- **`RecoverMiddleware`** (`middleware/http/middleware.go` ~87-100): replace the inline anonymous struct with `errorenvelope.Write(w, http.StatusInternalServerError, errorenvelope.Body{Code: "PANIC", Message: "internal server error", RequestID: GetRequestID(r.Context())})`. `Details` is left nil → `omitempty` keeps it absent, which now matches a `writeHandlerError` 500 exactly (both omit it). The panic logging at lines 78-84 stays untouched.
- **Auth** (`middleware/http/auth.go` ~57-175): replace each `http.Error(w, "Unauthorized…", 401)` with an `errorenvelope.Write(w, http.StatusUnauthorized, Body{Code: "UNAUTHORIZED", Message: ..., RequestID: GetRequestID(r.Context())})`. Preserve the existing message text where it carries signal (e.g. the JWT `err.Error()` detail can go into `Message` or `Details`, author's call — keep it inside the envelope, not as bare text).
- **Rate limit** (`middleware/http/middleware.go` ~251): replace `http.Error(w, "Too Many Requests", 429)` with the canonical 429 envelope, code `TOO_MANY_REQUESTS`.

### Step 3.4 — Flip the Characterization Tests Into Regression Locks

Update the Step 3.1 tests to assert the **new** canonical shape, so they now lock the unified behavior the way `TestWithLayers_ExtractorErrorReturnsStructuredJSON` locks the extractor path. The before/after of these assertions is the proof of the fix and belongs in the PR body.

## Risks

- **text/plain compatibility.** Auth and rate-limit responses are currently `text/plain` 401/429. Some callers (proxies, simple clients, existing Barista integration code) may key off the body string `"Unauthorized"` / `"Too Many Requests"` or the `text/plain` content type. Switching to JSON is a **behavior change** and must be called out in the migration/upgrade note (Task 5, under `Changed`), with the before/after body and content type shown. Confirm with Barista whether anything parses these bodies before merging.
- **Cycle reintroduction.** The whole point is to avoid root ↔ httpmiddleware. The leaf must stay stdlib-only; do **not** let it pull in the root package, the validator, or `sonic`. A `go list` / build check that `middleware/http` does not import the root package belongs in the verification task (Task 4).
- **Double-write / header-after-write.** `http.Error` sets the content type and status itself; the replacement writer also sets them. Ensure each replaced site still writes headers exactly once and returns immediately after (the existing `return` statements stay).

## Tests Required

- `TestRecoverMiddleware_EnvelopeShape`: trigger a panic through `RecoverMiddleware`; assert `Content-Type: application/json`, status 500, body `{"error":{"code":"PANIC","message":"internal server error","request_id":...}}`, and that the `details` key handling is identical to a `writeHandlerError` 500 (both omit it).
- `TestAuthMiddleware_Unauthorized_Envelope`: JWT/BasicAuth/APIKey rejection → 401 canonical JSON envelope with code `UNAUTHORIZED` and a `request_id` (assert with `RequestIDMiddleware` in front so the ID is populated).
- `TestRateLimit_TooManyRequests_Envelope`: limiter denies → 429 canonical JSON envelope with code `TOO_MANY_REQUESTS`.
- `TestWithLayers_ExtractorErrorReturnsStructuredJSON` (existing): **must still pass unchanged** — proves the extractor/handler/SSE/WS paths and the root-side envelope wiring did not regress.
- A build/import-direction guard (or hand it to Task 4): `middleware/http` does not import the root `espresso` package.
- Run with `-race`.

## Breaking Changes

**Behavior change (documented).** Auth (401) and rate-limit (429) responses change from `text/plain` to the canonical JSON envelope; the panic (500) body gains envelope-consistent `details` handling. No API signature changes. Goes under `Changed` in the CHANGELOG (Task 5) with before/after bodies, and into the upgrade note.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] The cycle-safe leaf is stdlib-only; `middleware/http` still does not import the root package (verified).
- [x] `go test -race ./...` clean, including the unchanged `TestWithLayers_ExtractorErrorReturnsStructuredJSON`.
- [x] `golangci-lint run ./...` clean.
- [x] CHANGELOG `[Unreleased]` → `Changed` entry drafted with before/after for the auth/rate-limit/panic bodies.
- [x] PR description shows the flipped characterization tests (before/after assertions) and explicitly flags the `text/plain` → JSON behavior change for auth/rate-limit, with a note to confirm no Barista caller parses the old text bodies.
