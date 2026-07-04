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

### Service Layers (WithLayers)

Runs **after** extraction, operates on typed `Req/Res` pairs. Attach them to a
route by wrapping the handler in `espresso.WithLayers(handler, layers...)`:

```go
func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    // Business logic
}

router.Post("/users", espresso.WithLayers(createUser,
    espresso.Timeout(5*time.Second),
    espresso.Retry(3, 100*time.Millisecond, servicemiddleware.BackoffExponential),
))
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

Middleware runs in the order registered — **first added is outermost and
executes first** (`router.go:208-215` wraps last-to-first, so `middleware[0]`
ends up as the outermost wrapper). A request flows through the wrappers in
the order they were added; the response unwinds in the reverse order.

```go
router := espresso.Portafilter()
router.Use(mw1()) // Executes 1st (outermost)
router.Use(mw2()) // Executes 2nd
router.Use(mw3()) // Executes 3rd
router.Use(mw4()) // Executes 4th (innermost to handler)
```

Because middleware is bound at route-registration time (`Get`/`Post`/… snapshot
the middleware slice as they run), any `Use()` call after registering a route
does not apply to that route — call `Use()` first, then register routes.

For service layers via `espresso.WithLayers` (or `BuildService`), the same rule
holds: first added is outermost.

```go
router.Post("/api", espresso.WithLayers(handler,
    layer1, // Outermost (executes first)
    layer2, // Middle
    layer3, // Innermost (executes last, closest to handler)
))
```

## Common Patterns

### Standard Production Setup

```go
import (
    httpmiddleware "github.com/suryakencana007/espresso/v2/middleware/http"
    servicemiddleware "github.com/suryakencana007/espresso/v2/middleware/service"
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
    router.Post("/users", espresso.WithLayers(createUser,
        espresso.Timeout(30*time.Second),
        espresso.CircuitBreaker(cbConfig),
    ))
    
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