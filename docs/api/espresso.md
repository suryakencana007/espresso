---
title: Espresso API Reference
description: Core package types and functions
---

# Espresso API Reference

Core package provides router, handlers, and response types.

```go
import "github.com/suryakencana007/espresso/v2"
```

## Router

### Portafilter

Create a new router:

```go
func Portafilter() *Router
```

Named after the portafilter in espresso machines.

### Router Type

```go
type Router struct { ... }

func (r *Router) Use(mw ...func(http.Handler) http.Handler) *Router
func (r *Router) WithState(state any) *Router
func (r *Router) Get(path string, f any) *Router
func (r *Router) Post(path string, f any) *Router
func (r *Router) Put(path string, f any) *Router
func (r *Router) Delete(path string, f any) *Router
func (r *Router) Patch(path string, f any) *Router
func (r *Router) Options(path string, f any) *Router
func (r *Router) Head(path string, f any) *Router
func (r *Router) Brew(opts ...ServerOption)
```

## Handlers

### Ristretto

Context-only handler (no request body, no error). Lightweight enough for
health checks while still allowing access to request-scoped state via
`MustGetState[T]`. Since v2.0: takes `context.Context` (previously `func() T`).

```go
func Ristretto[Res IntoResponse](f func(context.Context) Res) http.HandlerFunc
```

`Res` must satisfy `IntoResponse` — a bare `string` return type will not
compile. Use `espresso.Text{Body: "..."}` for plain-text responses.

Example:

```go
router.Get("/health", espresso.Ristretto(func(ctx context.Context) espresso.Text {
    state := espresso.MustGetState[AppState](ctx)
    if err := state.DB.PingContext(ctx); err != nil {
        return espresso.Text{Body: "db unreachable", StatusCode: 503}
    }
    return espresso.Text{Body: "OK"}
}))
```

If your handler needs to return an error, use `Doppio` (or the typed
`HandlerCtxErr[Res]` family) instead. See also `RistrettoNoErr`,
`RistrettoErr` for variants.

### Solo

Single-extractor handler (extractor argument, no context):

```go
func Solo[Req FromRequest, Res IntoResponse](fn func(Req) (Res, error)) http.HandlerFunc
```

`Req` must implement `FromRequest` (typically a pointer to a built-in
extractor like `*espresso.JSON[T]` or `*extractor.Query[T]`). The `error`
return is mandatory — extractor failures surface here, and `nil` succeeds.

Example:

```go
router.Post("/users", espresso.Solo(func(req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    return espresso.JSON[User]{Data: User{ID: 1, Name: req.Data.Name}}, nil
}))
```

For handlers that need `context.Context` (state, tracing, cancellation),
use `Doppio` instead. For a single-argument handler that only needs `ctx`
(no extractor), use `Ristretto`.

### Doppio

Two-argument handler (`context.Context` + one extractor). The most common
shape for request-body-driven endpoints.

```go
func Doppio[Req FromRequest, Res IntoResponse](fn func(context.Context, Req) (Res, error)) http.HandlerFunc
```

The `(Res, error)` return is mandatory. `Req` must implement `FromRequest`
(a pointer to a built-in extractor is the common case).

Example:

```go
router.Post("/users", espresso.Doppio(createUser))

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    // req.Data contains parsed JSON, already auto-validated if a default
    // validator is installed via espresso.SetDefaultValidator.
    return espresso.JSON[User]{Data: user}, nil
}
```

### Lungo

Three-argument handler (`context.Context` + two extractors). This is the
only supported two-extractor path — the reflection path (`router.Post(path, fn)`
without `espresso.Lungo(...)`) rejects two-extractor signatures at registration
by design (fail-fast introduced in v2.2).

```go
func Lungo[Req1 FromRequest, Req2 FromRequest, Res IntoResponse](fn func(context.Context, Req1, Req2) (Res, error)) http.HandlerFunc
```

`LungoNoErr` is the no-error variant for handlers that cannot fail.

Example:

```go
router.Put("/users/{id}", espresso.Lungo(updateUser))

func updateUser(
    ctx context.Context,
    path *extractor.Path[UserPath],
    req *espresso.JSON[UpdateUserReq],
) (espresso.JSON[User], error) {
    // path.Data.ID contains the path parameter
    // req.Data contains the request body
    return espresso.JSON[User]{Data: User{ID: path.Data.ID, Name: req.Data.Name}}, nil
}
```

## Response Types

### JSON

JSON response. Doubles as a request extractor (`Extract` decodes the body into `Data`).
The `Cookies` field, if non-empty, writes `Set-Cookie` headers via `http.SetCookie`
before the status header is committed — required so cookies land in the response head.

```go
type JSON[T any] struct {
    StatusCode int
    Data       T
    Cookies    []*http.Cookie  // since v1.5.0
}

func (j JSON[T]) WriteResponse(w http.ResponseWriter) error
func (j *JSON[T]) Extract(r *http.Request) error
func (j *JSON[T]) Reset()
```

Zero-value (`nil`) `Cookies` produces byte-identical output to v1.4 — no
`Set-Cookie` header is emitted. See [Setting Cookies on JSON Responses](../guide/response.md#setting-cookies-on-json-responses)
for the refresh-token pattern.

### Text

Plain text response:

```go
type Text struct {
    StatusCode int
    Body       string
}

func (t Text) WriteResponse(w http.ResponseWriter) error
func (t *Text) Reset()
```

### Status

Status-only response:

```go
type Status int

func (s Status) WriteResponse(w http.ResponseWriter) error
func (s *Status) Reset()
```

For Server-Sent Events streaming, use `Stream[T]` / `StreamSimple` and
the `*SSEStream` API documented in [Streaming](../streaming.md). The
legacy `SSE`, `SSEEvent`, `SSEWriter`, and `NewSSEWriter` types were
removed in v2.1.0 (deprecated since v1.3).

### Stream options

`Stream[T]` and `StreamSimple` accept a variadic `...StreamOption` that
configures the SSE transport. All options are additive — omit them and
the v2.0 stream flow is unchanged.

```go
type StreamOption func(*streamConfig)

func WithKeepAlive(interval time.Duration) StreamOption
func WithRetryHint(d time.Duration) StreamOption
func WithPreFlight(fn func(ctx context.Context) error) StreamOption
```

| Option | Purpose |
|--------|---------|
| `WithKeepAlive(interval)` | Send periodic `: keepalive` comment frames so proxies don't drop the idle connection. Set to `0` (default) to disable. |
| `WithRetryHint(d)` | Emit an initial `retry: <ms>` field so EventSource clients use the given reconnection delay. |
| `WithPreFlight(fn)` | Run an authorization / resource-existence check **before** the SSE response headers commit. Returning a non-nil error short-circuits the stream and routes through the standard JSON error pipeline — an `*espresso.Error` surfaces as a real HTTP 4xx with the framework's structured envelope, not an `event: error` frame on a 200-OK stream. The closure receives the request context, so it can call `MustGetState[T]` / `GetState[T]`. Added in v2.1 to close [USAGE_ESPRESSO.md F-02](../../roadmaps/USAGE_ESPRESSO.md#f-02). |

See [Streaming → Rejecting requests before the stream opens](../streaming.md#rejecting-requests-before-the-stream-opens) for the canonical `WithPreFlight` pattern.

## Error Constructors

Structured errors use `*espresso.Error`. The `Err*` family covers
common HTTP statuses; use `NewError(status, msg)` for codes not in the
list. Each constructor seeds a default machine-readable code (e.g.
`BAD_REQUEST`); override with `.WithCode(...)`.

```go
func ErrBadRequest(message string) *Error          // 400 BAD_REQUEST
func ErrUnauthorized(message string) *Error        // 401 UNAUTHORIZED
func ErrForbidden(message string) *Error           // 403 FORBIDDEN
func ErrNotFound(message string) *Error            // 404 NOT_FOUND
func ErrConflict(message string) *Error            // 409 CONFLICT
func ErrPreconditionFailed(message string) *Error  // 412 PRECONDITION_FAILED
func ErrUnprocessableEntity(message string) *Error // 422 UNPROCESSABLE_ENTITY
func ErrTooManyRequests(message string) *Error     // 429 TOO_MANY_REQUESTS
func ErrInternal(message string) *Error            // 500 INTERNAL
func ErrServiceUnavailable(message string) *Error  // 503 SERVICE_UNAVAILABLE

func NewError(statusCode int, message string) *Error
func ValidationErrors(errs []ValidationError) *Error
```

Builder methods on `*Error`:

```go
(*Error) WithCode(code string) *Error
(*Error) WithDetail(key string, value any) *Error
(*Error) WithDetails(details map[string]any) *Error
(*Error) WithRequestID(id string) *Error
(*Error) Wrap(err error) *Error
```

::: tip Migrating from v1.x
The lowercase-prefix forms (`BadRequest`, `Unauthorized`, etc.) and the
`ErrorResponse` type alias were removed in v2.0. See the
[v1 → v2 migration guide](../migration-v1-to-v2.md#removed-legacy-error-constructors).
:::

## Auto-Validation Hook

Opt-in validation that runs after every successful extraction. When set,
every built-in extractor (`JSON[T]`, `Query[T]`, `Path[T]`, `Form[T]`,
`Header[T]`, `Cookie[T]`, `XML[T]`, `Multipart[T]`,
`RawBodyWithHeaders[H]`) calls the hook with a pointer to the decoded
value; a non-nil error becomes a structured 400 response and the
handler does not run.

```go
const DefaultHandlerCacheSize = 1024

func SetDefaultValidator(fn func(v any) error)
func DefaultValidator() func(v any) error
func RunDefaultValidator(v any) error
```

Default is nil — extractors behave identically to v1.x. Hot path is one
atomic load (~2.24 ns/op, 0 allocations) when the hook is unset. Pair
with the bundled struct-tag validator:

```go
import (
    "github.com/suryakencana007/espresso/v2"
    "github.com/suryakencana007/espresso/v2/validator"
)

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

`RunDefaultValidator` is exposed so custom `Extract` implementations can
opt in to the same hook. See [Auto-Validate on Extract](../guide/validation.md#auto-validate-on-extract-since-v20).

## Handler Cache

Reflection-based handler registration (`Handler()` / `router.Handle()` /
`WithLayers()`) caches per-handler analysis in a process-global LRU
cache. Default upper bound is `DefaultHandlerCacheSize` (1024).

```go
func SetHandlerCacheSize(n int)
func OnHandlerCacheEvict(fn func(reflect.Type))
```

`SetHandlerCacheSize(0)` resets to the default. `OnHandlerCacheEvict(nil)`
clears the hook. Both calls are concurrent-safe; the hook fires
synchronously per eviction outside the cache mutex.

Hot-path cost on a cache hit is ~24 ns/op (mutex + map lookup + LRU
promotion). Static apps stay well under the bound and never evict; apps
that synthesize handler types at runtime (plugin hosts, per-tenant
codegen, `reflect.MakeFunc`) get a memory ceiling regardless of churn.
See [Handler-Reflection Cache](../performance.md#handler-reflection-cache).

## Service Layer Configs

Each layer is constructed via a typed config function returning
`LayerConfig`. Apply via `WithLayers(handler, layers...)`.

```go
func Timeout(d time.Duration) LayerConfig
func Logging(logger zerolog.Logger, serviceName string) LayerConfig
func Retry(maxRetries int, initialBackoff time.Duration, strategy servicemiddleware.BackoffStrategy) LayerConfig
func CircuitBreaker(cfg servicemiddleware.CircuitBreakerConfig) LayerConfig
func ConcurrencyLimit(maxConcurrent int) LayerConfig
func Metrics(collector servicemiddleware.MetricsCollector, serviceName string) LayerConfig
func Validation[Req any](validator servicemiddleware.Validator[Req]) LayerConfig
func CustomLayer(buildFunc func() any) LayerConfig
```

`Validation` is generic since v2.0: a mismatched validator (one whose
`Req` type doesn't match the handler's request type) panics at
registration time with a descriptive message. See
[`Validation` Is Now Generic](../migration-v1-to-v2.md#validation-is-now-generic-validationreq).

## Server Options

Passed to `router.Brew(opts...)` or `router.BrewContext(ctx, opts...)` to
tune the embedded `http.Server`.

### WithAddr

Address to bind. Defaults to `":8080"`.

```go
func WithAddr(addr string) ServerOption
```

### WithReadTimeout

Maximum duration for reading the entire request, including the body.
Defaults to 30 seconds.

```go
func WithReadTimeout(d time.Duration) ServerOption
```

### WithReadHeaderTimeout

Maximum duration allowed for reading request headers. Defaults to
10 seconds. Set independently of `WithReadTimeout` when large bodies
are legitimate but slow clients still need to be cut off.

```go
func WithReadHeaderTimeout(d time.Duration) ServerOption
```

### WithWriteTimeout

Maximum duration before timing out writes of the response. Defaults to
10 seconds. **Note:** long-lived SSE streams are affected — a
follow-up (v2.4 task-04b) will make SSE handlers automatically clear
this per-connection.

```go
func WithWriteTimeout(d time.Duration) ServerOption
```

### WithIdleTimeout

Maximum duration to wait for the next request on a keep-alive
connection. Defaults to 120 seconds.

```go
func WithIdleTimeout(d time.Duration) ServerOption
```

### WithShutdownTimeout

Maximum duration `gracefulShutdown` will wait for in-flight requests
to drain after `Brew`/`BrewContext` begins shutdown. Defaults to
10 seconds.

```go
func WithShutdownTimeout(d time.Duration) ServerOption
```

## Server Lifecycle

### Brew

Blocks until the server receives `SIGINT`, `SIGTERM`, or `SIGQUIT`,
then runs the full graceful shutdown sequence: registered `OnShutdown`
hooks (in order) → SSE registry close → WebSocket close (code 1001) →
`http.Server.Shutdown` (drain up to `ShutdownTimeout`).

```go
func (r *Router) Brew(opts ...ServerOption)
```

### BrewContext

Programmatic variant of `Brew` — shuts down when `ctx` is canceled,
then runs the same sequence with a fresh timeout (uncancelled parent
via `context.WithoutCancel`, so hooks and `Shutdown` see a live ctx).
Returns the first server error, or `nil` on clean shutdown.

```go
func (r *Router) BrewContext(ctx context.Context, opts ...ServerOption) error
```

Idiomatic usage with signal-driven shutdown:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
if err := router.BrewContext(ctx, espresso.WithAddr(":8080")); err != nil {
    slog.Error("server error", "error", err)
}
```

### OnShutdown

Registers a hook that runs during graceful shutdown, before SSE/WS
close and `http.Server.Shutdown`. Hooks run in registration order;
each is panic-recovered and error-logged so one bad hook does not
prevent the others. Use for closing databases, flushing metrics,
draining queues.

```go
func (r *Router) OnShutdown(hook ShutdownHook) *Router
type ShutdownHook func(ctx context.Context) error
```

Example:

```go
router.OnShutdown(func(ctx context.Context) error {
    return db.Close(ctx)
})
```

## Interfaces

### IntoResponse

Response types implement this interface:

```go
type IntoResponse interface {
    WriteResponse(w http.ResponseWriter) error
}
```

### FromRequest

Request extractors implement this interface:

```go
type FromRequest interface {
    Extract(r *http.Request) error
}
```

## See Also

- [Handlers Guide](/guide/handlers) - Handler patterns
- [Routing Guide](/guide/routing) - Routing patterns
- [Response Types Guide](/guide/response) - Response types