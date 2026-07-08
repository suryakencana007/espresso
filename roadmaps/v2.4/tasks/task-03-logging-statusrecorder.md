# Task 3: LoggingMiddleware statusRecorder — Flusher/Hijacker/Unwrap Forwarding

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 0.5 day
**Dependencies:** None

> **Status: ✅ Shipped 2026-07-05 (v2.4.0).** Delivered via #61 — LoggingMiddleware statusRecorder forwards Flusher/Hijacker/Push/Unwrap — SSE and WS routes work through the middleware again.

## Context

`LoggingMiddleware`'s `statusRecorder` (`middleware/http/middleware.go:472-501`) embeds `http.ResponseWriter` and overrides only `WriteHeader`. It implements neither `http.Flusher`, `http.Hijacker`, nor `Unwrap()` for `http.ResponseController`. Router middleware wraps every route (`router.go:208-214`), so with `LoggingMiddleware` installed:

- SSE routes hit the `w.(http.Flusher)` assertion at `sse.go:346-350` and return `500 {"error":{"code":"INTERNAL","message":"streaming not supported"}}`.
- `coder/websocket`'s `Accept` cannot hijack through the wrapper; WS upgrades fail with a non-101 response (empirically 501 on `coder/websocket v1.8.14`).

Two first-party features (`LoggingMiddleware` + `StreamSimple`/`WebSocketSimple`) are **mutually exclusive today** on any router that mounts both.

The `gzipResponseWriter` in the same file (`middleware.go:210-229`) forwards `Flush/Hijack/Push` correctly — the inconsistency is the tell that `statusRecorder` was an oversight, not a design decision.

## Acceptance Criteria

- [x] With `LoggingMiddleware` installed on a router, an SSE `StreamSimple` route returns `200 Content-Type: text/event-stream` (not 500).
- [x] With `LoggingMiddleware` installed, a WebSocket `WebSocketSimple` route upgrades to `101 Switching Protocols` (not 501).
- [x] `statusRecorder` forwards `Flush()`, `Hijack()`, `Push()`, and implements `Unwrap() http.ResponseWriter` for `http.ResponseController`.
- [x] `statusRecorder` records status on first `Write()` when `WriteHeader` was never called (default 200), so the log's status field reflects the actual response.
- [x] No public API signature change; `LoggingMiddleware` remains `func LoggingMiddleware(logger zerolog.Logger) espresso.Middleware`.

## Technical Approach

### Step 3.1 — Reproduce the SSE break

Add a regression test that installs `LoggingMiddleware` and hits an SSE route:

```go
func TestLoggingMiddleware_SSECompatible(t *testing.T) {
    r := espresso.Portafilter()
    r.Use(httpmiddleware.LoggingMiddleware(zerolog.Nop()))
    r.Get("/events", espresso.StreamSimple(func(ctx context.Context, s *espresso.SSEStream) error {
        return s.Send(espresso.Event{Data: "hi"})
    }))
    // Pre-fix: 500 INTERNAL "streaming not supported"
    // Post-fix: 200 text/event-stream
    srv := httptest.NewServer(r); defer srv.Close()
    resp, err := http.Get(srv.URL + "/events")
    if err != nil { t.Fatal(err) }
    if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
        t.Fatalf("SSE broke: status=%d content-type=%q", resp.StatusCode, resp.Header.Get("Content-Type"))
    }
}
```

Confirm it returns 500 pre-fix. Same shape for `TestLoggingMiddleware_WebSocketCompatible`.

### Step 3.2 — Extend statusRecorder

Mirror `gzipResponseWriter` (`middleware.go:210-229`):

```go
type statusRecorder struct {
    http.ResponseWriter
    status  int
    written bool
}

func (r *statusRecorder) WriteHeader(code int) {
    if !r.written { r.status = code; r.written = true }
    r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
    if !r.written { r.status = http.StatusOK; r.written = true }
    return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Flush() {
    if f, ok := r.ResponseWriter.(http.Flusher); ok { f.Flush() }
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
    if h, ok := r.ResponseWriter.(http.Hijacker); ok { return h.Hijack() }
    return nil, nil, fmt.Errorf("http.Hijacker not supported by underlying ResponseWriter")
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
    if p, ok := r.ResponseWriter.(http.Pusher); ok { return p.Push(target, opts) }
    return http.ErrNotSupported
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
```

`Unwrap()` is what `http.ResponseController` uses to walk wrapper chains (Go 1.20+); implementing it means future callers using the controller instead of type-asserting still work.

### Step 3.3 — Verify SetWriteDeadline flow

`http.NewResponseController(w).SetWriteDeadline(...)` (used by Task 4's SSE fix) walks the wrapper chain via `Unwrap()`. Add a smoke test that `SetWriteDeadline` through `statusRecorder` succeeds (does not return `errors.ErrUnsupported`).

## Tests Required

- `TestLoggingMiddleware_SSECompatible`: SSE route through the middleware returns 200 text/event-stream.
- `TestLoggingMiddleware_WebSocketCompatible`: WS upgrade returns 101.
- `TestLoggingMiddleware_RecordsStatusOnDefaultWrite`: a handler that only calls `w.Write(b)` (no explicit `WriteHeader`) is logged with `status=200`.
- `TestLoggingMiddleware_UnwrapWorks`: `http.NewResponseController(w).SetWriteDeadline(...)` succeeds through the wrapper.
- Run with `-race`.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `go test -race ./middleware/http/... ./... -count=2` clean.
- [x] `golangci-lint run ./...` clean.
- [x] CI's `Test (race)` job green on the PR.
- [x] CHANGELOG `[Unreleased]` entry under `Fixed`: `LoggingMiddleware`'s response-writer wrapper now forwards `Flusher`/`Hijacker`/`Pusher` and provides `Unwrap()` for `http.ResponseController`, restoring SSE and WebSocket compatibility.
- [x] No public API signature changed.
