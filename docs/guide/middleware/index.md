# Middleware Overview

Espresso provides a two-level middleware architecture inspired by Tower:

<Mermaid source="sequenceDiagram
    participant C as Client
    participant M as HTTP Middleware
    participant R as Router
    participant S as Service Layer
    participant H as Handler
    C->>M: Request
    M->>M: Request ID, CORS, Logging
    M->>R: Routed Request
    R->>S: Typed Request
    S->>S: Retry, Timeout, Circuit Breaker
    S->>H: Process
    H->>C: Response" />

## Middleware Levels

### HTTP Middleware (Use)

Runs **before** extraction, operates on raw HTTP requests.

```go
router := espresso.Portafilter()
    .Use(httpmiddleware.RequestIDMiddleware())
    .Use(httpmiddleware.LoggingMiddleware())
    .Use(httpmiddleware.CORSMiddleware(httpmiddleware.DefaultCORSConfig))
    .Use(httpmiddleware.RecoverMiddleware())
    .Use(httpmiddleware.CompressMiddleware())
```

Use for:
- Request/response logging
- CORS headers
- Compression
- Rate limiting
- Authentication (token validation)
- Request ID generation

### Service Layers (PostWith, GetWith, etc.)

Runs **after** extraction, operates on typed `Req/Res` pairs.

```go
type UserService struct{}

func (s UserService) Call(ctx context.Context, req CreateUserReq) (User, error) {
    // Business logic
}

router.Post("/users", UserService{},
    servicemiddleware.TimeoutLayer[CreateUserReq, User](5*time.Second),
    servicemiddleware.RetryLayer[CreateUserReq, User](3, 100*time.Millisecond, servicemiddleware.BackoffExponential),
)
```

Use for:
- Retry logic
- Timeouts (at business logic level)
- Circuit breakers
- Concurrency limits
- Request validation
- Metrics collection

## Choosing the Right Level

| Use Case | Level | Why |
|----------|-------|-----|
| CORS | HTTP | Operates on raw request headers |
| Compression | HTTP | Operates on response body |
| Rate Limiting | HTTP | Before routing/ extraction |
| Request Logging | HTTP | Logs method, path, status |
| Retry | Service | Retry typed operations |
| Timeout | Both | HTTP for overall, Service for business logic |
| Authentication | HTTP | Before extraction |
| Validation | Service | After extraction |
| Circuit Breaker | Service | Protect service calls |
| Metrics | Both | Different granularity |

## Order of Application

Middleware runs in reverse order (last added = first executed):

```go
router := espresso.Portafilter()
    .Use(mw1()) // Executes 4th (outermost)
    .Use(mw2()) // Executes 3rd
    .Use(mw3()) // Executes 2nd
    .Use(mw4()) // Executes 1st (innermost to handler)
```

For service layers:

```go
// Layers are applied in order provided
PostWith("/api", handler, 
    layer1, // Outermost
    layer2, // Middle
    layer3, // Innermost
)
```

## Common Patterns

### Standard Production Setup

```go
import (
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
)

func main() {
    router := espresso.Portafilter().
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.LoggingMiddleware()).
        Use(httpmiddleware.RecoverMiddleware()).
        Use(httpmiddleware.CORSMiddleware(config.CORS)).
        Use(httpmiddleware.RateLimitMiddleware(limiter))
    
    // Health check (no service middleware)
    router.Get("/health", func() string { return "ok" })
    
    // API routes with service layers
    router.PostWith("/users", UserService{},
        servicemiddleware.TimeoutLayer[CreateUserReq, User](30*time.Second),
        servicemiddleware.CircuitBreakerLayer[CreateUserReq, User](cbConfig),
    )
    
    router.Brew()
}
```

### Authentication Flow

```go
type AuthValidator struct{}

func (v AuthValidator) Validate(r *http.Request) (context.Context, error) {
    token := r.Header.Get("Authorization")
    claims, err := validateToken(token)
    if err != nil {
        return nil, err
    }
    ctx := context.WithValue(r.Context(), authKey{}, claims)
    return ctx, nil
}

func main() {
    router := espresso.Portafilter().
        Use(httpmiddleware.AuthMiddleware(AuthValidator{})).
        Use(httpmiddleware.LoggingMiddleware())
    
    // All routes require auth
    router.Get("/profile", getProfile)
    
    router.Brew()
}
```