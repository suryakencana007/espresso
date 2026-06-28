# Task 4: Status-Code Matrix + Signature + Doc/Code Consistency Tests

**Priority:** 🔵 Verification — tests/coverage, no new features
**Estimated Effort:** 1 day
**Dependencies:** Tasks 1, 2, 3

> **Status: ✅ Shipped 2026-06-28 (v2.2.0).** Delivered via #44 — status / signature / doc-consistency matrices.

## Context

Tasks 1-3 are three independent behavior-correctness fixes that each touch the
error pipeline or the dispatch path, but they share a property: each replaces an
*ad hoc* assertion (a single `if status == 500` here, a single panic recovery
there) with a contract. A contract is only worth as much as the test that locks
it. This task does not change behavior — it builds the regression net that keeps
Tasks 1, 2, and 3 from silently regressing, and it codifies the v2.2 emphasis
("behavior matches contract") as an executable check rather than prose.

Today the relevant assertions are scattered: `TestWithLayers_ExtractorErrorReturnsStructuredJSON`
locks the envelope for the extractor/handler/SSE/WS paths only; `TestWithLayersTyped_WithTimeout`
(`withlayers_test.go` ~144-167) asserts a single status for one layer; there is
no single place that asserts "for every way an error can originate, the HTTP
status and the JSON envelope are both correct." There is also no guard against
re-introducing the false `func(ctx, *Req1, *Req2)` reflection claim that PR #39
removed, or other removed symbols creeping back into docs/godoc. This task
consolidates all of that into three matrix-style tests plus one
doc-consistency check.

This is verification work: it adds no new feature surface, only tests. It runs
last because it asserts the *combined* post-fix behavior of Tasks 1, 2, and 3.

## Acceptance Criteria

- [x] A table-driven `TestErrorStatusMatrix` asserts, for every error origin, both the HTTP status code AND the structured-JSON envelope shape (`{"error":{"code","message","details","request_id"}}`), with one row per origin:
  - [x] `extract_error` → 400, code `BAD_REQUEST` (extractor returns a non-`*espresso.Error`).
  - [x] `extract_espresso_error` → the carried status/code (extractor returns an `*espresso.Error`, e.g. `ErrUnprocessableEntity`).
  - [x] `handler_error` → the carried status/code (handler returns `ErrNotFound(...)`).
  - [x] `handler_plain_error` → 500, code `INTERNAL` (handler returns a non-`*espresso.Error`; the unchanged fallback).
  - [x] `service_validation` → 400, code `VALIDATION_ERROR`, with `details` carrying field errors (Task 2; matches auto-validate-on-extract).
  - [x] `service_circuit_breaker_open` → 503, code `SERVICE_UNAVAILABLE` (Task 2).
  - [x] `service_timeout` → 503, code `SERVICE_UNAVAILABLE` (Task 2; this row is the regression-lock for the behavior change called out in Task 2).
  - [x] `panic` → 500, code `PANIC`, envelope INCLUDES the `details` key (Task 3; previously omitted).
  - [x] `auth_failure` → 401, code `UNAUTHORIZED`, `Content-Type: application/json` (Task 3; previously text/plain).
  - [x] `rate_limit` → 429, code `TOO_MANY_REQUESTS`, `Content-Type: application/json` (Task 3; previously text/plain).
- [x] Every matrix row asserts `Content-Type: application/json; charset=utf-8` and that the body decodes into the envelope struct with a non-empty `code` and `message`; no row asserts on a text/plain body.
- [x] A table-driven `TestHandlerSignatureMatrix` registers one route per supported handler shape and asserts each produces a 2xx with the expected body:
  - [x] reflection `func() T`, `func(*Req) T`, `func(ctx, *Req) (T, error)`, and `Service[Req,Res]`.
  - [x] typed `HandlerCtxReqErr`, `HandlerCtxReq`, `HandlerReqErr`, `HandlerReq`, `HandlerCtxNoErr`.
  - [x] typed `HandlerCtxReq1Req2Err` / `Lungo` populates BOTH extractors (the shape that works today).
  - [x] the two-extractor reflection outcome from Task 1: if approach A (extend reflection), a reflection-registered two-extractor route returns 2xx with both extractors populated; if approach B (fail-fast), registration panics with the actionable message and the request-time `panic("espresso: invalid handler argument - this is a bug")` is unreachable.
- [x] A `TestNoRequestTimeInvalidArgumentPanic` (or equivalent) asserts the `"this is a bug"` panic in `createHandlerFromInfo` (`handler.go` ~744) is never reached at request time for any registered shape.
- [x] A doc/godoc-vs-code consistency check (`TestDocsConsistency`, scripted grep over `docs/` + exported godoc) fails if a removed symbol or a false signature claim reappears.
- [x] All new tests pass under `-race`.

## Technical Approach

### Step 4.1 — Status-Code Matrix Test

Add `error_status_matrix_test.go` at the root package. Build a single `*Router`
wired with the auth and rate-limit HTTP middleware and a recover middleware, then
register one route per origin. Drive each with `httptest` and assert against a
shared decoder for the canonical envelope:

```go
type envelope struct {
    Error struct {
        Code      string         `json:"code"`
        Message   string         `json:"message"`
        Details   map[string]any `json:"details"`
        RequestID string         `json:"request_id"`
    } `json:"error"`
}

cases := []struct {
    name        string
    path        string
    request     func() *http.Request
    wantStatus  int
    wantCode    string
    wantDetails bool // assert the "details" key is present (panic row, validation row)
}{
    {"extract_error", "/extract-bad", ..., 400, "BAD_REQUEST", false},
    {"handler_error", "/not-found", ..., 404, "NOT_FOUND", false},
    {"handler_plain_error", "/boom", ..., 500, "INTERNAL", false},
    {"service_validation", "/validate", ..., 400, "VALIDATION_ERROR", true},
    {"service_circuit_breaker_open", "/cb", ..., 503, "SERVICE_UNAVAILABLE", false},
    {"service_timeout", "/slow", ..., 503, "SERVICE_UNAVAILABLE", false},
    {"panic", "/panic", ..., 500, "PANIC", true},
    {"auth_failure", "/secure", ..., 401, "UNAUTHORIZED", false},
    {"rate_limit", "/limited", ..., 429, "TOO_MANY_REQUESTS", false},
}
```

For each case assert: `rec.Code == wantStatus`; `Content-Type` header is
`application/json; charset=utf-8`; the body decodes into `envelope`;
`env.Error.Code == wantCode`; `env.Error.Message != ""`. For `wantDetails`
rows, assert the `details` key is present in the raw JSON (decode into
`map[string]json.RawMessage` to distinguish "absent" from "null").

The circuit-breaker row needs a layer pre-tripped into the open state; the
timeout row needs a `Timeout` layer shorter than the handler's sleep (reuse the
`slowHandler` pattern from `withlayers_test.go`). Drive these through `WithLayers`
so they exercise the real service-layer error path that Task 2 wired.

### Step 4.2 — Handler-Signature Matrix Test

Add `handler_signature_matrix_test.go`. Register every supported shape on one
router and assert each returns 2xx with the expected body. Cover both the
reflection path (`router.Get`/`Post`/`Handle` with `any`) and the typed
constructors. Include the coffee aliases (`Ristretto`, `Solo`, `Doppio`,
`Lungo`) as their own rows so the alias wiring is covered.

The two-extractor rows are conditional on Task 1's chosen approach — write the
assertion to match whichever shipped:

```go
// Approach A (reflection extended):
//   register a reflection-path func(ctx, *R1, *R2) (T, error); assert 200 + both populated.
// Approach B (fail-fast at registration):
//   assert that registering the reflection-path two-extractor func PANICS with
//   the actionable message, and that the typed Lungo equivalent returns 200.
```

Either way, add `TestNoRequestTimeInvalidArgumentPanic`: exhaustively fire one
request at every *successfully registered* shape and assert none produces the
500-from-`"this is a bug"` panic (assert no 500 with an empty/internal body for
shapes that should succeed, and confirm via the recover middleware that the
specific bug-panic string never appears).

### Step 4.3 — Doc/Code Consistency Check

Add `docs_consistency_test.go`. Two complementary guards:

1. **Forbidden-string grep over `docs/`** — fail if any doc file reintroduces a
   removed/false claim. Seed the deny-list with the PR #39 removal:

   ```go
   forbidden := []string{
       // PR #39 removed the false reflection-path two-extractor claim.
       "reflection path supports func(ctx, *Req1, *Req2)",
       // add removed symbols here as they are retired.
   }
   ```

   Walk `docs/` (skip generated/build dirs), read each `.md`, and `t.Errorf`
   on any hit, naming the file and line.

2. **Godoc claim guard** — assert the `Handler` doc comment in `handler.go` does
   NOT claim reflection support for the two-extractor signature (parse the
   doc comment via `go/doc` or a targeted source grep). This locks the PR #39
   godoc fix against reintroduction.

Keep the check stdlib-only and fast (no network, no `go doc` subprocess) so it
runs inside the normal `go test ./...` pass.

## Tests Required

- `TestErrorStatusMatrix` — one sub-test per origin (table above); asserts status + envelope shape + content-type for all 10 rows.
- `TestHandlerSignatureMatrix` — one sub-test per supported reflection/typed/alias shape, including the Task 1 two-extractor outcome row.
- `TestNoRequestTimeInvalidArgumentPanic` — the `"this is a bug"` panic is unreachable at request time for every registered shape.
- `TestDocsConsistency` — forbidden-string grep over `docs/` + godoc claim guard; fails if a removed symbol or false signature claim reappears.
- All run under `go test -race ./... -count=2`.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `go test -race ./... -count=2` clean.
- [x] `golangci-lint run ./...` clean (gocyclo min 15; keep the matrix drivers table-driven so per-function cyclomatic complexity stays under the threshold).
- [x] Project coverage does not regress below the established minimum; the new matrices measurably raise coverage of `writeHandlerError`, the service-layer error mapping, and the unified envelope writer.
- [x] `TestErrorStatusMatrix` includes the `service_timeout` row asserting 503 — confirming (not contradicting) the Task 2 change to `TestWithLayersTyped_WithTimeout`.
- [x] PR description lists the three matrices and notes that this task adds tests only — no production code changes.
