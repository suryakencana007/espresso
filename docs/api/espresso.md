---
title: Espresso API Reference
description: Core package types and functions
---

# Espresso API Reference

Core package provides router, handlers, and response types.

```go
import "github.com/suryakencana007/espresso"
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

Zero-argument handler:

```go
func Ristretto[T any](f func() T) http.HandlerFunc
```

Example:
```go
router.Get("/health", espresso.Ristretto(func() string {
    return "OK"
}))
```

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

### Trio

Three-argument handler:

```go
func Trio[T any, Req1 any, Req2 any](f func(context.Context, *Req1, *Req2) T) http.HandlerFunc
```

Example:
```go
router.Put("/users/{id}", espresso.Trio(updateUser))

func updateUser(ctx context.Context, path *espresso.Path[UserPath], req *espresso.JSON[UpdateUserReq]) (espresso.JSON[User], error) {
    // path.Data.ID contains path parameter
    // req.Data contains request body
}
```

## Response Types

### JSON

JSON response:

```go
type JSON[T any] struct {
    StatusCode int
    Data       T
}

func (j JSON[T]) WriteResponse(w http.ResponseWriter) error
func (j *JSON[T]) Extract(r *http.Request) error
func (j *JSON[T]) Reset()
```

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

Response types implement this:

```go
type IntoResponse interface {
    WriteResponse(w http.ResponseWriter) error
}
```

### FromRequest

Request extractors implement this:

```go
type FromRequest interface {
    Extract(r *http.Request) error
}
```

## See Also

- [Handlers Guide](/guide/handlers) - Handler patterns
- [Routing Guide](/guide/routing) - Routing patterns
- [Response Types Guide](/guide/response) - Response types