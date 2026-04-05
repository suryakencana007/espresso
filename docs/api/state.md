---
title: State API Reference
description: State management and dependency injection
---

# State API Reference

State management provides Axum-style dependency injection.

## Functions

### GetState

Retrieve state from context:

```go
func GetState[T any](ctx context.Context) (T, error)
```

Returns the state value and an error if:
- State was not injected
- State is not of type T

### MustGetState

Retrieve state or panic:

```go
func MustGetState[T any](ctx context.Context) T
```

Panics if state is not found or wrong type. Use when state is guaranteed to exist.

### WithStateMiddleware

Create middleware that injects state:

```go
func WithStateMiddleware(state any) func(http.Handler) http.Handler
```

## Router Method

### WithState

Inject state into router:

```go
func (r *Router) WithState(state any) *Router
```

## State Extractor

Type-safe state extraction:

```go
type State[T any] struct {
    Data T
}

func (s *State[T]) Extract(ctx context.Context) error
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
        DB:    db,
        Cache: redisClient,
        Config: config,
    }
    
    router := espresso.Portafilter().
        WithState(state).
        Get("/users", espresso.Doppio(listUsers))
    
    router.Brew()
}
```

### In Handlers

Using GetState:

```go
func listUsers(ctx context.Context, req *espresso.JSON[Query]) (espresso.JSON[[]User], error) {
    state, err := espresso.GetState[AppState](ctx)
    if err != nil {
        return espresso.JSON[[]User]{}, err
    }
    
    users := state.DB.QueryUsers(ctx)
    return espresso.JSON[[]User]{Data: users}, nil
}
```

Using MustGetState:

```go
func getUser(ctx context.Context, req *espresso.Path[UserPath]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)
    user := state.DB.FindUser(req.Data.ID)
    return espresso.JSON[User]{Data: user}, nil
}
```

Using State extractor:

```go
func handler(ctx context.Context, req *espresso.JSON[Req], state espresso.State[AppState]) (Response, error) {
    db := state.Data.DB
    // ...
}
```

## Context Key

```go
type stateKey = struct{}

var StateKey = stateKey{}
```

State is stored in context under this key.

## See Also

- [State Management Guide](/guide/state) - Detailed usage patterns
- [Examples: State Management](/examples/state-management) - Complete example