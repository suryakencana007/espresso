# State & Dependency Injection

Espresso provides **Axum-style state management** with Go-idiomatic `context.Context` optimization.

## Overview

State is immutable and thread-safe - perfect for application-wide dependencies like databases, configuration, and loggers.

<Mermaid source="graph TB
    Router[Router.WithState] -->|inject| MW[Middleware]
    MW -->|context.WithValue| H[Handler]
    H -->|GetState T| S[AppState]
    S --> DB[Database]
    S --> Config[Config]
    S --> Logger[Logger]
    subgraph Immutable[Immutable State]
        DB
        Config
        Logger
    end" />

## Quick Start

### 1. Define State

```go
type AppState struct {
    DB     *sql.DB
    Config Config
    Logger zerolog.Logger
}

type Config struct {
    Port int
    Env  string
}
```

### 2. Inject State

```go
func main() {
    appState := AppState{
        DB:     db,
        Config: config,
        Logger: logger,
    }

    espresso.Portafilter().
        WithState(appState).                    // Inject state
        Get("/users", espresso.Doppio(getUsers)).
        Brew(espresso.WithAddr(":8080"))
}
```

### 3. Access in Handlers

```go
func getUsers(ctx context.Context, req *extractor.Path[UserReq]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)
    
    // Use state
    users := state.DB.FindAllUsers()
    return espresso.JSON[User]{Data: users}, nil
}
```

## API

### GetState

Type-safe state retrieval with error handling:

```go
state, ok := espresso.GetState[AppState](ctx)
if !ok {
    return espresso.JSON[User]{}, errors.New("state not found")
}
```

### MustGetState

Panics if state not found (use when state is guaranteed):

```go
state := espresso.MustGetState[AppState](ctx)
// Guaranteed to have state
```

### WithStateMiddleware

Inject state via middleware:

```go
app.Use(espresso.WithStateMiddleware(appState))
```

### State Extractor

Axum-style extractor:

```go
func handler(ctx context.Context, req *espresso.JSON[Req], 
             state espresso.State[AppState]) (Res, error) {
    db := state.Data.DB
    // ...
}
```

## Patterns

### Database Pattern

```go
type AppState struct {
    DB *sql.DB
}

func getUsers(ctx context.Context, req *espresso.JSON[GetUsersReq]) (espresso.JSON[UsersRes], error) {
    state := espresso.MustGetState[AppState](ctx)
    
    users, err := state.DB.Query("SELECT * FROM users")
    if err != nil {
        return espresso.JSON[UsersRes]{}, err
    }
    
    return espresso.JSON[UsersRes]{Data: users}, nil
}
```

### Repository Pattern

```go
type AppState struct {
    UserRepo    *UserRepository
    ProductRepo *ProductRepository
}

func getUser(ctx context.Context, req *extractor.Path[UserReq]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)
    
    user, err := state.UserRepo.FindByID(req.Data.ID)
    if err != nil {
        return espresso.JSON[User]{}, err
    }
    
    return espresso.JSON[User]{Data: user}, nil
}
```

### Configuration Pattern

```go
type AppState struct {
    Config Config
}

func getConfig(ctx context.Context) (espresso.JSON[Config], error) {
    state := espresso.MustGetState[AppState](ctx)
    return espresso.JSON[Config]{Data: state.Config}, nil
}
```

## Substate Pattern

Extract specific components from parent state:

```go
type AppState struct {
    DB     *sql.DB
    Config Config
    Logger zerolog.Logger
}

// Extract only DB from AppState
func getUser(ctx context.Context, req *extractor.Path[UserReq]) (espresso.JSON[User], error) {
    db, ok := espresso.FromState[AppState, *sql.DB](ctx, func(s AppState) *sql.DB {
        return s.DB
    })
    if !ok {
        return espresso.JSON[User]{}, errors.New("database not found")
    }
    
    return espresso.JSON[User]{Data: db.FindUser(req.Data.ID)}, nil
}
```

## Immutable State

State is immutable by design. For mutable state, use Go's synchronization primitives:

### ✅ Immutable (Recommended)

```go
type AppState struct {
    DB     *sql.DB          // Thread-safe
    Config Config            // Immutable
    Cache  *sync.Map         // Thread-safe map
}
```

### ⚠️ Mutable with Synchronization

```go
type AppState struct {
    Counter *atomic.Int64      // Atomic counter
    Cache   *sync.Map           // Thread-safe map
    Mu      sync.RWMutex        // Mutex
    Data    map[string]string   // Protected by Mu
}

func (s *AppState) Increment() {
    s.Counter.Add(1)  // Thread-safe
}

func (s *AppState) GetData(key string) (string, bool) {
    s.Mu.RLock()
    defer s.Mu.RUnlock()
    val, ok := s.Data[key]
    return val, ok
}
```

## Complete Example

```go
package main

import (
    "context"
    "database/sql"
    "net/http"
    
    "github.com/suryakencana007/espresso"
)

type AppState struct {
    DB     *sql.DB
    Config Config
}

type Config struct {
    Port int
    Env  string
}

type CreateUserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    db := initDB()
    config := Config{Port: 8080, Env: "production"}
    
    appState := AppState{DB: db, Config: config}

    espresso.Portafilter().
        WithState(appState).
        Get("/users/{id}", getUser).
        Post("/users", createUser).
        Brew(espresso.WithAddr(":8080"))
}

func getUser(ctx context.Context, req *extractor.Path[struct{ ID int }]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)
    
    user, err := state.DB.FindUser(req.Data.ID)
    if err != nil {
        return espresso.JSON[User]{}, err
    }
    
    return espresso.JSON[User]{Data: user}, nil
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)
    
    user, err := state.DB.CreateUser(req.Data.Name, req.Data.Email)
    if err != nil {
        return espresso.JSON[User]{}, err
    }
    
    return espresso.JSON[User]{
        StatusCode: http.StatusCreated,
        Data:       user,
    }, nil
}
```

## Best Practices

1. **Keep State Immutable** - Safer for concurrency
2. **Use Interfaces for Testing** - Mock dependencies easily
3. **Group Related Dependencies** - Use repositories, services
4. **Database Connections** - Always thread-safe (*sql.DB)
5. **Configuration** - Immutable after creation

## Next Steps

- [Examples](/examples/state-management) — Production examples
- [Middleware](/guide/middleware/) — Middleware reference
- [API Reference](/api/state) — State API docs