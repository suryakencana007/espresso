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
| [pool](/api/pool) | Object pooling |

## Import Paths

```go
import (
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
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
func Ristretto(f func() T) http.HandlerFunc  // 0 args
func Solo(f func(context.Context) T) http.HandlerFunc  // 1 arg
func Doppio(f func(context.Context, *Req) T) http.HandlerFunc  // 2 args
func Lungo(f func(context.Context, *Req1, *Req2) (T, error)) http.HandlerFunc  // 3 args (context + 2 extractors)
```

## See Also

- [Handlers Guide](/guide/handlers) - Handler patterns
- [Routing Guide](/guide/routing) - Routing patterns
- [State Guide](/guide/state) - State management