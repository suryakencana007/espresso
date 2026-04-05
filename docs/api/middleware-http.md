---
title: HTTP Middleware API Reference
description: HTTP middleware types and functions
---

# HTTP Middleware API Reference

Package `middleware/http` provides HTTP-level middleware.

```go
import httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
```

## Middleware Type

```go
type Middleware func(http.Handler) http.Handler
```

## Built-in Middleware

### RequestIDMiddleware

Generate or propagate request IDs:

```go
func RequestIDMiddleware() Middleware
func GetRequestID(ctx context.Context) string
```

### LoggingMiddleware

Log request method, path, status, duration:

```go
func LoggingMiddleware() Middleware
```

Requires zerolog.

### RecoverMiddleware

Recover from panics:

```go
func RecoverMiddleware() Middleware
```

### CORSMiddleware

Handle CORS requests:

```go
type CORSConfig struct {
    AllowOrigins     []string
    AllowMethods     []string
    AllowHeaders     []string
    AllowCredentials bool
    ExposeHeaders    []string
    MaxAge           int
}

var DefaultCORSConfig CORSConfig

func CORSMiddleware(config CORSConfig) Middleware
```

### CompressMiddleware

Gzip compression:

```go
func CompressMiddleware() Middleware
```

### RateLimitMiddleware

Rate limiting:

```go
type RateLimiter interface {
    Allow(key string) bool
}

func RateLimitMiddleware(limiter RateLimiter) Middleware
```

### AuthMiddleware

Authentication:

```go
type AuthValidator interface {
    Validate(r *http.Request) (context.Context, error)
}

func AuthMiddleware(validator AuthValidator) Middleware
```

## Rate Limiters

### TokenBucketLimiter

```go
func NewTokenBucketLimiter(rate, capacity int) *TokenBucketLimiter
func NewTokenBucketLimiterPerKey(rate, capacity int) *TokenBucketLimiter
```

### SlidingWindowLimiter

```go
func NewSlidingWindowLimiter(window time.Duration, maxReq int) *SlidingWindowLimiter
func NewSlidingWindowLimiterWithCleanup(window time.Duration, maxReq int, cleanupInterval time.Duration) *SlidingWindowLimiter
```

## Utility Functions

### MiddlewareChain

Combine multiple middleware:

```go
func MiddlewareChain(middleware ...Middleware) Middleware
```

## Context Keys

```go
type RequestIDKey = struct{}
type AuthKey = struct{}
```

## Example

```go
func main() {
    router := espresso.Portafilter().
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.LoggingMiddleware()).
        Use(httpmiddleware.RecoverMiddleware()).
        Use(httpmiddleware.CORSMiddleware(httpmiddleware.DefaultCORSConfig)).
        Use(httpmiddleware.CompressMiddleware()).
        Use(httpmiddleware.RateLimitMiddleware(
            httpmiddleware.NewSlidingWindowLimiter(time.Minute, 1000),
        ))
    
    router.Brew()
}
```

## See Also

- [HTTP Middleware Guide](/guide/middleware/http) - Detailed usage
- [Middleware Overview](/guide/middleware/) - Architecture