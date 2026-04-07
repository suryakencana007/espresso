# Middleware Stack

This example shows how to configure a complete production middleware stack.

## Overview

Espresso provides two levels of middleware:

1. **HTTP Middleware** - Operates on raw HTTP requests (Use)
2. **Service Layers** - Operates on typed requests (PostWith, GetWith)

## Production HTTP Middleware

```go
package main

import (
    "time"
    
    "github.com/suryakencana007/espresso"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func main() {
    // Configure logging
    zerolog.SetGlobalLevel(zerolog.InfoLevel)
    
    router := espresso.Portafilter().
        // 1. Request ID - First, for tracing
        Use(httpmiddleware.RequestIDMiddleware()).
        
        // 2. Recovery - Catch panics
        Use(httpmiddleware.RecoverMiddleware()).
        
        // 3. Logging - Log all requests
        Use(httpmiddleware.LoggingMiddleware()).
        
        // 4. CORS - Handle cross-origin requests
        Use(httpmiddleware.CORSMiddleware(httpmiddleware.CORSConfig{
            AllowOrigins:     []string{"https://example.com", "https://api.example.com"},
            AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
            AllowHeaders:     []string{"Content-Type", "Authorization", "X-Request-ID"},
            AllowCredentials: true,
            ExposeHeaders:    []string{"X-Request-ID", "X-Total-Count"},
            MaxAge:           86400,
        })).
        
        // 5. Compression - Gzip responses
        Use(httpmiddleware.CompressMiddleware()).
        
        // 6. Rate Limiting - Prevent abuse
        Use(httpmiddleware.RateLimitMiddleware(
            httpmiddleware.NewSlidingWindowLimiter(time.Minute, 1000), // 1000 req/min
        ))
    
    // Routes
    router.Get("/health", healthHandler)
    router.Get("/api/users", listUsers)
    router.Post("/api/users", createUser)
    
    router.Brew()
}
```

## Middleware Order

The order matters! Middleware runs in reverse order:

```
Request -> RateLimit -> Compress -> CORS -> Logging -> Recover -> RequestID -> Handler
```

Recommended order:

| Position | Middleware | Why |
|----------|------------|-----|
| 1 | Request ID | Generate trace ID first |
| 2 | Recovery | Catch panics early |
| 3 | Logging | Log with request ID |
| 4 | CORS | Handle preflight before auth |
| 5 | Compression | Compress responses |
| 6 | Rate Limit | Limit before expensive operations |
| 7 | Auth | Validate credentials last |

## Service Layers

For typed request/response handling:

```go
import servicemiddleware "github.com/suryakencana007/espresso/middleware/service"

type UserService struct {
    db *sql.DB
}

func (s UserService) Call(ctx context.Context, req CreateUserReq) (User, error) {
    // Business logic
}

func main() {
    router := espresso.Portafilter().
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.LoggingMiddleware())
    
    // With service layers
    router.PostWith("/api/users", UserService{},
        servicemiddleware.TimeoutLayer[CreateUserReq, User](30*time.Second),
        servicemiddleware.ValidationLayer[CreateUserReq, User](UserValidator{}),
        servicemiddleware.MetricsLayer[CreateUserReq, User](metricsCollector, "UserService"),
    )
    
    router.Brew()
}
```

## Circuit Breaker Pattern

Protect your service from cascading failures:

```go
func main() {
    cbConfig := servicemiddleware.CircuitBreakerConfig{
        ServiceName:      "external-api",
        FailureThreshold: 5,              // Open after 5 failures
        Timeout:          30*time.Second, // Wait before half-open
        SuccessThreshold: 3,              // Close after 3 successes
    }
    
    router := espresso.Portafilter()
    
    router.PostWith("/api/external", ExternalService{},
        servicemiddleware.CircuitBreakerLayer[Req, Res](cbConfig),
    )
    
    router.Brew()
}

// Handle circuit breaker errors
func externalHandler(ctx context.Context, req *espresso.JSON[Req]) (espresso.JSON[Res], error) {
    // Service layer handles circuit breaker
    // If open, returns CircuitBreakerError
}
```

## Retry with Backoff

Retry transient failures:

```go
func main() {
    router := espresso.Portafilter()
    
    router.PostWith("/api/unstable", UnstableService{},
        // Timeout after 30s total
        servicemiddleware.TimeoutLayer[Req, Res](30*time.Second),
        
        // Retry up to 3 times with exponential backoff
        servicemiddleware.RetryLayer[Req, Res](
            3,                                      // max retries
            100*time.Millisecond,                   // initial backoff
            servicemiddleware.BackoffExponential,   // double each time
        ),
    )
    
    router.Brew()
}
```

## Authentication Middleware

Custom authentication:

```go
type JWTValidator struct {
    secret []byte
}

func (v JWTValidator) Validate(r *http.Request) (context.Context, error) {
    auth := r.Header.Get("Authorization")
    if auth == "" {
        return nil, errors.New("missing authorization header")
    }
    
    token := strings.TrimPrefix(auth, "Bearer ")
    claims, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
        return v.secret, nil
    })
    if err != nil {
        return nil, err
    }
    
    // Add claims to context
    ctx := context.WithValue(r.Context(), userKey{}, claims)
    return ctx, nil
}

func main() {
    router := espresso.Portafilter()
    
    // Public routes (no auth)
    router.Get("/health", healthHandler)
    router.Post("/login", loginHandler)
    
    // Protected routes (require auth)
    protected := espresso.Portafilter().
        Use(httpmiddleware.AuthMiddleware(JWTValidator{secret: config.JWTSecret}))
    
    protected.Get("/profile", getProfile)
    protected.Put("/profile", updateProfile)
    
    router.Brew()
}
```

## Request Context Values

Pass data through middleware:

```go
type userKey struct{}

func AuthMiddleware() httpmiddleware.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := authenticate(r)
            ctx := context.WithValue(r.Context(), userKey{}, user)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Access in handler
func handler(ctx context.Context, req *espresso.JSON[Req]) (espresso.JSON[Res], error) {
    user := ctx.Value(userKey{}).(User)
    // ...
}
```

## Rate Limiting Strategies

### Token Bucket

Good for bursty traffic:

```go
// Global limit
limiter := httpmiddleware.NewTokenBucketLimiter(100, 100) // 100 req/sec

// Per-client limit (by IP)
limiter := httpmiddleware.NewTokenBucketLimiterPerKey(10, 10) // 10 req/sec per IP
router.Use(httpmiddleware.RateLimitMiddleware(limiter))
```

### Sliding Window

More accurate time-based limiting:

```go
// 1000 requests per minute per IP
limiter := httpmiddleware.NewSlidingWindowLimiter(time.Minute, 1000)
router.Use(httpmiddleware.RateLimitMiddleware(limiter))
```

## Metrics Collection

Integrate with Prometheus:

```go
type PrometheusMetrics struct {
    requestsTotal   *prometheus.CounterVec
    requestsLatency *prometheus.HistogramVec
    activeRequests  *prometheus.GaugeVec
}

func (m *PrometheusMetrics) RecordRequest(service string, duration time.Duration, err error) {
    status := "success"
    if err != nil {
        status = "error"
    }
    m.requestsTotal.WithLabelValues(service, status).Inc()
    m.requestsLatency.WithLabelValues(service).Observe(duration.Seconds())
}

func (m *PrometheusMetrics) RecordActiveRequests(service string, delta int) {
    m.activeRequests.WithLabelValues(service).Add(float64(delta))
}

func main() {
    metrics := NewPrometheusMetrics()
    
    router := espresso.Portafilter()
    
    router.PostWith("/api/users", UserService{},
        servicemiddleware.MetricsLayer[CreateUserReq, User](metrics, "UserService"),
    )
    
    // Expose metrics endpoint
    router.Get("/metrics", prometheusHandler())
    
    router.Brew()
}
```

## Combined Production Setup

Complete example:

```go
package main

import (
    "time"
    
    "github.com/suryakencana007/espresso"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
)

func main() {
    // Rate limiter
    rateLimiter := httpmiddleware.NewSlidingWindowLimiter(time.Minute, 1000)
    
    // Circuit breaker
    cbConfig := servicemiddleware.CircuitBreakerConfig{
        ServiceName:      "api",
        FailureThreshold: 5,
        Timeout:          30*time.Second,
        SuccessThreshold: 3,
    }
    
    // HTTP middleware stack
    router := espresso.Portafilter().
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.RecoverMiddleware()).
        Use(httpmiddleware.LoggingMiddleware()).
        Use(httpmiddleware.CORSMiddleware(httpmiddleware.DefaultCORSConfig)).
        Use(httpmiddleware.CompressMiddleware()).
        Use(httpmiddleware.RateLimitMiddleware(rateLimiter))
    
    // Health check (no service layers)
    router.Get("/health", func() string { return "OK" })
    
    // API routes (with service layers)
    router.PostWith("/api/users", UserService{},
        servicemiddleware.TimeoutLayer[CreateUserReq, User](30*time.Second),
        servicemiddleware.MetricsLayer[CreateUserReq, User](metrics, "UserService"),
    )
    
    // External API (with circuit breaker + retry)
    router.PostWith("/api/external", ExternalService{},
        servicemiddleware.TimeoutLayer[Req, Res](10*time.Second),
        servicemiddleware.RetryLayer[Req, Res](3, 100*time.Millisecond, 
            servicemiddleware.BackoffExponential),
        servicemiddleware.CircuitBreakerLayer[Req, Res](cbConfig),
    )
    
    router.Brew(espresso.WithAddr(":8080"))
}
```