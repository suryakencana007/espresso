# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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