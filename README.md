<div align="center">
  <img src="logo.png" alt="Espresso Logo" width="200"/>
</div>

# Espresso ☕

> **Production-grade HTTP routing framework for Go** — Brew robust APIs with the precision of a barista.

[![Go Reference](https://pkg.go.dev/badge/github.com/suryakencana007/espresso.svg)](https://pkg.go.dev/github.com/suryakencana007/espresso)
[![Go Report Card](https://goreportcard.com/badge/github.com/suryakencana007/espresso)](https://goreportcard.com/report/github.com/suryakencana007/espresso)

---

## Why Espresso?

Like a perfectly pulled espresso shot, this framework delivers:

- **Fast** — Zero-allocation handlers with sync.Pool for request objects
- **Strong** — Production-ready with battle-tested patterns inspired by Axum (Rust) and Tower
- **Pure** — No magic, just clean Go code with explicit types
- **Aromatic** — Rich type-safe extractors without manual implementation

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Package Structure](#package-structure)
- [Core Concepts](#core-concepts)
- [Routing](#routing)
- [Handlers & Coffee-Themed Aliases](#handlers--coffee-themed-aliases)
- [Axum-Style Extractors](#axum-style-extractors)
- [Response Types](#response-types)
- [Middleware](#middleware)
- [Service Layers](#service-layers)
- [Object Pooling](#object-pooling)
- [Complete Example](#complete-example)
- [Benchmarks](#benchmarks)
- [API Reference](#api-reference)
- [Contributing](#contributing)

---

## Installation

```bash
go get github.com/suryakencana007/espresso
```

**Requirements:**
- Go 1.22+ (for path parameter support)

---

## Quick Start

```go
package main

import (
    "context"
    "net/http"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
)

type CreateUserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

    espresso.Portafilter().
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.RecoverMiddleware()).
        Use(httpmiddleware.LoggingMiddleware()).
        Use(httpmiddleware.CORSMiddleware(httpmiddleware.DefaultCORSConfig)).
        
        Get("/health", espresso.Ristretto(healthCheck)).
        Post("/users", espresso.Doppio(createUser)).
        Get("/users/{id}", espresso.Doppio(getUser)).
        
        Brew(espresso.WithAddr(":8080"))
}

func healthCheck() espresso.Text {
    return espresso.Text{Body: "OK"}
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
    return espresso.JSON[UserRes]{
        StatusCode: http.StatusCreated,
        Data:       UserRes{ID: 1, Name: req.Data.Name},
    }, nil
}

func getUser(ctx context.Context, req *extractor.Path[UserReq]) (espresso.JSON[UserRes], error) {
    return espresso.JSON[UserRes]{Data: UserRes{ID: req.Data.ID}}, nil
}
```

---

## Package Structure

Espresso uses a modular package structure for better organization:

```go
import (
    // Core - handlers, router, server, response types
    "github.com/suryakencana007/espresso"
    
    // HTTP Middleware - CORS, rate limiting, compression, etc.
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    
    // Service Layers - timeout, retry, circuit breaker, etc.
    servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
    
    // Request Extractors - JSON, Query, Path, Header, Form, XML
    "github.com/suryakencana007/espresso/extractor"
    
    // Object Pooling - buffer pools for performance
    "github.com/suryakencana007/espresso/pool"
)
```

### Package Overview

| Package | Purpose | Key Types |
|---------|---------|-----------|
| `espresso` | Core framework | `Router`, `Handler`, `JSON`, `Text`, `Status` |
| `middleware/http` | HTTP-level middleware | `CORS`, `RateLimit`, `Compress`, `RequestID` |
| `middleware/service` | Service-level layers | `Timeout`, `Retry`, `CircuitBreaker`, `Validation` |
| `extractor` | Request extractors | `JSON`, `Query`, `Path`, `Header`, `Form`, `XML` |
| `pool` | Object pooling | `BufferPool`, `ByteSlicePool`, `StringSlicePool` |

---

## Core Concepts

### The Coffee Metaphor

| Espresso Term | Framework Component | Purpose |
|---------------|---------------------|---------|
| **Portafilter** | `Portafilter()` | Creates the router that holds all routes |
| **Ristretto** | `Ristretto()` | 0-param handler (concentrated, simple) |
| **Solo** | `Solo()` | 1-param handler (single shot) |
| **Doppio** | `Doppio()` | 2-param handler (double shot, full power) |
| **Brew** | `Brew()` | Starts the server (brews and serves) |
| **Use** | `Use()` | Adds middleware (like grinding beans) |
| **Extract** | `FromRequest.Extract()` | Extracts data from request (like espresso extraction) |

---

## Routing

### Chain Pattern (Recommended)

```go
espresso.Portafilter().
    Use(httpmiddleware.RequestIDMiddleware()).
    Use(httpmiddleware.RecoverMiddleware()).
    Use(httpmiddleware.CORSMiddleware(httpmiddleware.DefaultCORSConfig)).
    
    Get("/health", espresso.Ristretto(health)).
    Post("/users", espresso.Doppio(createUser)).
    Get("/users/{id}", espresso.Doppio(getUser)).
    
    Brew(espresso.WithAddr(":8080"))
```

### Route Methods

```go
router.Get("/path", handler)      // GET
router.Post("/path", handler)    // POST
router.Put("/path", handler)     // PUT
router.Delete("/path", handler)  // DELETE
router.Patch("/path", handler)   // PATCH
```

### Path Parameters (Go 1.22+)

```go
type UserReq struct {
    ID int `path:"id"`
}

func getUser(ctx context.Context, req *extractor.Path[UserReq]) (espresso.JSON[User], error) {
    userID := req.Data.ID
    // ...
}

router.Get("/users/{id}", espresso.Doppio(getUser))
```

---

## Handlers & Coffee-Themed Aliases

| Alias | Signature | Use Case |
|-------|-----------|---------|
| `Ristretto` | `func() Res` | Health checks, static responses |
| `Solo` | `func(*Req) (Res, error)` | Simple handlers, no context |
| `Doppio` | `func(ctx, *Req) (Res, error)` | Production handlers |

### Ristretto (0 params)

```go
func healthCheck() espresso.Text {
    return espresso.Text{Body: "OK"}
}

router.Get("/health", espresso.Ristretto(healthCheck))
```

### Solo (1 param)

```go
func createUser(req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
    return espresso.JSON[UserRes]{Data: User{ID: 1, Name: req.Data.Name}}, nil
}

router.Post("/users", espresso.Solo(createUser))
```

### Doppio (2 params)

```go
func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
    requestID := espresso.GetRequestID(ctx)
    return espresso.JSON[UserRes]{
        StatusCode: http.StatusCreated,
        Data:       User{ID: 1, Name: req.Data.Name},
    }, nil
}

router.Post("/users", espresso.Doppio(createUser))
```

---

## Axum-Style Extractors

### Built-in Extractors

| Extractor | Tags | Description |
|-----------|------|-------------|
| `JSON[T]` | `json:"field"` | JSON body extraction |
| `Query[T]` | `query:"param"` | URL query parameters |
| `Path[T]` | `path:"id"` | Path parameters |
| `Header[T]` | `header:"Name"` | HTTP headers |
| `Form[T]` | `form:"field"` | Form data |
| `Cookie[T]` | `cookie:"name"` | HTTP cookies |
| `XML[T]` | `xml:"field"` | XML body |
| `RawBody` | — | Raw bytes body |

### JSON

```go
func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
    user := req.Data
    return espresso.JSON[UserRes]{
        StatusCode: http.StatusCreated,
        Data:       UserRes{ID: 1, Name: user.Name},
    }, nil
}
```

### Query Parameters

```go
type SearchReq struct {
    Query string `query:"q,required"`
    Page  int    `query:"page"`
    Limit int    `query:"limit"`
}

func search(ctx context.Context, req *extractor.Query[SearchReq]) (espresso.JSON[SearchRes], error) {
    params := req.Data
    // params.Query is required, params.Page defaults to 0
    return espresso.JSON[SearchRes]{Data: results}, nil
}
```

### Custom Extractor

```go
type CreateUserReq struct {
    Name  string
    Email string
    Role  string // from query param
}

func (r *CreateUserReq) Extract(req *http.Request) error {
    if err := json.NewDecoder(req.Body).Decode(r); err != nil {
        return err
    }
    r.Role = req.URL.Query().Get("role")
    if r.Role == "" {
        r.Role = "user"
    }
    return nil
}
```

---

## Middleware

### HTTP-Level Middleware

```go
import httpmiddleware "github.com/suryakencana007/espresso/middleware/http"

espresso.Portafilter().
    Use(httpmiddleware.RequestIDMiddleware()).
    Use(httpmiddleware.RecoverMiddleware()).
    Use(httpmiddleware.LoggingMiddleware()).
    Use(httpmiddleware.CORSMiddleware(httpmiddleware.DefaultCORSConfig)).
    Use(httpmiddleware.CompressMiddleware()).
    Use(httpmiddleware.RateLimitMiddleware(limiter)).
    Brew()
```

### CORS Configuration

```go
corsConfig := httpmiddleware.CORSConfig{
    AllowOrigins:     []string{"https://example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
    MaxAge:           3600,
}
router.Use(httpmiddleware.CORSMiddleware(corsConfig))
```

### Rate Limiting

```go
// Token Bucket - global rate limiting
limiter := httpmiddleware.NewTokenBucketLimiter(100, 100)
router.Use(httpmiddleware.RateLimitMiddleware(limiter))

// Per-key rate limiting (recommended)
limiter := httpmiddleware.NewTokenBucketLimiterPerKey(100, 100)
router.Use(httpmiddleware.RateLimitMiddleware(limiter))

// Sliding Window
limiter := httpmiddleware.NewSlidingWindowLimiter(time.Minute, 100)
router.Use(httpmiddleware.RateLimitMiddleware(limiter))
```

---

## Service Layers

Service layers run **after extraction** with typed request/response.

```go
import servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
```

### Timeout

```go
layer := servicemiddleware.TimeoutLayer[*CreateUserReq, espresso.JSON[UserRes]](5 * time.Second)
```

### Retry with Backoff

```go
// Fixed backoff
servicemiddleware.RetryLayer[*CreateUserReq, espresso.JSON[UserRes]](
    3, 100 * time.Millisecond, servicemiddleware.BackoffFixed)

// Exponential backoff
servicemiddleware.RetryLayer[*CreateUserReq, espresso.JSON[UserRes]](
    3, 100 * time.Millisecond, servicemiddleware.BackoffExponential)
```

### Circuit Breaker

```go
config := servicemiddleware.CircuitBreakerConfig{
    ServiceName:      "UserService",
    FailureThreshold: 5,
    Timeout:          30 * time.Second,
    SuccessThreshold: 3,
}
servicemiddleware.CircuitBreakerLayer[*CreateUserReq, espresso.JSON[UserRes]](config)
```

### WithLayers (Type Erasure)

```go
// Define reusable layers
commonLayers := espresso.Layers(
    espresso.Timeout(5*time.Second),
    espresso.Logging(logger, "api"),
)

// Apply to handlers with type inference
app.Post("/users", espresso.WithLayers(createUser, commonLayers...))
```

---

## Dependency Injection (State Management)

Espresso provides **Axum-style state management** with Go-idiomatic context-based approach.

### Quick Start

```go
// Define your application state
type AppState struct {
    DB     *sql.DB
    Config Config
    Logger zerolog.Logger
}

// Provide state to router
func main() {
    appState := AppState{
        DB:     db,
        Config: config,
        Logger: logger,
    }

    espresso.Portafilter().
        WithState(appState).                    // Inject state
        Get("/users", espresso.Doppio(getUsers)).
        Post("/users", espresso.Doppio(createUser)).
        Brew(espresso.WithAddr(":8080"))
}

// Access state in handlers
func getUsers(ctx context.Context, req *espresso.JSON[GetUsersReq]) (espresso.JSON[UsersRes], error) {
    // Method 1: GetState (returns state and boolean)
    state, ok := espresso.GetState[AppState](ctx)
    if !ok {
        return espresso.JSON[UsersRes]{}, errors.New("state not found")
    }
    
    users := state.DB.FindAllUsers()
    return espresso.JSON[UsersRes]{Data: users}, nil
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
    // Method 2: MustGetState (panics if not found - use when state is guaranteed)
    state := espresso.MustGetState[AppState](ctx)
    
    user := state.DB.CreateUser(req.Data)
    return espresso.JSON[UserRes]{Data: user}, nil
}
```

### Three Ways to Use State

#### 1. Context-Based (Recommended)

```go
func handler(ctx context.Context, req *espresso.JSON[Req]) (Res, error) {
    state := espresso.MustGetState[AppState](ctx)
    db := state.DB
    // use state...
}
```

**Advantages:**
- ✅ Zero breaking changes to existing handlers
- ✅ Works with all existing handler signatures
- ✅ Most Go-idiomatic approach
- ✅ No extra handler parameters needed

#### 2. State Extractor (Axum-style)

```go
func handler(ctx context.Context, req *espresso.JSON[Req], 
             state espresso.State[AppState]) (Res, error) {
    db := state.Data.DB
    // use state...
}
```

**Advantages:**
- ✅ Similar to Axum's `State<S>` extractor
- ✅ Explicit dependency in handler signature
- ✅ Type-safe at compile time

#### 3. Substate Pattern

```go
// Extract substate from parent state
func handler(ctx context.Context, req *espresso.JSON[Req]) (Res, error) {
    db, ok := espresso.FromState[AppState, *sql.DB](ctx, func(s AppState) *sql.DB {
        return s.DB
    })
    if !ok {
        return Res{}, errors.New("database not found")
    }
    // use db...
}
```

### Middleware State Injection

```go
// Via router method (fluent API)
router := espresso.Portafilter().
    WithState(appState).
    Get("/api/users", handler)

// Via middleware (manual)
router := espresso.Portafilter()
router.Use(espresso.WithStateMiddleware(appState))
router.Get("/api/users", handler)
```

### Immutable State Pattern

State is immutable and thread-safe by design:

```go
type AppState struct {
    DB     *sql.DB          // Thread-safe
    Config Config            // Immutable after creation
    Cache  *sync.Map         // Thread-safe map
}

// ❌ Don't: Mutable state without synchronization
type BadState struct {
    Counter int  // NOT thread-safe!
}

// ✅ Do: Use sync primitives for mutable state
type GoodState struct {
    Counter *atomic.Int64  // Thread-safe
}
```

### Shared Mutable State

For mutable state, use Go's synchronization primitives:

```go
type AppState struct {
    DB     *sql.DB
    Counter *atomic.Int64      // Atomic counter
    Cache  *sync.Map           // Thread-safe map
    Mu     sync.RWMutex        // Mutex for complex state
    Data   map[string]string   // Protected by Mu
}

func (s *AppState) Increment() {
    s.Counter.Add(1)
}

func (s *AppState) GetData(key string) (string, bool) {
    s.Mu.RLock()
    defer s.Mu.RUnlock()
    val, ok := s.Data[key]
    return val, ok
}
```

### Complete Example

```go
package main

import (
    "context"
    "database/sql"
    "net/http"
    
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
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

func main() {
    db := initDB()
    config := loadConfig()
    
    appState := AppState{DB: db, Config: config}

    espresso.Portafilter().
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.RecoverMiddleware()).
        WithState(appState).                    // Inject state
        Get("/users/{id}", getUser).
        Post("/users", createUser).
        Brew(espresso.WithAddr(":8080"))
}

func getUser(ctx context.Context, req *extractor.Path[struct{ ID int `path:"id"` }]) (espresso.JSON[User], error) {
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
    return espresso.JSON[User]{Data: user}, nil
}
```

---

## Object Pooling

Reduce GC pressure with object pools:

```go
import "github.com/suryakencana007/espresso/pool"

// Get a buffer
buf := pool.GetBuffer(256)
defer pool.PutBuffer(buf)
buf.WriteString("Hello, World!")

// Get a byte slice
slice := pool.GetByteSlice(1024)
defer pool.PutByteSlice(slice)

// Get a string slice
strSlice := pool.GetStringSlice()
defer pool.PutStringSlice(strSlice)
```

---

## Response Types

### JSON

```go
return espresso.JSON[UserRes]{
    StatusCode: http.StatusCreated,
    Data:       UserRes{ID: 1, Name: "John"},
}
```

### Text

```go
return espresso.Text{Body: "OK"}
return espresso.Text{StatusCode: http.StatusNotFound, Body: "not found"}
```

### Status Only

```go
return espresso.Status(http.StatusNoContent) // 204
```

### Custom Response

```go
type HTML struct {
    Body string
}

func (h HTML) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "text/html")
    _, err := w.Write([]byte(h.Body))
    return err
}
```

---

## Complete Example

See [cmd/example/main.go](cmd/example/main.go) for a complete working example.

```go
package main

import (
    "context"
    "net/http"
    "os"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
)

type CreateUserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

type UserRes struct {
    Message string `json:"message"`
}

func main() {
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

    // Reusable layer stack
    commonLayers := espresso.Layers(
        espresso.Timeout(5*time.Second),
        espresso.Logging(log.Logger, "api"),
    )

    espresso.Portafilter().
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.RecoverMiddleware()).
        Use(httpmiddleware.LoggingMiddleware()).
        Use(httpmiddleware.CORSMiddleware(httpmiddleware.DefaultCORSConfig)).
        
        // Handlers with layers
        Post("/api/users", espresso.WithLayers(createUser, commonLayers...)).
        Get("/api/users/{id}", espresso.Doppio(getUser)).
        Get("/api/health", espresso.Ristretto(healthCheck)).
        
        Brew(espresso.WithAddr(":38080"))
}

func healthCheck() espresso.Text {
    return espresso.Text{Body: "pong"}
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
    return espresso.JSON[UserRes]{
        StatusCode: http.StatusCreated,
        Data:       UserRes{Message: "Created user: " + req.Data.Name},
    }, nil
}

func getUser(ctx context.Context, req *extractor.Path[struct{ ID int `path:"id"` }]) (espresso.JSON[UserRes], error) {
    return espresso.JSON[UserRes]{Data: UserRes{Message: "User ID: " + string(rune(req.Data.ID))}}, nil
}
```

---

## Benchmarks

### Handler Performance

| Handler Type | Allocation | Pool |
|--------------|-------------|------|
| `Doppio` | Zero per request | sync.Pool |
| `Solo` | Zero per request | sync.Pool |
| `Ristretto` | Zero | None needed |

```bash
BenchmarkDecodeSafeJSON-16    357073    3208 ns/op    5669 B/op    15 allocs/op
BenchmarkBufferPool-16         67260426   17.92 ns/op      0 B/op     0 allocs/op
```

### Test Coverage

| Package | Coverage |
|---------|----------|
| Root | 75.9% |
| middleware/http | 93.0% |
| middleware/service | 78.3% |
| extractor | 58.6% |
| pool | 90.0% |

Run tests:
```bash
go test ./... -cover
go test -bench=. -benchmem
```

---

## API Reference

### Router

```go
router := espresso.Portafilter()
router.Use(middleware...)
router.Get(path, handler)
router.Post(path, handler)
router.Brew(opts...)
```

### Server Options

```go
espresso.WithAddr(":8080")
espresso.WithReadTimeout(10 * time.Second)
espresso.WithWriteTimeout(10 * time.Second)
espresso.WithShutdownTimeout(10 * time.Second)
```

### Extractors

```go
espresso.JSON[T]      // JSON body
extractor.Query[T]   // Query parameters
extractor.Path[T]    // Path parameters
extractor.Header[T]  // Headers
extractor.Form[T]    // Form data
extractor.XML[T]     // XML body
extractor.RawBody    // Raw bytes
```

### Middleware

```go
// HTTP-level
httpmiddleware.RequestIDMiddleware()
httpmiddleware.RecoverMiddleware()
httpmiddleware.LoggingMiddleware()
httpmiddleware.CORSMiddleware(config)
httpmiddleware.CompressMiddleware()
httpmiddleware.RateLimitMiddleware(limiter)

// Rate limiters
httpmiddleware.NewTokenBucketLimiter(rate, capacity)
httpmiddleware.NewTokenBucketLimiterPerKey(rate, capacity)
httpmiddleware.NewSlidingWindowLimiter(window, maxReq)

// Service-level
servicemiddleware.TimeoutLayer[Req, Res](timeout)
servicemiddleware.RetryLayer[Req, Res](maxRetries, backoff, strategy)
servicemiddleware.CircuitBreakerLayer[Req, Res](config)
servicemiddleware.ValidationLayer[Req, Res](validator)
servicemiddleware.LoggingLayer[Req, Res](logger, serviceName)
```

---

## Contributing

```bash
git clone https://github.com/suryakencana007/espresso.git
cd espresso
go mod download
go test ./...
golangci-lint run ./...
```

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

## Credits

- Inspired by [Axum](https://github.com/tokio-rs/axum) (Rust) and [Tower](https://github.com/tower-rs/tower) (Rust)

---

<p align="center">
  <strong>Espresso</strong> — Brew robust APIs with precision ☕
</p>