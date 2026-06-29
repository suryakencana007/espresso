# State Management

Espresso provides Axum-style state management for dependency injection.

## Overview

State allows you to share application-wide dependencies (databases, configurations, clients) with handlers without global variables.

```go
// Application state
type AppState struct {
    DB     *sql.DB
    Cache  *redis.Client
    Config Config
}

func main() {
    // Create state
    state := AppState{
        DB:     db,
        Cache:  redisClient,
        Config: config,
    }
    
    // Inject into router
    router := espresso.Portafilter().
        WithState(state).
        Get("/users", espresso.Doppio(listUsers))
    
    router.Brew()
}
```

## Accessing State

### Using GetState

```go
func listUsers(ctx context.Context, req *espresso.JSON[ListQuery]) (espresso.JSON[[]User], error) {
    state, err := espresso.GetState[AppState](ctx)
    if err != nil {
        return espresso.JSON[[]User]{}, err
    }
    
    users := state.DB.QueryUsers(ctx, req.Data.Page, req.Data.PerPage)
    return espresso.JSON[[]User]{Data: users}, nil
}
```

### Using MustGetState

Panics if state is not found or wrong type:

```go
func getUser(ctx context.Context, req *extractor.Path[UserPath]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)
    user := state.DB.FindUser(req.Data.ID)
    return espresso.JSON[User]{Data: user}, nil
}
```

### Using State Extractor

Type-safe state extraction:

```go
func handler(ctx context.Context, req *espresso.JSON[Req], state espresso.State[AppState]) (Response, error) {
    db := state.Data.DB
    config := state.Data.Config
    // ...
}
```

## Complete Example

The program below is self-contained — it uses an in-memory `*Store` in place of
a real database/cache so it compiles and runs as written. Swap `Store` for your
own `*sql.DB` / cache client in production.

```go
package main

import (
    "context"
    "encoding/json"
    "net/http"
    "sync"

    "github.com/suryakencana007/espresso/v2"
    "github.com/suryakencana007/espresso/v2/extractor"
    httpmiddleware "github.com/suryakencana007/espresso/v2/middleware/http"
)

// Store is a tiny in-memory user store standing in for a database + cache.
type Store struct {
    mu     sync.RWMutex
    users  map[int64]User
    cache  map[int64][]byte // marshaled User cache, keyed by ID
    nextID int64
}

func NewStore() *Store {
    return &Store{users: make(map[int64]User), cache: make(map[int64][]byte), nextID: 1}
}

func (s *Store) List(limit int) []User {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]User, 0, len(s.users))
    for _, u := range s.users {
        if len(out) >= limit {
            break
        }
        out = append(out, u)
    }
    return out
}

func (s *Store) Find(id int64) (User, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    u, ok := s.users[id]
    return u, ok
}

func (s *Store) Create(u User) User {
    s.mu.Lock()
    defer s.mu.Unlock()
    u.ID = s.nextID
    s.users[u.ID] = u
    s.nextID++
    return u
}

// Application state
type AppState struct {
    Store  *Store
    Config Config
}

type Config struct {
    AppName    string
    Version    string
    Debug      bool
    MaxResults int
}

// Models
type User struct {
    ID    int64  `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type UserPath struct {
    ID int64 `path:"id,required"`
}

type ListQuery struct {
    Page int `query:"page"`
}

type CreateUserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// Handlers
func listUsers(ctx context.Context, query *extractor.Query[ListQuery]) (espresso.JSON[[]User], error) {
    state := espresso.MustGetState[AppState](ctx)

    limit := state.Config.MaxResults
    if limit <= 0 {
        limit = 10
    }

    return espresso.JSON[[]User]{Data: state.Store.List(limit)}, nil
}

func getUser(ctx context.Context, path *extractor.Path[UserPath]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)

    // Try cache first
    if cached, ok := state.Store.cache[path.Data.ID]; ok {
        var user User
        if err := json.Unmarshal(cached, &user); err == nil {
            return espresso.JSON[User]{Data: user}, nil
        }
    }

    user, ok := state.Store.Find(path.Data.ID)
    if !ok {
        return espresso.JSON[User]{}, espresso.ErrNotFound("user not found")
    }

    // Cache result
    if data, err := json.Marshal(user); err == nil {
        state.Store.cache[path.Data.ID] = data
    }

    return espresso.JSON[User]{Data: user}, nil
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)

    user := state.Store.Create(User{
        Name:  req.Data.Name,
        Email: req.Data.Email,
    })

    return espresso.JSON[User]{
        StatusCode: http.StatusCreated,
        Data:       user,
    }, nil
}

func main() {
    config := Config{
        AppName:    "MyAPI",
        Version:    "1.0.0",
        Debug:      true,
        MaxResults: 100,
    }

    // Create application state
    state := AppState{
        Store:  NewStore(),
        Config: config,
    }

    // Create router with state
    router := espresso.Portafilter().
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.LoggingMiddleware()).
        WithState(state).
        Get("/health", func() string { return "OK" }).
        Get("/users", espresso.Doppio(listUsers)).
        Get("/users/{id}", espresso.Doppio(getUser)).
        Post("/users", espresso.Doppio(createUser))

    router.Brew()
}
```

## Immutable State

State is immutable after creation. This ensures thread-safety:

```go
// Good - State is read-only
func handler(ctx context.Context, req *espresso.JSON[Req]) (Response, error) {
    state := espresso.MustGetState[AppState](ctx)
    db := state.DB // Read-only access
    // ...
}

// Bad - Don't modify state directly
func handler(ctx context.Context, req *espresso.JSON[Req]) (Response, error) {
    state := espresso.MustGetState[AppState](ctx)
    state.Config.Debug = false // Don't do this!
    // ...
}
```

If you need mutable state, use pointers:

```go
type AppState struct {
    DB     *sql.DB
    Cache  *redis.Client
    Config *Config // Pointer - can modify fields
}
```

## Multiple State Types

You can store multiple state types:

```go
type DBState struct {
    DB *sql.DB
}

type CacheState struct {
    Redis *redis.Client
}

func main() {
    // Store multiple states in a composite
    state := struct {
        DBState
        CacheState
    }{
        DBState:   DBState{DB: db},
        CacheState: CacheState{Redis: redisClient},
    }
    
    router := espresso.Portafilter().
        WithState(state)
    // ...
}
```

## State with Middleware

Access state in middleware:

```go
func StateMiddleware(state AppState) httpmiddleware.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Inject state into context
            ctx := context.WithValue(r.Context(), espresso.StateKey{}, state)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Note: WithState() already does this internally
```

## Testing with State

Mock state for tests:

```go
func TestGetUser(t *testing.T) {
    // Create mock state
    mockDB := &MockDB{}
    mockCache := &MockCache{}
    
    state := AppState{
        DB:    mockDB,
        Cache: mockCache,
        Config: Config{MaxResults: 10},
    }
    
    // Create router with mock state
    router := espresso.Portafilter().
        WithState(state).
        Get("/users/{id}", espresso.Doppio(getUser))
    
    // Test request
    req := httptest.NewRequest("GET", "/users/123", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    // Assertions...
}
```

## Dependency Injection Container

Use state as a simple DI container:

```go
type Container struct {
    DB           *sql.DB
    Redis        *redis.Client
    UserService  *UserService
    EmailService *EmailService
    Logger       *zerolog.Logger
    Config       Config
}

func NewContainer(config Config) (*Container, error) {
    db, err := sql.Open("postgres", config.DatabaseURL)
    if err != nil {
        return nil, err
    }
    
    redis := redis.NewClient(&redis.Options{
        Addr: config.RedisAddr,
    })
    
    logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
    
    userService := NewUserService(db, redis)
    emailService := NewEmailService(config.SMTP)
    
    return &Container{
        DB:           db,
        Redis:        redis,
        UserService:  userService,
        EmailService: emailService,
        Logger:       &logger,
        Config:       config,
    }, nil
}
```

## Best Practices

1. **Keep state simple**: Only store what you need
2. **Use pointers for shared objects**: DB connections, clients
3. **Don't mutate state**: Treat it as immutable
4. **Initialize once**: Create state at startup
5. **Close resources properly**: Use defer in main

```go
func main() {
    container, err := NewContainer(config)
    if err != nil {
        log.Fatal(err)
    }
    defer container.DB.Close()
    defer container.Redis.Close()
    
    router := espresso.Portafilter().
        WithState(container).
        Get("/health", healthHandler)
    
    router.Brew()
}
```