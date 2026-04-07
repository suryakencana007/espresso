# HTTP Middleware

HTTP middleware operates on raw HTTP requests before route extraction. Use `router.Use()` to add middleware.

## Built-in Middleware

### Request ID

Generates unique IDs for request tracing:

```go
router.Use(httpmiddleware.RequestIDMiddleware())

// Access in handlers
func handler(ctx context.Context, req *espresso.JSON[Req]) (Response, error) {
    requestID := httpmiddleware.GetRequestID(ctx)
    // or from context
    requestID = ctx.Value(httpmiddleware.RequestIDKey{}).(string)
}
```

### Logging

Logs request method, path, status, and duration:

```go
router.Use(httpmiddleware.LoggingMiddleware())

// Output:
// INFO request method=GET path=/api/users status=200 duration=15.234ms request_id=abc123
```

Requires zerolog:

```go
import "github.com/rs/zerolog/log"
```

### Recovery

Recovers from panics and returns HTTP 500:

```go
router.Use(httpmiddleware.RecoverMiddleware())

// Panics become HTTP 500 responses
```

### CORS

Handles Cross-Origin Resource Sharing:

```go
config := httpmiddleware.CORSConfig{
    AllowOrigins:     []string{"https://example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
    ExposeHeaders:    []string{"X-Custom-Header"},
    MaxAge:           86400,
}
router.Use(httpmiddleware.CORSMiddleware(config))

// Or use defaults
router.Use(httpmiddleware.CORSMiddleware(httpmiddleware.DefaultCORSConfig))
```

### Compression

Gzip compression for responses:

```go
router.Use(httpmiddleware.CompressMiddleware())

// Only compresses if client sends Accept-Encoding: gzip
```

### Rate Limiting

Prevent abuse with rate limiting:

```go
// Token bucket - global limit
limiter := httpmiddleware.NewTokenBucketLimiter(100, 100) // 100 req/sec
router.Use(httpmiddleware.RateLimitMiddleware(limiter))

// Token bucket - per-key limit (e.g., per IP)
limiter := httpmiddleware.NewTokenBucketLimiterPerKey(10, 10) // 10 req/sec per IP
router.Use(httpmiddleware.RateLimitMiddleware(limiter))

// Sliding window - more accurate
limiter := httpmiddleware.NewSlidingWindowLimiter(time.Minute, 100) // 100 req/min
router.Use(httpmiddleware.RateLimitMiddleware(limiter))
```

### Authentication

Validate authentication before routing:

#### JWT Middleware

```go
import "github.com/golang-jwt/jwt/v5"

config := httpmiddleware.JWTConfig{
    Secret: "your-secret-key",
    SigningMethod: "HS256",
    TokenLookup: "header:Authorization",
    TokenHeader: "Bearer",
    ContextKey: "user",
    ClaimsExtractor: func(token string) (map[string]any, error) {
        parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
            return []byte("your-secret-key"), nil
        })
        if err != nil {
            return nil, err
        }
        return parsed.Claims.(jwt.MapClaims), nil
    },
}

router.Use(httpmiddleware.JWTMiddleware(config))

// Access claims in handler
func handler(ctx context.Context, req *espresso.JSON[Req]) (Response, error) {
    claims := httpmiddleware.GetClaims(ctx, "user")
    userID := claims["sub"].(string)
    // ...
}
```

#### Basic Auth Middleware

```go
config := httpmiddleware.BasicAuthConfig{
    Realm: "API",
    Users: map[string]string{
        "admin": "password123",
    },
}

router.Use(httpmiddleware.BasicAuthMiddleware(config))

// Access username in handler
func handler(ctx context.Context, req *espresso.JSON[Req]) (Response, error) {
    username := httpmiddleware.GetUsername(ctx)
    // ...
}
```

With custom validator:

```go
config := httpmiddleware.BasicAuthConfig{
    Realm: "API",
    Validator: func(username, password string) bool {
        // Check against database
        return validateUser(username, password)
    },
}
```

#### API Key Middleware

```go
config := httpmiddleware.APIKeyConfig{
    Keys: []string{"key-123", "key-456"},
    KeyLookup: "header:X-API-Key",
}

router.Use(httpmiddleware.APIKeyMiddleware(config))

// Access API key in handler
func handler(ctx context.Context, req *espresso.JSON[Req]) (Response, error) {
    apiKey := httpmiddleware.GetAPIKey(ctx, "api_key")
    // ...
}
```

With custom validator:

```go
config := httpmiddleware.APIKeyConfig{
    KeyLookup: "header:X-API-Key",
    KeyValidator: func(key string) bool {
        // Check against database
        return validateAPIKey(key)
    },
}
```

#### Skipping Authentication

Use `Skipper` to bypass authentication for specific routes:

```go
config := httpmiddleware.JWTConfig{
    Secret: "secret",
    Skipper: func(r *http.Request) bool {
        // Skip auth for health check and login
        return r.URL.Path == "/health" || r.URL.Path == "/login"
    },
    ClaimsExtractor: extractClaims,
}
```

#### Multiple Token Sources

Token can be extracted from multiple sources:

```go
// From header (default)
TokenLookup: "header:Authorization"

// From query parameter
TokenLookup: "query:token"

// From cookie
TokenLookup: "cookie:jwt"
```

#### Custom AuthValidator

For custom authentication logic:

```go
type JWTValidator struct {
    secret []byte
}

func (v JWTValidator) Validate(r *http.Request) (context.Context, error) {
    token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
    claims, err := validateJWT(token, v.secret)
    if err != nil {
        return nil, err
    }
    ctx := context.WithValue(r.Context(), authKey{}, claims)
    return ctx, nil
}

func main() {
    router := espresso.Portafilter().
        Use(httpmiddleware.AuthMiddleware(JWTValidator{secret: config.JWTSecret}))
    
    // All routes require valid JWT
    router.Get("/protected", handler)
}
```

## Custom Middleware

Create your own middleware:

```go
func CustomMiddleware() httpmiddleware.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Before: pre-processing
            start := time.Now()
            
            // Call next handler
            next.ServeHTTP(w, r)
            
            // After: post-processing
            duration := time.Since(start)
            log.Printf("Request took %v", duration)
        })
    }
}

router.Use(CustomMiddleware())
```

### Context Values

Pass data through context:

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
func handler(ctx context.Context, req *espresso.JSON[Req]) (Response, error) {
    user := ctx.Value(userKey{}).(User)
    // ...
}
```

## Middleware Chain

Combine multiple middleware:

```go
func main() {
    router := espresso.Portafilter().
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.LoggingMiddleware()).
        Use(httpmiddleware.RecoverMiddleware()).
        Use(httpmiddleware.CORSMiddleware(config.CORS)).
        Use(httpmiddleware.CompressMiddleware()).
        Use(httpmiddleware.RateLimitMiddleware(limiter)).
        Use(httpmiddleware.AuthMiddleware(validator))
    
    // Routes
    router.Get("/public", publicHandler)    // All middleware still applies
    router.Post("/private", privateHandler)
    
    router.Brew()
}
```

## Per-Route Middleware

Apply middleware to specific routes:

```go
func main() {
    router := espresso.Portafilter()
    
    // Public routes - no auth
    router.Get("/health", healthHandler)
    router.Get("/public", publicHandler)
    
    // Protected routes - add auth middleware
    protected := espresso.Portafilter().
        Use(httpmiddleware.AuthMiddleware(validator))
    protected.Get("/profile", profileHandler)
    protected.Post("/data", dataHandler)
    
    // Combine routers (advanced)
    // Note: This requires manual handler wrapping
}
```

Alternatively, use a closure:

```go
func withAuth(handler any) any {
    return func(w http.ResponseWriter, r *http.Request) {
        // Auth check
        if err := validateAuth(r); err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        // Call actual handler
        // ...
    }
}
```

## Best Practices

1. **Order matters**: Add middleware in logical order
   - Request ID first (for tracing)
   - Recovery early (to catch panics)
   - Logging after recovery
   - CORS before auth

2. **Keep it simple**: One responsibility per middleware

3. **Use context**: Pass data through context, not globals

4. **Pool resources**: Use sync.Pool for expensive objects

5. **Short-circuit**: Return early on errors

```go
func TimeoutMiddleware(timeout time.Duration) httpmiddleware.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx, cancel := context.WithTimeout(r.Context(), timeout)
            defer cancel()
            
            done := make(chan struct{})
            go func() {
                defer close(done)
                next.ServeHTTP(w, r.WithContext(ctx))
            }()
            
            select {
            case <-done:
                return
            case <-ctx.Done():
                http.Error(w, "Request timeout", http.StatusRequestTimeout)
            }
        })
    }
}
```