# Quick Start

Build a complete REST API in 5 minutes.

## Prerequisites

- Go 1.22+ installed
- Basic Go knowledge

## Step 1: Create Project

```bash
mkdir my-api && cd my-api
go mod init my-api
go get github.com/suryakencana007/espresso
```

## Step 2: Basic Server

Create `main.go`:

```go
package main

import (
    "context"
    "net/http"

    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
)

func main() {
    espresso.Portafilter().
        // Middleware
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.RecoverMiddleware()).
        
        // Routes
        Get("/health", espresso.Ristretto(healthCheck)).
        Get("/users/{id}", espresso.Doppio(getUser)).
        Post("/users", espresso.Doppio(createUser)).
        
        // Start server
        Brew(espresso.WithAddr(":8080"))
}

func healthCheck() espresso.Text {
    return espresso.Text{Body: "OK"}
}

func getUser(ctx context.Context, req *extractor.Path[struct {
    ID int `path:"id"`
}]) (espresso.JSON[User], error) {
    return espresso.JSON[User]{
        Data: User{ID: req.Data.ID, Name: "John Doe"},
    }, nil
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    return espresso.JSON[User]{
        StatusCode: http.StatusCreated,
        Data: User{ID: 1, Name: req.Data.Name},
    }, nil
}

type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type CreateUserReq struct {
    Name string `json:"name"`
}
```

## Step 3: Run

```bash
go run main.go
```

Server starts at `http://localhost:8080`

## Step 4: Test

```bash
# Health check
curl http://localhost:8080/health
# Output: OK

# Get user
curl http://localhost:8080/users/42
# Output: {"id":42,"name":"John Doe"}

# Create user
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Jane Doe"}'
# Output: {"id":1,"name":"Jane Doe"}
```

## Understanding the Code

### Handler Types

| Type | Signature | Use Case |
|------|-----------|----------|
| `Ristretto` | `func() Res` | No params, returns response |
| `Solo` | `func(*Req) Res` | One param (request) |
| `Doppio` | `func(ctx, *Req) (Res, error)` | Full control |

### Extractors

```go
// JSON body extraction
func handler(ctx context.Context, req *espresso.JSON[CreateUserReq]) (Res, error)

// Path parameter extraction
func handler(ctx context.Context, req *extractor.Path[UserReq]) (Res, error)

// Query parameter extraction
func handler(ctx context.Context, req *extractor.Query[SearchReq]) (Res, error)

// Header extraction
func handler(ctx context.Context, req *extractor.Header[AuthReq]) (Res, error)
```

### Middleware

```go
// HTTP-level middleware (runs before extraction)
Use(httpmiddleware.CORSMiddleware(config))
Use(httpmiddleware.RateLimitMiddleware(limiter))

// Service-level layers (runs after extraction)
WithLayers(createUser, espresso.Timeout(5*time.Second))
```

## Next Steps

- [Core Concepts](/guide/core-concepts) — Deep dive into architecture
- [Handlers](/guide/handlers) — Learn about handler types
- [Extractors](/guide/extractors) — Master request extraction
- [Examples](/examples/) — Real-world examples