# Task 2: `Stream` Pre-Flight Phase (closes Barista F-02)

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 2-3 days
**Dependencies:** None (builds on v2.0 task-01's `serveStream` restructure)


> **Status: ✅ Shipped 2026-05-12.** Delivered via #33.

## Context

Barista F-02 ([`roadmaps/USAGE_ESPRESSO.md`](../../USAGE_ESPRESSO.md#f-02)) is the v0.2-era observation that `Stream` commits HTTP headers as part of accepting the request, so a "resource not found" decision the handler would like to surface as an HTTP 404 can only become an `event: error` frame on a 200-OK stream. CDNs, proxies, and observability tools don't treat that as a real 4xx; downstream integrators have to special-case it.

Barista's workaround is per-route preflight middleware (`RequireAppAccess`, `RequireDeploymentAccess`) — boilerplate per SSE route plus a middleware per resource kind. F-02 was deferred from v2.0 because v2.0 task-01 (per-Router registries) was already restructuring `serveStream`; this task lands cleanly on top of that work.

## Acceptance Criteria

- [x] SSE handlers can return an `*espresso.Error` before headers commit, surfacing as a real HTTP 4xx response with structured JSON body (matching the rest of the framework's error pipeline).
- [x] Existing `Stream[T]` / `StreamSimple` callers continue to work unchanged.
- [x] The "happy path" (handler accepts the stream and writes events) has zero new overhead.
- [x] A regression test asserts a pre-flight rejection produces a real 4xx with the structured error body — NOT a 200-OK SSE stream containing an `event: error` frame.
- [x] Migration-guide entry in `docs/migration-v2-to-v2.1.md` shows the Barista `RequireAppAccess`-style middleware pattern collapsing into a single pre-flight call.

## Technical Approach

### Step 2.1 — Re-Read the Existing `serveStream` Flow

v2.0 task-01 added `routerRegistriesFrom(ctx)` and refactored `serveStream` to take a `func(*SSEStream) error` closure. The current call shape is roughly:

```go
serveStream(w, r, opts, func(s *SSEStream) error {
    return userHandler(ctx, req, s)  // for Stream[T] variant
})
```

Headers commit inside `serveStream` before the closure runs. F-02's fix needs to add a pre-flight phase **before** the headers commit, where the handler can short-circuit with an `*espresso.Error`.

### Step 2.2 — Pick the API Shape

Three options, document the chosen one in the PR:

**(a) Optional pre-flight return value on the handler signature.**
```go
type StreamHandler[Req any] func(ctx context.Context, req Req, s *SSEStream) error
type StreamPreFlightHandler[Req any] func(ctx context.Context, req Req) (*SSEStream-or-error)
```
Pro: single signature. Con: changes existing handler signatures (breaking).

**(b) Separate `PreFlight` interface** users opt into:
```go
type PreFlightStreamer[Req any] interface {
    PreFlight(ctx context.Context, req Req) error
    Run(ctx context.Context, req Req, s *SSEStream) error
}
```
Pro: explicit. Con: handler authors must define a struct, not a function.

**(c) Composable wrapper** (recommended):
```go
func WithPreFlight[Req any](
    preflight func(ctx context.Context, req Req) error,
    handler   func(ctx context.Context, req Req, s *SSEStream) error,
) func(ctx context.Context, req Req, s *SSEStream) error
```
Pro: additive — doesn't change `Stream[T]` / `StreamSimple` signatures. Both opt-in and clear about intent. Con: nested closures.

The `serveStream` helper internally checks: if a pre-flight closure was registered, run it before committing headers; on non-nil `*espresso.Error`, route through `writeHandlerError(w, r, err)` (the same path JSON handlers use) and return without opening the stream.

### Step 2.3 — Implementation Sketch

In `sse.go`, extend the `streamOpts` (or equivalent options struct) with an optional pre-flight closure:

```go
type streamOpts struct {
    keepAliveInterval time.Duration
    initialRetryHint  time.Duration
    preflight         func(ctx context.Context) error  // new
}

// Public option:
func WithPreFlight(fn func(ctx context.Context) error) StreamOption { ... }
```

Inside `serveStream`, before flushing headers:

```go
if opts.preflight != nil {
    if err := opts.preflight(r.Context()); err != nil {
        writeHandlerError(w, r, err)
        return
    }
}
// ... existing header commit + handler invocation
```

For the typed `Stream[Req]` variant, the pre-flight closure has access to the extracted `Req`. Provide a separate option `WithPreFlightT[Req]` or thread the extraction differently.

### Step 2.4 — Docs

- `docs/streaming.md`: new section "Rejecting requests before the stream opens" with the canonical Barista pattern.
- `docs/api/espresso.md`: `WithPreFlight` documented under the Stream options.
- Migration guide (Task 5): show the `RequireAppAccess`/`RequireDeploymentAccess` middleware collapsing into a single pre-flight call.

## Tests Required

- `TestStream_PreFlightReject_404`: pre-flight returns `ErrNotFound("...")`; assert status 404, JSON body `{"error":{"code":"NOT_FOUND",...}}`, NO `Content-Type: text/event-stream`, NO `event: error` frame in the body.
- `TestStream_PreFlightAccept_HeadersOK`: pre-flight returns nil; stream proceeds normally.
- `TestStream_PreFlightHonorsCtx`: pre-flight closure receives the request's context, can call `MustGetState[T]`.
- `TestStream_NoPreFlight_BackwardCompat`: existing `StreamSimple(handler)` (no pre-flight option) behaves byte-identical to v2.0.
- Run with `-race -count=2`.

## Breaking Changes

Per the chosen design (option c above): **none**. This is purely additive. Existing `Stream[T]` / `StreamSimple` callers see no difference.

If options (a) or (b) are picked, document the breaking change in the migration guide.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `go test -race ./... -count=2` clean.
- [x] `golangci-lint run ./...` clean.
- [x] CHANGELOG `[Unreleased]` → `Added` (or `Changed (BREAKING)` if a non-additive design was chosen).
- [x] Migration-guide entry pairs the Barista preflight-middleware pattern with the new pre-flight call.
- [x] PR description references USAGE_ESPRESSO.md F-02 with a "closed" note that the next release of `roadmaps/USAGE_ESPRESSO.md` should mark accordingly.
