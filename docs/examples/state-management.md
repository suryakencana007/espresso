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
func getUser(ctx context.Context, req *espresso.Path[UserPath]) (espresso.JSON[User], error) {
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

```go
package main

import (
    "context"
    "database/sql"
    "log"
    
    "github.com/suryakencana007/espresso"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    "github.com/redis/go-redis/v9"
)

// Application state
type AppState struct {
    DB     *sql.DB
    Cache  *redis.Client
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

// Handlers
func listUsers(ctx context.Context, query *espresso.Query[ListQuery]) (espresso.JSON[[]User], error) {
    state := espresso.MustGetState[AppState](ctx)
    
    limit := state.Config.MaxResults
    if limit <= 0 {
        limit = 10
    }
    
    users, err := state.DB.QueryUsers(ctx, limit)
    if err != nil {
        return espresso.JSON[[]User]{}, err
    }
    
    return espresso.JSON[[]User]{Data: users}, nil
}

func getUser(ctx context.Context, path *espresso.Path[UserPath]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)
    
    // Try cache first
    cached, err := state.Cache.Get(ctx, "user:"+string(path.Data.ID)).Result()
    if err == nil {
        var user User
        json.Unmarshal([]byte(cached), &user)
        return espresso.JSON[User]{Data: user}, nil
    }
    
    // Query database
    user, err := state.DB.FindUser(ctx, path.Data.ID)
    if err != nil {
        return espresso.JSON[User]{}, err
    }
    
    // Cache result
    data, _ := json.Marshal(user)
    state.Cache.Set(ctx, "user:"+string(path.Data.ID), data, 5*time.Minute)
    
    return espresso.JSON[User]{Data: user}, nil
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    state := espresso.MustGetState[AppState](ctx)
    
    user := User{
        Name:  req.Data.Name,
        Email: req.Data.Email,
    }
    
    if err := state.DB.CreateUser(ctx, &user); err != nil {
        return espresso.JSON[User]{}, err
    }
    
    return espresso.JSON[User]{
        StatusCode: http.StatusCreated,
        Data:       user,
    }, nil
}

func main() {
    // Initialize dependencies
    db, err := sql.Open("postgres", "postgres://...")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    
    config := Config{
        AppName:    "MyAPI",
        Version:    "1.0.0",
        Debug:      true,
        MaxResults: 100,
    }
    
    // Create application state
    state := AppState{
        DB:     db,
        Cache:  redisClient,
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