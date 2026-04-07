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

### JWTMiddleware

JWT token validation:

```go
type JWTConfig struct {
    Secret          string
    SigningMethod   string
    TokenLookup     string
    TokenHeader     string
    ContextKey      string
    Skipper         func(r *http.Request) bool
    ClaimsExtractor func(token string) (map[string]any, error)
}

func JWTMiddleware(config JWTConfig) Middleware
func GetClaims(ctx context.Context, key string) map[string]any
```

Example:

```go
config := httpmiddleware.JWTConfig{
    Secret: "your-secret-key",
    SigningMethod: "HS256",
    TokenLookup: "header:Authorization",
    TokenHeader: "Bearer",
    ContextKey: "user",
    ClaimsExtractor: func(token string) (map[string]any, error) {
        // Parse JWT and return claims
        return claims, nil
    },
}

router.Use(httpmiddleware.JWTMiddleware(config))
```

### BasicAuthMiddleware

HTTP Basic Authentication:

```go
type BasicAuthConfig struct {
    Realm     string
    Users     map[string]string
    Skipper   func(r *http.Request) bool
    Validator func(username, password string) bool
}

func BasicAuthMiddleware(config BasicAuthConfig) Middleware
func GetUsername(ctx context.Context) string
```

Example:

```go
config := httpmiddleware.BasicAuthConfig{
    Realm: "API",
    Users: map[string]string{
        "admin": "password123",
    },
}

router.Use(httpmiddleware.BasicAuthMiddleware(config))
```

### APIKeyMiddleware

API Key validation:

```go
type APIKeyConfig struct {
    Keys        []string
    KeyLookup   string
    ContextKey  string
    Skipper     func(r *http.Request) bool
    KeyValidator func(key string) bool
}

func APIKeyMiddleware(config APIKeyConfig) Middleware
func GetAPIKey(ctx context.Context, key string) string
```

Example:

```go
config := httpmiddleware.APIKeyConfig{
    Keys: []string{"key-123", "key-456"},
    KeyLookup: "header:X-API-Key",
}

router.Use(httpmiddleware.APIKeyMiddleware(config))
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