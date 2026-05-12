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
`MustGetState[T]`. Since v1.6: takes `context.Context` (previously `func() T`).

```go
func Ristretto[T any](f func(context.Context) T) http.HandlerFunc
```

Example:

```go
router.Get("/health", espresso.Ristretto(func(ctx context.Context) string {
    state := espresso.MustGetState[AppState](ctx)
    if err := state.DB.PingContext(ctx); err != nil {
        return "db unreachable"
    }
    return "OK"
}))
```

If your handler needs to return an error, use `Doppio` or `HandlerCtx` instead.

### Solo

Single-argument handler (context only):

```go
func Solo[T any](f func(context.Context) T) http.HandlerFunc
```

Example:

```go
router.Get("/time", espresso.Solo(func(ctx context.Context) espresso.Text {
    return espresso.Text{Body: time.Now().String()}
}))
```

### Doppio

Two-argument handler (most common):

```go
func Doppio[T any, Req any](f func(context.Context, *Req) T) http.HandlerFunc
```

Example:

```go
router.Post("/users", espresso.Doppio(createUser))

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    // req.Data contains parsed JSON
    return espresso.JSON[User]{Data: user}, nil
}
```

### Lungo

Three-argument handler (context + two extractors):

```go
func Lungo[T any, Req1 any, Req2 any](f func(context.Context, *Req1, *Req2) (T, error)) http.HandlerFunc
```

Example:

```go
router.Put("/users/{id}", espresso.Lungo(updateUser))

func updateUser(ctx context.Context, path *espresso.Path[UserPath], req *espresso.JSON[UpdateUserReq]) (espresso.JSON[User], error) {
    // path.Data.ID contains path parameter
    // req.Data contains request body
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

### WithAddr

Custom address:

```go
func WithAddr(addr string) ServerOption
```

### WithServer

Custom HTTP server:

```go
func WithServer(srv *http.Server) ServerOption
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