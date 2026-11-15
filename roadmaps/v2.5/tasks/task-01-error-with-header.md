# Task 1: `Error.WithHeader` / `WithHeaderAdd` on the handler error path

**Priority:** 🔴 P0 — feature
**Estimated Effort:** 1.5 days
**Dependencies:** None (Task 4 for CI visibility)

> **Status: ✅ Shipped 2026-11-15 (v2.5.0).** Delivered via #79, closing
> issue #77.

## Context

Handlers return `(context.Context, Req) (Res, error)` and never receive the
`http.ResponseWriter`. `writeHandlerError` (`error.go`) answers a returned
error by `errors.As`-ing a `*espresso.Error` and calling `writeErrorResponse`,
which delegates to `internal/errorenvelope.Write` — a writer that sets only
`Content-Type`. There was no way for a handler to attach a response header to
an error, and a custom error type implementing `WriteResponse` was not
consulted on the error path.

Issue #77 is the concrete case: `tamper` maps its identity rate limiter's
`ThrottledError` to `ErrTooManyRequests("too many attempts")`, and a `429`
should carry `Retry-After` (RFC 9110 §10.2.3). tamper's middleware path can
set it (it sees the writer); the handler path could not, so it fell back to a
`retry_after_seconds` body detail and recorded the gap
(suryakencana007/tamper#163).

## Acceptance Criteria

- [x] A handler returning
      `espresso.ErrTooManyRequests("…").WithHeader("Retry-After", "30")`
      yields status `429` with the header present.
- [x] With no header set, the response is byte-identical to before.
- [x] `writeHandlerError`'s unknown-error → 500 fallback is unchanged.
- [x] Reserved framing and hop-by-hop headers are ignored.
- [x] A repeatable-header variant exists (added after the issue's review):
      `WithHeaderAdd`, mirroring `http.Header.Add`.

## Technical Approach

### Step 1.1 — Store headers on `*Error`

Add an unexported `headers http.Header` field to `Error`. The struct already
carried `Details map[string]any`, so it was already non-comparable; the new
field changes nothing about equality or JSON (unexported fields are skipped).

```go
headers http.Header
```

### Step 1.2 — Builder methods

```go
func (e *Error) WithHeader(key, value string) *Error    // Set semantics
func (e *Error) WithHeaderAdd(key, value string) *Error // Add semantics
```

Both reject reserved keys via `isReservedHeader` and lazily allocate the map.
`WithHeader` matches `WithCode`/`WithDetail` (last write wins); `WithHeaderAdd`
mirrors `http.Header.Add` for repeatable headers. A later `WithHeader`
collapses values accumulated by `WithHeaderAdd`.

### Step 1.3 — Reserved-header denylist

Reject framing + hop-by-hop headers, case-insensitively (trim + lower):
`content-type`, `content-length`, `transfer-encoding`, `connection`,
`keep-alive`, `proxy-authenticate`, `proxy-authorization`, `te`, `trailer`,
`upgrade`. `Content-Type` stays writer-owned regardless: `applyErrorHeaders`
runs before `errorenvelope.Write`, which `Set`s it afterward.

### Step 1.4 — Apply on the error path

```go
func writeErrorResponse(w http.ResponseWriter, r *http.Request, err *Error) {
	applyErrorHeaders(w, err)
	errorenvelope.Write(w, err.StatusCode, errorBody{ /* … */ })
}
```

Applying in `writeErrorResponse` (before the status line) covers every error
origin: `Error.WriteResponse`, `writeHandlerError`, translated service-layer
errors, and the SSE/WebSocket setup paths. No other writer changes.

## Tests Required

- `TestError_WithHeader_WriteResponse` — direct builder.
- `TestError_WithHeader_HandlerReturnsError` — `writeHandlerError`.
- `TestError_WithHeader_EndToEnd` — real `Portafilter()` / `ServeHTTP`.
- `TestError_WithHeader_Replace` — Set semantics.
- `TestError_WithHeaderAdd_MultipleValues` — two `WWW-Authenticate` + two `Set-Cookie`.
- `TestError_WithHeaderAdd_SetAfterAddReplaces` — Set collapses Add.
- `TestError_WithHeaderAdd_ReservedIgnored`.
- `TestError_WithHeader_ReservedIgnored` — table of 10 reserved headers.
- `TestError_WithHeader_NoHeaderUnchanged` — exact-body byte identity.

## Definition of Done

- [x] All acceptance criteria met.
- [x] `go test -race`. `go vet`. `TestDocsSnippetsCompile` green.
- [x] `docs/api/espresso.md` and `docs/error-handling.md` document both methods.
- [x] CHANGELOG `[2.5.0]` `Added` entry references #77.
