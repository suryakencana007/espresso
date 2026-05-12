# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Documentation

- **`docs/guide/testing.md`** — new page documenting the recommended
  test-seam pattern for services with unexported function-field
  dispatch. Recommends private functional options keyed through
  `_test.go`-only setters so `*ForTest` setters don't leak into the
  production API surface or appear on `pkg.go.dev`. Includes a "when
  to promote to an interface seam instead" decision rule (4+ stubs
  across test files). Wired into the docs sidebar under Advanced.
  Closes the last open friction item from Barista's feedback log
  (F-08, see [`roadmaps/USAGE_ESPRESSO.md`](roadmaps/USAGE_ESPRESSO.md#f-08))
  — application-layer pattern, framework ships guidance not API.

## [2.1.0] - 2026-05-12

A maintenance release that pays off v2.0's deferred SSE-removal debt
(#30), closes Barista F-02 with a new `WithPreFlight` Stream option
(#33), and ships one validator ergonomics helper (#31). Plus a bench
refresh against current hardware/Go-version (#32) and a v2.0 → v2.1
migration guide (#34). See
[`docs/migration-v2-to-v2.1.md`](docs/migration-v2-to-v2.1.md) for
upgrade recipes.

### Added

- **`validator.AsDefaultValidator()` helper** (v2.1 task-03): returns
  the canonical `func(any) error` adapter for
  `espresso.SetDefaultValidator(...)`. Wraps `validator.Struct` and
  converts the resulting `espresso.FieldErrors` into
  `espresso.ValidationErrors` so failures surface as the framework's
  standard 400 JSON shape. Drops the auto-validate wiring in user
  code from ~10 lines to 1:
  ```go
  func init() {
      espresso.SetDefaultValidator(validator.AsDefaultValidator())
  }
  ```
  Users who need a different error code, extra detail keys, or other
  customization keep writing the inline closure — the helper is the
  most-common-case shortcut, not a configuration surface.

- **`WithPreFlight(fn)` Stream option** (v2.1 task-02, closes Barista
  F-02): SSE handlers can reject a request with a real HTTP 4xx
  *before* the response headers commit. Pass any
  `func(ctx context.Context) error` to `Stream[T]` or `StreamSimple`;
  a non-nil return routes through `writeHandlerError`, so an
  `*espresso.Error` (e.g. `ErrNotFound`, `ErrForbidden`) surfaces with
  its declared status code and the framework's structured JSON
  envelope — not as an `event: error` frame on a 200-OK stream. The
  closure receives the request context and can call
  `MustGetState[T]` / `GetState[T]`. Closes
  [USAGE_ESPRESSO.md F-02](roadmaps/USAGE_ESPRESSO.md#f-02) — the
  Barista per-route `RequireAppAccess` / `RequireDeploymentAccess`
  preflight middleware collapses into a single
  `WithPreFlight(...)` call. Additive: existing `Stream[T]` /
  `StreamSimple` callers see no behavioural change and pay zero
  overhead on the happy path. See
  [`roadmaps/v2.1/tasks/task-02-stream-preflight.md`](roadmaps/v2.1/tasks/task-02-stream-preflight.md).

### Removed (BREAKING)

- **Deprecated SSE types removed from `response.go`** (v2.1 task-01):
  `SSE`, `SSEEvent`, `SSEWriter`, `NewSSEWriter`, and their methods.
  Tagged `// Deprecated:` since v1.3; carry-over from v2.0 task-02
  deferral. Replace with `Stream[T]` / `StreamSimple` and `*SSEStream`:
  ```go
  // Before (v2.0)
  sse := espresso.NewSSEWriter(w)
  sse.Event("update", "hello")

  // After (v2.1)
  router.Get("/stream", espresso.StreamSimple(func(ctx context.Context, s *espresso.SSEStream) error {
      return s.SendText("update", "hello")
  }))
  ```
  Net diff: -185 lines from `response.go`, -178 lines from
  `response_test.go` (ten SSE/SSEWriter tests deleted). The `fmt`
  import is now unused in `response.go` and dropped. Docs API
  reference (`docs/api/espresso.md`) updated — the deprecated
  sections that v2.0 PR #26 left in with warning banners are now
  removed entirely; a one-paragraph forward pointer to the streaming
  guide replaces them.

### Documentation

- **v2.0 → v2.1 migration guide** (v2.1 task-05): new
  [`docs/migration-v2-to-v2.1.md`](docs/migration-v2-to-v2.1.md)
  covering the four user-facing v2.1 deltas — the removed deprecated
  SSE types (PR #30), the new `WithPreFlight` Stream option (PR #33,
  closes Barista F-02), the `validator.AsDefaultValidator()` helper
  (PR #31), and the framework-comparison bench refresh (PR #32). Five-
  minute upgrade checklist at the top in the same style as the v1 → v2
  guide; Before/After recipes for each change. Docs nav
  (`docs/.vitepress/config.ts`) gains a "Migrations" dropdown at the
  top level and a second entry under the "Upgrading" sidebar group so
  readers on either v1.x or v2.0 land in the right guide. README
  `Upgrading` section gains a parallel paragraph for the v2.0 → v2.1
  jump. Cross-references added from `roadmaps/v2.0/README.md` and
  `roadmaps/v2.1/README.md` to the new guide. Pure documentation; no
  behavioural change.

- **Framework comparison benchmarks refreshed against v2.1.x**
  (v2.1 task-04). Re-ran `bench/` (Gin / Echo / Espresso / Fiber across
  static-text, JSON round-trip, and path-parameter scenarios) on
  Go 1.25.6 / AMD Ryzen 7 4800H with
  `go test -bench . -benchmem -benchtime=3s -count=3 -cpu=1`, then
  substituted the mean values into the README's three "Framework
  Comparison" tables. Caption updated to record run conditions
  (hardware, Go version, commit, command line). The new numbers run
  ~2-3x higher than the v1.4 publication across **all four**
  frameworks — this is a benchmark-runner hardware shift (the v1.4
  baseline was an Intel Core Ultra 7 155H), not an Espresso dispatch
  regression. Allocations and B/op are essentially unchanged for the
  competitor frameworks and only marginally up for Espresso, which
  rules out an allocation regression. Added a paragraph to
  `bench/README.md` capturing the hardware-shift explanation so
  future readers don't mistake the absolute numbers for a v2.x
  slowdown. Purely informational; no behavioral change to Espresso.

## [2.0.0] - 2026-05-10

The first major release since v1.0. Bundles five breaking changes and
two additive features that v1.x deferred under its strict
backward-compatibility promise. Every breaking change has a mechanical
migration recipe in
[`docs/migration-v1-to-v2.md`](docs/migration-v1-to-v2.md); the full
upgrade for a typical app fits in one sitting.

### Changed (BREAKING)

- **Module path changed** to
  `github.com/suryakencana007/espresso/v2` per Go's major-version
  module convention. Update imports:

  ```bash
  gofmt -r '"github.com/suryakencana007/espresso" -> "github.com/suryakencana007/espresso/v2"' -w .
  # plus the same rewrite for each subpackage (extractor, middleware/http, etc.) — see migration guide.
  ```

- **`espresso.Validation` is now generic** (v2.0 task-04). Signature
  changed from `Validation(validator any) LayerConfig` to
  `Validation[Req any](validator servicemiddleware.Validator[Req]) LayerConfig`.
  Closes the v1.x footgun where a mismatched validator failed silently
  (the type assertion in `buildLayer` returned a panic only when the
  pipeline actually built; before that, the assertion result was
  invisible). Now: a `Validator[Req1]` applied to a handler with `Req2`
  panics at registration with a descriptive message naming both types.

  Migration: most callers don't need a syntactic change because Go
  infers the type parameter from the argument:

  ```go
  validator := servicemiddleware.ValidatorFunc[*JSON[CreateUserReq]](...)
  Validation(validator)            // Req inferred as *JSON[CreateUserReq]
  Validation[*JSON[CreateUserReq]](validator)  // explicit form, optional
  ```

  The explicit form is recommended for documentation purposes when the
  validator is constructed elsewhere and the call site benefits from a
  visible type at the binding point. New regression-locking test:
  `TestValidation_TypeMismatch_PanicsAtBuild`. Internal `validationConfig`
  is now `validationConfig[Req]` (unexported, no external migration).

### Added

- **Auto-validate on extract** (v2.0 task-05). New package-level hook
  `espresso.SetDefaultValidator(fn func(any) error)`. When set, every
  built-in extractor — `JSON[T]`, `Query[T]`, `Path[T]`, `Form[T]`,
  `Header[T]`, `Cookie[T]`, `XML[T]`, `Multipart[T]`,
  `RawBodyWithHeaders[H]` — calls the hook with a pointer to the decoded
  value at the end of `Extract`. A non-nil error is propagated as an
  extraction failure, which the existing structured-JSON 400 path handles;
  the handler does not run.
  Pair with `validator.Struct` for one-line wiring of struct-tag
  validation:
  ```go
  func init() {
      espresso.SetDefaultValidator(func(v any) error {
          if err := validator.Struct(v); err != nil {
              if fe, ok := err.(espresso.FieldErrors); ok {
                  return espresso.ValidationErrors(fe.ToValidationErrors())
              }
              return err
          }
          return nil
      })
  }
  ```
  Default is nil — v1.x behavior is preserved exactly when the hook is
  unset. Hot-path overhead measured at **2.24 ns/op, 0 allocs** for the
  nil-fast path (`BenchmarkRunDefaultValidator_NilHook`) and 2.62 ns/op
  with a hook installed. `RunDefaultValidator(v any) error` is also
  exported so custom `Extract` methods can opt into the same hook.
  New example at `cmd/example/validate/main.go`. Composable with the
  existing `Validation[Req]` service layer (which runs **after**
  extraction); the two solve different problems and can be used together.

  Implementation note: the hook lives in `internal/validatehook` so both
  the root `espresso` package and the `extractor` subpackage can depend
  on it without forming an import cycle.

- **Handler-reflection cache is now bounded with LRU eviction**
  (v2.0 task-03). Default upper bound is `DefaultHandlerCacheSize`
  (1024). Tuning surface added:
  ```go
  espresso.SetHandlerCacheSize(2048)
  espresso.OnHandlerCacheEvict(func(t reflect.Type) {
      metrics.Inc("handler_cache.evict", "type", t.String())
  })
  ```
  Static apps stay well under the default and never evict — the LRU
  bookkeeping adds ~24 ns/registration (measured) and does not touch
  the per-request hot path. Dynamic-registration scenarios (plugin
  hosts, per-tenant codegen, `reflect.MakeFunc`) now have an upper
  bound on cache memory regardless of churn rate. In-flight requests
  are unaffected by eviction: `*handlerInfo` values are immutable, and
  request-side handlers hold the pointer captured at registration time.
  Replaces the previous unbounded `sync.Map`. New benchmarks:
  `BenchmarkHandlerCache_SteadyState` (24 ns/op, 0 allocs),
  `BenchmarkHandlerCache_Overflow` (224 ns/op for insert+evict+hook).
  This is **purely additive** — apps that don't call the new setters
  see only the bound (1024) and identical hot-path behavior.

### Changed (BREAKING)

- **Stream registries are now per-Router instead of process-global**
  (v2.0 task-01). The package-level `defaultRegistry` (WebSocket) and
  `defaultSSERegistry` (SSE) are removed; each `*Router` owns a
  `wsReg` and `sseReg` initialized in `Portafilter()`. `gracefulShutdown`
  drains the owning Router's registries, so two `Portafilter()` instances
  in the same process now shut down independently — closing router A
  does not touch router B's streams.
  Internally, `*Router.ServeHTTP` injects the registries into the
  request context; `WebSocketSimple` / `WebSocket[T]` / `StreamSimple` /
  `Stream[T]` look them up via `routerRegistriesFrom(ctx)` and register
  the connections they create. If the wrappers are invoked outside a
  Router context (e.g., wired into a non-Espresso mux), registration is
  a silent no-op — the connection still works, but graceful-shutdown
  won't drain it.
  Migration: external callers of the package-level `defaultRegistry` /
  `defaultSSERegistry` must reach into `router.wsReg` / `router.sseReg`
  on a specific `*Router` instance instead. Internal users (handlers
  created via `router.Get(...)` etc.) need no change. Adds
  `TestShutdown_MultiRouter_WebSocketIsolation` and
  `TestShutdown_MultiRouter_SSEIsolation` regression locks.

- **`Ristretto[Res]` signature changed** from `func() Res` to
  `func(context.Context) Res`. Closes Barista F-01 — `Ristretto` was
  pitched as the lightweight health-check primitive but couldn't reach
  state via `MustGetState[T]` since it had no `context.Context`. The new
  signature delegates to `HandlerCtxNoErr` (instead of `HandlerNoReqNoErr`)
  and preserves the no-error character that distinguishes `Ristretto`
  from `Doppio` / `HandlerCtx`.
  Migration: add a `context.Context` parameter to every Ristretto handler.
  If you don't need it, name it `_`:
  ```go
  // Before
  func ping() Text { return Text{Body: "pong"} }
  router.Get("/ping", espresso.Ristretto(ping))

  // After
  func ping(_ context.Context) Text { return Text{Body: "pong"} }
  router.Get("/ping", espresso.Ristretto(ping))
  ```
  If your handler needs to return an error, use `Doppio` or `HandlerCtx`
  instead — `Ristretto` keeps its no-error stance.
  Mechanical search-and-replace isn't safe for this one (function literal
  parameter signature varies); a one-time `go build` flags the call sites.
  If you want a 0-arg handler with no error and no context, use
  `HandlerNoReqNoErr` (still available, unchanged).

### Removed (BREAKING)

- **Legacy error constructors** removed from `error.go`: `BadRequest`,
  `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `InternalError`,
  `ServiceUnavailable`. These were marked "prefer `Err*` for new code"
  in their godoc since v1.0 but lacked the machine-readable
  `// Deprecated:` tag, so v1.x callers got no migration runway. They
  were originally scoped for v2.0 (`roadmaps/v2.0/tasks/task-02`); folded
  forward into pre-v2.0 cleanup. Migration: rename per the table —
  `BadRequest` → `ErrBadRequest`, etc. The replacement constructors
  have shipped since v1.0; mechanical rename via
  `gofmt -r 'espresso.BadRequest(x) -> espresso.ErrBadRequest(x)' -w .`
  (and similarly for the other six).
- **`ErrorResponse` type alias** for `Error` removed. Migration:
  use `*espresso.Error` directly (the alias was a thin pass-through).
- **Dead handlers in `cmd/example/main.go`** (`createUserWithError`,
  `circuitBreakerExample`) removed. They were `//nolint:unused`-flagged
  and used the legacy constructors above.
- **Backward Compatibility section in `docs/error-handling.md`**
  removed — the constructors it described are gone.

### Changed

- **Test coverage table in `README.md` refreshed** with current numbers
  (Root: 80.8%, extractor: 85.5%, middleware/http: 86.9%, validator:
  80.6%, openapi: 77.3%; the latter two were missing from the table).

### Still present, deprecated

The following types remain in v2.0 with `// Deprecated:` markers and are
targeted for removal in v2.1. Existing callers continue to work
unchanged; `staticcheck SA1019` will flag them when you're ready to
migrate.

- `SSE`, `SSEEvent` — use `Stream[T]` / `StreamSimple` and `*SSEStream`.
- `SSEWriter`, `NewSSEWriter` — use `*SSEStream`'s `SendText` / `SendJSON`
  / `SendData` / `Comment`.

## [1.5.0] - 2026-05-10

### Added

- **`JSON[T].Cookies` field** — set HTTP cookies alongside JSON
  responses. Cookies are written via `http.SetCookie` before the
  status header, ensuring `Set-Cookie` lands in the response head.
  Zero-value (`nil`) `Cookies` is byte-identical to v1.4. `Reset()`
  clears the slice for `sync.Pool` reuse. Closes Barista F-05.

- **`extractor.RawBodyWithHeaders[H]`** — new extractor that reads the
  raw request body alongside structured headers in a single pass.
  Designed for webhook receivers that verify HMAC against the
  unparsed payload (GitHub `X-Hub-Signature-256`, GitLab `X-Gitlab-Token`,
  Stripe, Slack, etc.). `H` uses the existing `header:"Name,required"`
  tag convention. `Reset()` releases buffers larger than 64KB to
  prevent pool memory bloat, mirroring `RawBodyExtractor`.
  Closes Barista F-06.

- **`ErrPreconditionFailed(message string)`** — 412 Precondition Failed
  constructor, completing the symmetry with the other status-keyed
  helpers. Use when a request precondition is not met (missing
  prerequisite infrastructure, If-Match mismatch, required feature
  flag disabled). Closes Barista F-07.

- **`/api/login` and `/api/webhook/github` examples** in
  `cmd/example/main.go` demonstrate the refresh-token cookie pattern
  and HMAC-SHA256 webhook verification respectively.

### Migration Notes

v1.5 is **strictly additive**. No v1.4 caller is affected — every
public type, function, and method retains its v1.4 signature and
behavior.

If you wrote a chart-internal `JSONWithCookies[T]` wrapper (the
pattern Barista shipped as `httpx.JSONWithCookies[T]`), you can now
retire it:

```go
// Before
return httpx.JSONWithCookies[Token]{
    Data:    Token{Access: t},
    Cookies: []*http.Cookie{refreshCookie},
}

// After
return espresso.JSON[Token]{
    Data:    Token{Access: t},
    Cookies: []*http.Cookie{refreshCookie},
}
```

If you wrote a custom `webhookRequest`-style extractor for "raw body +
provider header", switch to `extractor.RawBodyWithHeaders[H]`. Define
a struct with `header:"X-Foo,required"` tags and use the generic.

If you used `NewError(http.StatusPreconditionFailed, msg).WithCode(...)`,
swap for `ErrPreconditionFailed(msg)`.

## [1.4.0] - 2026-04-20

### Added

- **`validator/` subpackage** — struct-tag-driven validation via
  `validator.Struct(v any) error`. Returns `espresso.FieldErrors` that flow
  into the existing structured-error pipeline. Built-in rules: `required`,
  `min`, `max` (numeric value or string/slice/map length), `email`, `url`,
  `regex`, `oneof`. Recurses into nested structs, pointers to structs, and
  slices of structs with path tracking. See `docs/guide/validation.md` and
  `docs/api/validator.md`. 16 unit tests.

- **`bench/` framework-comparison module** — separate Go module (replace
  directive back to the parent) so comparison deps don't leak into the main
  module. Head-to-head benchmarks against Gin, Echo, and Fiber on three
  scenarios (static text, JSON round-trip, path parameter). Results tabled
  in `README.md` under "Framework Comparison".

- **`TestWebSocket_GracefulShutdown`** — end-to-end test verifying
  connected WebSocket clients receive close code 1001 via
  `router.gracefulShutdown`.

- **`TestShutdown_WebSocketsClosed`** — mirrors the existing SSE shutdown
  test; covers registry close-all on shutdown.

- **`TestSSE_Stream_StateInjection`** — covers state injection through the
  `Stream[Req]` variant (previously only `StreamSimple` had a state test).

- **`TestWithLayers_ExtractorErrorReturnsStructuredJSON`** — locks the JSON
  error shape so regressing back to `http.Error` text/plain would fail CI.

- **v2.0 roadmap** — `roadmaps/v2.0/` mirroring the v1.3 layout. Scopes
  per-Router registries, deprecated-API removal, handler-cache eviction,
  typed `Validation[Req]`, opt-in auto-validate, migration guide, release.

- **Handler-cache growth documentation** — `handler.go` now explains the
  cache's growth semantics; `docs/performance.md` carries the same under
  "Known Limitations".

### Changed

- **Extractor failures now produce structured JSON error responses**
  instead of `text/plain` via `http.Error()`. Paths affected: `withlayers.go`
  extractor and service-call failures, `sse.go` extract/flusher failures,
  `websocket.go` extract and upgrade failures. Response now uses the same
  shape as handler errors: `{"error":{"code":"BAD_REQUEST",...}}`. Clients
  parsing 4xx bodies as text will need to update — most treat 4xx as error
  regardless of body shape.
- **Unified `serveStream` / `serveStreamSimple`** — the ~90% duplicated
  transport code in `sse.go` is now a single helper that both `Stream[Req]`
  and `StreamSimple` delegate to via a closure.
- **`WS.closed` is now `atomic.Bool`** — previously a plain `bool` read
  without the mutex in two call sites (handler wrapper end-of-func guards).
  `Close` is idempotent via CAS.
- **`WS.Close` is idempotent and always removes from the registry** —
  previously a leaked registry entry when the client disconnected before
  the handler explicitly closed.
- **`WS.readLoop` channel sends guarded by `ctx.Done()`** — previously
  could block indefinitely if the handler already returned and nothing was
  reading `msgCh`.
- **`go.mod` go directive lowered from 1.25.6 to 1.23** — verified
  codebase + full test suite + lint pass under 1.23. Widens supported
  toolchain range.

### Fixed

- **Data race on `WS.closed`** caught by `go test -race`. The two
  plain-bool reads in the handler wrappers now go through the atomic API.
- **Gosec G115** — two test-only `int → rune` overflow conversions in
  `handler_test.go` and `sse_test.go` replaced with `strconv.FormatInt`
  and a slice lookup.

### Removed

- **`(*Router).Routes() []Route` and the `Route` type** — the method
  always returned `nil` (documented as "ServeMux doesn't expose routes").
  No caller inside the repo; any external caller was getting no data.
  Migration: delete the call. If you need route introspection, track it
  yourself at registration time.
- **`closeErr` field on `*WS`** — was set in `readLoop` but never read.
  Unexported, no migration needed.

### Migration Notes

v1.4 is **mostly** backward compatible with v1.3:

- The one wire-format change is extractor failures returning JSON instead
  of text/plain. Handler-returned errors already produced JSON in v1.3;
  this change just brings the extractor path into line. Most clients treat
  4xx as an error regardless of body shape, so no action is usually needed.

- `Routes()` is gone. It returned `nil` in v1.x so no working code
  depended on its output — just remove the call.

To adopt the new validator:

```go
import "github.com/suryakencana007/espresso/validator"

type CreateUserReq struct {
    Name  string `json:"name"  validate:"required,min=3,max=50"`
    Email string `json:"email" validate:"required,email"`
}

// In your handler:
if err := validator.Struct(req.Data); err != nil {
    fe := err.(espresso.FieldErrors)
    return zero, espresso.ValidationErrors(fe.ToValidationErrors())
}
```

See `docs/guide/validation.md` for the full pattern, including service-layer
integration and custom-rule composition.

## [1.3.0] - 2026-04-19

### Added

- **WebSocket handler support** with the new `espresso.WS` type and
  `espresso.WebSocket[T]()` / `espresso.WebSocketSimple()` wrappers. Supports
  text and binary frames, ping/pong keepalive, state injection, context
  cancellation on disconnect, and graceful shutdown integration.
  Uses `github.com/coder/websocket` as the underlying library.
  Includes `cmd/example/websocket/` example.

- **Typed SSE streaming** with the new `espresso.SSEStream` type and
  `espresso.Stream[T]()` / `espresso.StreamSimple()` handlers. Supports
  typed events, JSON streaming, keepalive pings, Last-Event-ID resumption,
  state injection, concurrent-safe writes, and graceful shutdown integration.
  Includes `cmd/example/sse/` example.

- **Structured error responses** with the new `espresso.Error` type and
  fluent builder API (`WithCode`, `WithDetail`, `WithDetails`, `Wrap`).
  Handler errors are now automatically serialized as `{"error": {"code": ..., "message": ..., ...}}`.
  Includes `ErrBadRequest`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`,
  `ErrConflict`, `ErrUnprocessableEntity`, `ErrTooManyRequests`, `ErrInternal`,
  `ErrServiceUnavailable` constructors. Backward-compatible `BadRequest()`,
  `NotFound()`, etc. constructors now return `*Error`.
  `ErrorResponse` is now a type alias for `Error`.
  `RecoverMiddleware` now returns structured JSON with `"PANIC"` code
  and stack trace logging.

- **Graceful shutdown hooks** with `router.OnShutdown(hook)` for registering
  cleanup functions that run before the server stops. Hooks run in registration
  order, each receiving a context with the shutdown timeout. Hook panics and
  errors are logged but don't block subsequent hooks. The shutdown sequence is:
  1) OnShutdown hooks, 2) SSE streams close, 3) WebSockets close (code 1001),
  4) HTTP server stops accepting connections, 5) in-flight requests drain.
  Added `BrewContext(ctx, opts)` for programmatic server lifecycle control.

- **CircuitBreakerError** type for circuit breaker integration with
  `IsCircuitBreakerError()` helper.

- **FieldError/FieldErrors** types for structured validation errors with
  convenience constructors: `RequiredFieldError`, `InvalidTypeError`,
  `RangeError`, `LengthError`, `PatternError`, `CustomValidationError`.

- Context cancellation tests verifying propagation to SSE and WebSocket
  handlers, and goroutine leak detection across 50 connect/disconnect cycles.

- Integration tests (build tag: `integration`) for long-lived SSE and
  WebSocket connection stability under load.

### Changed

- Handler error responses now produce structured JSON
  (`{"error": {"code": ..., "message": ...}}`) instead of plain text via
  `http.Error()`. Handlers returning `*espresso.Error` use the error's
  status code and fields; plain `error` returns produce a generic 500 response.
- `RecoverMiddleware` now returns structured JSON error responses with stack
  trace logging and request ID inclusion, instead of plain text.
- SSE keepalive goroutine now properly synchronizes with the handler goroutine
  to avoid data races.

### Deprecated

- `SSEWriter` low-level API is deprecated in favor of `SSEStream`.
  Will be removed in v2.0. No removal in v1.x.
- `SSE` and `SSEEvent` types in `response.go` are deprecated in favor of
  `SSEStream` and `Event` types in `sse.go`.

### Migration Notes

v1.3 is backward compatible with v1.2. No code changes required to upgrade.

To adopt new features:

- Replace manual SSE handling with `espresso.Stream[T]()` or `espresso.StreamSimple()`
  for better integration, concurrency safety, and graceful shutdown support.
- Use `*espresso.Error` in handlers for consistent JSON error responses.
- Register cleanup hooks with `router.OnShutdown(fn)` for graceful resource cleanup.
- Use `router.BrewContext(ctx, opts)` for programmatic server lifecycle control
  (useful in tests or embedding).

[Unreleased]: https://github.com/suryakencana007/espresso/compare/v1.4.0...HEAD
[1.4.0]: https://github.com/suryakencana007/espresso/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/suryakencana007/espresso/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/suryakencana007/espresso/compare/v1.1.0...v1.2.0

## [1.2.0] - 2025-04-09

### Added

- **Fluent API for OpenAPI** - Modern chainable interface for OpenAPI generation
  - `openapi.New()` as primary entry point (replaces `NewGenerator()`)
  - `Description()` method replaces `SetDescription()`
  - `Server()` method replaces `AddServer()`
  - `Schema()` method for automatic type generation and registration
  - `JSON()` convenience method for spec export
  - Full backward compatibility with deprecated old methods

- **Handler Introspection** - Automatic extraction of handler type information
  - `Introspect()` analyzes handler signatures to detect parameter types
  - `BuildOperation()` creates OpenAPI operations from introspected handlers
  - `GeneratePathParams()`, `GenerateQueryParams()`, `GenerateRequestBody()` helpers
  - Automatic detection of `Path[T]`, `Query[T]`, `JSON[T]`, etc. extractors
  - Support for response type introspection

- **Operation Options** - Fine-grained control over OpenAPI operations
  - `Tags()` for grouping endpoints
  - `Summary()` and `Description()` for documentation
  - `Security()` for authentication requirements
  - `Status()` for response codes
  - `Deprecated()` for marking deprecated endpoints
  - `AddParam()` and `AddResponse()` for custom parameters/responses

- **OpenAPIRouter** - Seamless integration of routing and documentation
  - `espresso.OpenAPI(gen)` wraps router with automatic OpenAPI generation
  - `Get()`, `Post()`, `Put()`, `Delete()`, etc. register both route and OpenAPI docs
  - Automatic introspection of handler parameters and return types
  - Works with Espresso's fluent API and middleware chains

- **ServeOpenAPI Methods** - Clean integration for serving documentation
  - `ServeOpenAPI()` serves OpenAPI spec as JSON
  - `ServeDocs()` serves Scalar UI documentation interface
  - `ServeOpenAPIWithDocs()` serves both in one call
  - All integrated into router middleware chain

- **Comprehensive Tests** - 104 new tests for OpenAPI functionality
  - Tests for introspection and type extraction
  - Tests for operation options and building
  - Tests for OpenAPIRouter integration
  - Tests for ServeOpenAPI methods

### Changed

- **OpenAPI API Improvements** - Better developer experience
  - Old methods (`NewGenerator`, `SetDescription`, `AddServer`) marked as deprecated
  - Backward compatible: deprecated methods still work
  - Chainable methods enable fluent programming style

### Documentation

- Updated `docs/api/openapi.md` with fluent API examples
- Updated `docs/examples/production.md` with new patterns
- Updated `README.md` OpenAPI section
- Updated `cmd/example/main.go` to demonstrate OpenAPIRouter best practices

### Migration Guide

From v1.1.0 to v1.2.0:

```go
// v1.1.0 (still works, but deprecated)
gen := openapi.NewGenerator("My API", "1.0.0")
gen.SetDescription("REST API")
gen.AddServer("http://localhost:8080", "Dev")

// v1.2.0 (recommended)
gen := openapi.New("My API", "1.0.0").
    Description("REST API").
    Server("http://localhost:8080", "Dev")

// Old way: manual registration
router := espresso.Portafilter()
router.Get("/users", getUsers)
http.Handle("/openapi.json", gen.Handler())

// New way: automatic integration
router := espresso.OpenAPI(gen)
router.Get("/users", getUsers, openapi.Tags("users")).
    ServeOpenAPI("/openapi.json").
    ServeDocs("/docs", "/openapi.json").
    Brew()
```

## [1.1.0] - 2025-04-08

### Added
- **Cookie extractor** - `extractor.Cookie[T]` for HTTP cookies
- **File upload extractors** - `extractor.File`, `extractor.Files`, `extractor.Multipart` for file uploads
- **SSE streaming** - `response.SSE` for Server-Sent Events support
- **Authentication middleware** - JWT, BasicAuth, APIKey middlewares in `middleware/http`
  - `JWTMiddleware` with RS256/HS256 support
  - `BasicAuthMiddleware` with user validation
  - `APIKeyMiddleware` with header/query param support
- **OpenAPI generator** - Package `openapi` for OpenAPI 3.0 specification generation
  - `NewGenerator()`, `AddPath()`, `AddSchema()`, `Handler()`
  - `GenerateSchemaFromType()` for automatic schema generation from Go types
- **Scalar UI** - Modern API documentation UI (`ScalarUIHandler`, `ScalarUI`)

### Changed
- Replaced `interface{}` with `any` throughout codebase (Go 1.18+ idiom)

### Documentation
- Added comprehensive examples for file upload, SSE streaming, and authentication
- Added authentication middleware documentation

## [1.0.2] - 2025-01-XX

### Added
- **Lungo handler** - New handler for 3-parameter functions (context + 2 extractors)
  - `HandlerCtxReq1Req2Err[Req1, Req2, Res]` - Typed handler for dual extractors
  - `HandlerCtxReq1Req2[Req1, Req2, Res]` - Variant without error return
  - `Lungo[Req1, Req2, Res]` - Coffee-themed alias (named after "long" espresso)
  - `LungoNoErr[Req1, Req2, Res]` - No-error variant
  - Use case: handlers needing both path parameters AND request body

### Fixed
- Escaped angle brackets in Go comments for Vue parsing
- Fixed documentation sidebar link (handler → espresso)
- Fixed ignore dead links in VitePress config
- Updated documentation for bun instead of npm

### Documentation
- Complete VitePress documentation site with guides, examples, and API reference
- Added Mermaid diagram support
- Added comprehensive examples (basic-api, middleware-stack, state-management, production)

## [1.0.1] - 2024-12-XX

### Added
- Initial VitePress documentation site
- Code Hike integration for syntax highlighting
- Mermaid diagram support

## [1.0.0] - 2024-12-XX

### Added
- Initial release
- Core router with fluent API (`Portafilter()`, `Get()`, `Post()`, `Put()`, `Delete()`, etc.)
- Handler aliases: `Ristretto()`, `Solo()`, `Doppio()`
- Built-in response types: `JSON[T]`, `Text`, `Status`
- State/dependency injection with `WithState()` and `GetState[T]()`
- Extractors: `JSON[T]`, `Query[T]`, `Path[T]`, `Form[T]`, `Header[T]`, `XML[T]`
- HTTP middleware: `RequestIDMiddleware`, `RecoverMiddleware`, `CORSMiddleware`, `CompressMiddleware`, `RateLimitMiddleware`, `AuthMiddleware`
- Service layers: `LoggingLayer`, `TimeoutLayer`, `RetryLayer`, `CircuitBreakerLayer`, `ConcurrencyLimitLayer`, `MetricsLayer`, `ValidationLayer`
- Object pooling: `BufferPool`, `ByteSlicePool`, `StringSlicePool`
- Comprehensive test coverage (78%+)