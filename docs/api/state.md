---
title: State API Reference
description: State management and dependency injection
---

# State API Reference

State management provides Axum-style dependency injection. State is stored
immutably in the request context; there is no built-in synchronization for
mutable state — use `atomic.*`, `sync.Map`, or `sync.RWMutex` inside the
state struct yourself.

## Functions

### GetState

Retrieve state from context. Returns the state value and an `ok` flag:

```go
func GetState[T any](ctx context.Context) (T, bool)
```

`ok` is `false` when state was not injected on the router, or when state was
injected as a different type than `T`. Prefer this form when the caller can
recover from missing state.

### MustGetState

Retrieve state or panic:

```go
func MustGetState[T any](ctx context.Context) T
```

Panics if state is not found or is not of type `T`. Use when state is
guaranteed to exist (i.e. `router.WithState(...)` is always called on the
router mounted for this request).

### FromState

Derive a substate slice from the injected state via a getter function.
Convenient when only a portion of the state is needed:

```go
func FromState[S any, T any](ctx context.Context, get func(S) T) (T, bool)
```

Returns `ok=false` if `S` cannot be recovered from context.

### FromMustState

Panic-variant of `FromState`:

```go
func FromMustState[S any, T any](ctx context.Context, get func(S) T) T
```

### WithStateMiddleware

Create middleware that injects state. Rarely needed directly — prefer
`(*Router).WithState`, which prepends this middleware for you.

```go
func WithStateMiddleware(state any) func(http.Handler) http.Handler
```

## Router Method

### WithState

Inject state into the router. Prepends `WithStateMiddleware(state)` so
state is available in the request context of every route registered on
this router afterwards.

```go
func (r *Router) WithState(state any) *Router
```

**Registration ordering:** middleware is bound at route-registration time.
Call `WithState` **before** registering routes; a `WithState` call after
`Get`/`Post`/… does not retroactively add state to already-registered
routes.

## State Extractor

Type-safe state extraction via the `FromRequest` extractor pattern — useful
when you want the extractor mechanism (composition with `Lungo`, OpenAPI
introspection) instead of `MustGetState[T](ctx)`.

```go
type State[T any] struct {
    Data T
}

func (s *State[T]) Extract(r *http.Request) error
func (s *State[T]) Reset()
```

`Extract` uses a **pointer receiver**, so handlers must take `*State[T]`,
not `State[T]` — a value-typed argument silently no-ops (this is the
same trap that applies to every `FromRequest`; the framework panics at
registration if the argument type does not satisfy the interface).

The `State` extractor is used via `Lungo` when the handler also needs a
body extractor (the reflection path rejects two-extractor signatures by
design; see [Lungo](/api/espresso#lungo)):

```go
router.Post("/things", espresso.Lungo(func(
    ctx context.Context,
    req *espresso.JSON[CreateReq],
    state *espresso.State[AppState],
) (espresso.JSON[Thing], error) {
    return espresso.JSON[Thing]{Data: state.Data.Store.Create(req.Data)}, nil
}))
```

## Usage

### Basic Usage

```go
type AppState struct {
    DB     *sql.DB
    Cache  *redis.Client
    Config Config
}

func main() {
    state := AppState{
        DB:     db,
        Cache:  redisClient,
        Config: config,
    }

    router := espresso.Portafilter().
        WithState(state).
        Get("/users", espresso.Doppio(listUsers))

    router.Brew()
}
```

### Using GetState (ok pattern)

```go
func listUsers(ctx context.Context, req *espresso.JSON[Query]) (espresso.JSON[[]User], error) {
    state, ok := espresso.GetState[AppState](ctx)
    if !ok {
        return espresso.JSON[[]User]{}, espresso.ErrInternal("state not configured")
    }

    users := state.DB.QueryUsers(ctx)
    return espresso.JSON[[]User]{Data: users}, nil
}
```

### Using MustGetState

```go
func getUser(ctx context.Context, req *extractor.Path[UserPath]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)
    user := state.DB.FindUser(req.Data.ID)
    return espresso.JSON[User]{Data: user}, nil
}
```

### Using FromState (substate)

```go
func getUser(ctx context.Context, req *extractor.Path[UserPath]) (espresso.JSON[User], error) {
    db, ok := espresso.FromState(ctx, func(s AppState) *sql.DB { return s.DB })
    if !ok {
        return espresso.JSON[User]{}, espresso.ErrInternal("db not configured")
    }
    return espresso.JSON[User]{Data: db.FindUser(req.Data.ID)}, nil
}
```

### Using the State extractor with Lungo

```go
func handler(
    ctx context.Context,
    req *espresso.JSON[CreateReq],
    state *espresso.State[AppState],
) (espresso.JSON[Response], error) {
    db := state.Data.DB
    // ...
    return espresso.JSON[Response]{Data: resp}, nil
}

router.Post("/things", espresso.Lungo(handler))
```

## Context Key

The context key used internally is an unexported `struct{}` type; there is
no exported symbol you can inject state under manually. Always inject via
`(*Router).WithState` or `WithStateMiddleware`, never via
`context.WithValue` directly.

## See Also

- [State Management Guide](/guide/state) — Detailed usage patterns
- [Examples: State Management](/examples/state-management) — Complete example
