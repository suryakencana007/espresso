---
title: API Reference
description: Espresso API Reference
---

# API Reference

Complete API reference for all Espresso packages.

## Core Packages

| Package | Description |
|---------|-------------|
| [espresso](/api/espresso) | Core - handlers, router, server |
| [extractor](/api/extractor) | Request extractors |
| [middleware/http](/api/middleware-http) | HTTP middleware |
| [middleware/service](/api/middleware-service) | Service layers |
| [openapi](/api/openapi) | OpenAPI 3.0 specification generator |
| [validator](/api/validator) | Struct-tag-driven request validation |
| [pool](/api/pool) | Object pooling |

## Import Paths

```go
import (
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
    "github.com/suryakencana007/espresso/openapi"
    "github.com/suryakencana007/espresso/validator"
    "github.com/suryakencana007/espresso/pool"
)
```

## Core Types

### Router

The main router type that wraps `http.ServeMux`.

```go
type Router struct { ... }
```

Functions:
- `Portafilter() *Router` - Create a new router
- `Use(mw ...func(http.Handler) http.Handler) *Router` - Add HTTP middleware
- `WithState(state any) *Router` - Add application state
- `Get(path string, f any) *Router` - Register GET handler
- `Post(path string, f any) *Router` - Register POST handler
- `Put(path string, f any) *Router` - Register PUT handler
- `Delete(path string, f any) *Router` - Register DELETE handler
- `Patch(path string, f any) *Router` - Register PATCH handler
- `Options(path string, f any) *Router` - Register OPTIONS handler
- `Head(path string, f any) *Router` - Register HEAD handler
- `Brew(opts ...ServerOption)` - Start the server

### Response Types

#### JSON

```go
type JSON[T any] struct {
    StatusCode int
    Data       T
    Cookies    []*http.Cookie  // since v1.5.0
}
```

#### Text

```go
type Text struct {
    StatusCode int
    Body       string
}
```

#### Status

```go
type Status int
```

### State Functions

```go
func GetState[T any](ctx context.Context) (T, error)
func MustGetState[T any](ctx context.Context) T
func WithStateMiddleware(state any) func(http.Handler) http.Handler
```

### Handler Functions

```go
func Ristretto(f func(context.Context) T) http.HandlerFunc  // ctx only, no error
func Solo(f func(context.Context) T) http.HandlerFunc  // 1 arg
func Doppio(f func(context.Context, *Req) T) http.HandlerFunc  // 2 args
func Lungo(f func(context.Context, *Req1, *Req2) (T, error)) http.HandlerFunc  // 3 args (context + 2 extractors)
```

### Error Constructors

```go
func ErrBadRequest(message string) *Error          // 400
func ErrUnauthorized(message string) *Error        // 401
func ErrForbidden(message string) *Error           // 403
func ErrNotFound(message string) *Error            // 404
func ErrConflict(message string) *Error            // 409
func ErrPreconditionFailed(message string) *Error  // 412
func ErrUnprocessableEntity(message string) *Error // 422
func ErrTooManyRequests(message string) *Error     // 429
func ErrInternal(message string) *Error            // 500
func ErrServiceUnavailable(message string) *Error  // 503
func NewError(statusCode int, message string) *Error
func ValidationErrors(errs []ValidationError) *Error
```

The lowercase-prefix forms (`BadRequest`, `Unauthorized`, etc.) and the
`ErrorResponse` alias were removed in v2.0.

### Auto-Validation Hook (since v2.0)

```go
const DefaultHandlerCacheSize = 1024

func SetDefaultValidator(fn func(v any) error)
func DefaultValidator() func(v any) error
func RunDefaultValidator(v any) error
```

When set, every built-in extractor calls the hook after decode; a
non-nil return becomes a structured 400. Default is nil → v1.x behavior.

### Handler-Cache Tuning (since v2.0)

```go
func SetHandlerCacheSize(n int)            // default 1024; pass 0 to reset
func OnHandlerCacheEvict(fn func(reflect.Type))
```

The reflection cache is now LRU-bounded. Static apps unaffected; apps
with dynamic handler registration get an eviction hook for telemetry.

### Service Layer Configs

```go
func Timeout(d time.Duration) LayerConfig
func Logging(logger zerolog.Logger, name string) LayerConfig
func Retry(maxRetries int, backoff time.Duration, strategy BackoffStrategy) LayerConfig
func CircuitBreaker(cfg CircuitBreakerConfig) LayerConfig
func ConcurrencyLimit(maxConcurrent int) LayerConfig
func Metrics(collector MetricsCollector, name string) LayerConfig
func Validation[Req any](validator Validator[Req]) LayerConfig  // generic since v2.0
func CustomLayer(buildFunc func() any) LayerConfig
```

## See Also

- [Handlers Guide](/guide/handlers) - Handler patterns
- [Routing Guide](/guide/routing) - Routing patterns
- [State Guide](/guide/state) - State management
- [v1 → v2 Migration](/migration-v1-to-v2) - Breaking changes and recipes