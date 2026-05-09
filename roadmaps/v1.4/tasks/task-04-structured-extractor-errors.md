# Task 4: Structured JSON Error Responses for Extractor Failures

**Priority:** 🟡 P1 — Hardening
**Estimated Effort:** 1 day
**Dependencies:** v1.3 `*Error` and `writeHandlerError` plumbing

## Context

v1.3 introduced structured `*espresso.Error` JSON responses for handler-returned errors. The shape is:

```json
{"error":{"code":"BAD_REQUEST","message":"…","details":{…},"request_id":"…"}}
```

But the **extractor** path — failures in `req.Extract(r)` and in service-call layers before the user's handler runs — still fell through to `http.Error(w, err.Error(), 400)`. That produces text/plain. Clients parsing the body as JSON broke; clients tolerant of either format treated extractor errors specially because they had to.

This task brings the extractor path into line with the handler path. One JSON shape, all error sources.

## Acceptance Criteria

- [x] Every `http.Error()` call in `withlayers.go`, `sse.go`, and `websocket.go` replaced with a structured-JSON helper.
- [x] Extractor failures emit `{"error":{"code":"BAD_REQUEST",...}}` with the same shape as handler errors.
- [x] Service-call layer failures (timeout, retry exhaustion, circuit-breaker open) emit the same shape.
- [x] SSE extractor failures and flusher-not-supported failures emit the same shape.
- [x] WebSocket extractor failures and upgrade failures emit the same shape.
- [x] HTTP status codes preserved across the change (400 stays 400, 503 stays 503).
- [x] Regression-locking test added: `TestWithLayers_ExtractorErrorReturnsStructuredJSON`.
- [x] CHANGELOG **Migration Notes** entry calls out the wire-format change.

## Technical Approach

### Step 4.1: Audit

Grep for `http.Error(` across `withlayers.go`, `sse.go`, `websocket.go`. Each hit is a wire-format mismatch with the handler-error path. List them and map each to the appropriate `*espresso.Error` constructor:

| Site | Trigger | Replacement |
|------|---------|-------------|
| `withlayers.go` extractor failure | `req.Extract(r) err != nil` | `ErrBadRequest(err.Error()).WithCause(err)` |
| `withlayers.go` service-call failure | service returns non-`*Error` | `ErrInternal(err.Error()).WithCause(err)` |
| `sse.go` extract | `req.Extract(r) err != nil` | `ErrBadRequest(...)` |
| `sse.go` flusher missing | `_, ok := w.(http.Flusher)` | `ErrInternal("streaming not supported")` |
| `websocket.go` extract | `req.Extract(r) err != nil` | `ErrBadRequest(...)` |
| `websocket.go` upgrade | `websocket.Accept` err | `ErrBadRequest("upgrade failed").WithCause(err)` |

### Step 4.2: Helper

Reuse the existing `writeExtractError(w, r, err)` helper from `handler.go`. If it doesn't fit, factor a sibling `writeServiceError(w, r, err)`.

### Step 4.3: Regression test

```go
func TestWithLayers_ExtractorErrorReturnsStructuredJSON(t *testing.T) {
    // POST /users with malformed JSON body
    // Expect 400, Content-Type: application/json
    // Body: {"error":{"code":"BAD_REQUEST",...}}
}
```

This test exists specifically so a future "let's just use http.Error" PR turns red in CI.

### Step 4.4: Migration note

Add a Migration Notes section to the v1.4 CHANGELOG entry (Task 10) explaining:

- 4xx body shape changed from text/plain to JSON.
- HTTP status code unchanged.
- Most clients treat 4xx as an error regardless of body shape; no action usually needed.
- For clients explicitly parsing 4xx as text, switch to JSON parsing.

## Tests Required

- `TestWithLayers_ExtractorErrorReturnsStructuredJSON` — locks the JSON shape.
- Existing tests that asserted text/plain bodies on extractor errors must be updated to assert JSON.

## Definition of Done

- [x] `grep -n 'http.Error(' withlayers.go sse.go websocket.go` returns zero hits.
- [x] All updated tests pass.
- [x] `go test ./... -race` clean.
- [x] CHANGELOG `[Unreleased]` → `Changed` entry written.
- [x] CHANGELOG `[Unreleased]` → `Migration Notes` entry written.
