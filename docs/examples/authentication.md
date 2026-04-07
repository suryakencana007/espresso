---
title: Authentication
description: Protect routes with JWT, Basic Auth, and API Key authentication
---

# Authentication Example

This example shows how to implement authentication with Espresso.

## JWT Authentication

### Setup

```go
package main

import (
    "context"
    "net/http"
    
    "github.com/golang-jwt/jwt/v5"
    "github.com/suryakencana007/espresso"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
)

func main() {
    router := espresso.Portafilter()
    
    // JWT middleware
    jwtConfig := httpmiddleware.JWTConfig{
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
    
    router.Use(httpmiddleware.JWTMiddleware(jwtConfig))
    
    // Protected routes
    router.Get("/profile", espresso.Doppio(getProfile))
    
    router.Brew(espresso.WithAddr(":8080"))
}
```

### Access Claims in Handler

```go
type ProfileResponse struct {
    UserID string `json:"user_id"`
    Email  string `json:"email"`
}

func getProfile(ctx context.Context, req *espresso.JSON[struct{}]) (espresso.JSON[ProfileResponse], error) {
    claims := httpmiddleware.GetClaims(ctx, "user")
    
    return espresso.JSON[ProfileResponse]{
        Data: ProfileResponse{
            UserID: claims["sub"].(string),
            Email:  claims["email"].(string),
        },
    }, nil
}
```

### Skip Authentication for Health Check

```go
jwtConfig := httpmiddleware.JWTConfig{
    Secret: "your-secret-key",
    Skipper: func(r *http.Request) bool {
        // Skip auth for health check and login
        return r.URL.Path == "/health" || r.URL.Path == "/login"
    },
    ClaimsExtractor: extractClaims,
}
```

### Login Endpoint

```go
type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type LoginResponse struct {
    Token string `json:"token"`
}

func loginHandler(ctx context.Context, req *espresso.JSON[LoginRequest]) (espresso.JSON[LoginResponse], error) {
    // Validate credentials (use database)
    if req.Data.Email != "user@example.com" || req.Data.Password != "password" {
        return espresso.JSON[LoginResponse]{}, fmt.Errorf("invalid credentials")
    }
    
    // Generate token
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub":   "user123",
        "email": req.Data.Email,
        "exp":   time.Now().Add(24 * time.Hour).Unix(),
    })
    
    tokenString, err := token.SignedString([]byte("your-secret-key"))
    if err != nil {
        return espresso.JSON[LoginResponse]{}, err
    }
    
    return espresso.JSON[LoginResponse]{
        Data: LoginResponse{Token: tokenString},
    }, nil
}

// Route (no auth required)
router.Post("/login", espresso.Doppio(loginHandler))
```

## Basic Authentication

### Setup

```go
func main() {
    router := espresso.Portafilter()
    
    // Basic Auth middleware
    basicAuthConfig := httpmiddleware.BasicAuthConfig{
        Realm: "API",
        Users: map[string]string{
            "admin": "admin123",
            "user":  "user123",
        },
    }
    
    router.Use(httpmiddleware.BasicAuthMiddleware(basicAuthConfig))
    
    // All routes require Basic Auth
    router.Get("/api/data", espresso.Doppio(getData))
    
    router.Brew(espresso.WithAddr(":8080"))
}
```

### Access Username in Handler

```go
func getData(ctx context.Context, req *espresso.JSON[struct{}]) (espresso.JSON[map[string]any], error) {
    username := httpmiddleware.GetUsername(ctx)
    
    return espresso.JSON[map[string]any]{
        Data: map[string]any{
            "message":  "Hello",
            "username": username,
        },
    }, nil
}
```

### Custom Validator

```go
basicAuthConfig := httpmiddleware.BasicAuthConfig{
    Realm: "API",
    Validator: func(username, password string) bool {
        // Check against database
        user, err := db.GetUser(username)
        if err != nil {
            return false
        }
        return bcrypt.CompareHashAndPassword(
            []byte(user.PasswordHash),
            []byte(password),
        ) == nil
    },
}

router.Use(httpmiddleware.BasicAuthMiddleware(basicAuthConfig))
```

### Skip Authentication for Public Routes

```go
basicAuthConfig := httpmiddleware.BasicAuthConfig{
    Realm: "API",
    Users: map[string]string{"admin": "password"},
    Skipper: func(r *http.Request) bool {
        return r.URL.Path == "/health" || 
               r.URL.Path == "/public" ||
               strings.HasPrefix(r.URL.Path, "/api/v1/public/")
    },
}
```

## API Key Authentication

### Setup

```go
func main() {
    router := espresso.Portafilter()
    
    // API Key middleware
    apiKeyConfig := httpmiddleware.APIKeyConfig{
        Keys: []string{
            "sk-api-key-123",
            "sk-api-key-456",
        },
        KeyLookup: "header:X-API-Key",
        ContextKey: "api_key",
    }
    
    router.Use(httpmiddleware.APIKeyMiddleware(apiKeyConfig))
    
    // All routes require API key
    router.Get("/api/data", espresso.Doppio(getData))
    
    router.Brew(espresso.WithAddr(":8080"))
}
```

### Access API Key in Handler

```go
func getData(ctx context.Context, req *espresso.JSON[struct{}]) (espresso.JSON[map[string]any], error) {
    apiKey := httpmiddleware.GetAPIKey(ctx, "api_key")
    
    // Log API usage
    logUsage(apiKey, "GET", "/api/data")
    
    return espresso.JSON[map[string]any]{
        Data: map[string]any{
            "message": "Success",
            "key":      apiKey,
        },
    }, nil
}
```

### Key Lookup Sources

```go
// From header (default)
KeyLookup: "header:X-API-Key"

// From query parameter
KeyLookup: "query:api_key"

// From cookie
KeyLookup: "cookie:api_key"
```

### Custom Validator

```go
apiKeyConfig := httpmiddleware.APIKeyConfig{
    KeyLookup: "header:X-API-Key",
    KeyValidator: func(key string) bool {
        // Check against database
        valid, err := db.IsValidAPIKey(key)
        return valid && err == nil
    },
}

router.Use(httpmiddleware.APIKeyMiddleware(apiKeyConfig))
```

### Rate Limiting per API Key

```go
func main() {
    router := espresso.Portafilter()
    
    // API Key middleware
    router.Use(httpmiddleware.APIKeyMiddleware(httpmiddleware.APIKeyConfig{
        KeyLookup: "header:X-API-Key",
        KeyValidator: validateAPIKey,
    }))
    
    // Rate limiting per key
    limiter := httpmiddleware.NewTokenBucketLimiterPerKey(100, 100)
    router.Use(httpmiddleware.RateLimitMiddleware(limiter))
    
    router.Brew()
}
```

## Multiple Authentication Methods

### Route-Specific Authentication

```go
func main() {
    router := espresso.Portafilter()
    
    // Public routes
    router.Get("/health", espresso.Ristretto(healthHandler))
    router.Post("/login", espresso.Doppio(loginHandler))
    
    // JWT protected routes
    apiRouter := espresso.Portafilter()
    apiRouter.Use(httpmiddleware.JWTMiddleware(jwtConfig))
    apiRouter.Get("/profile", espresso.Doppio(getProfile))
    apiRouter.Put("/profile", espresso.Lungo(updateProfile))
    
    // Admin routes with Basic Auth
    adminRouter := espresso.Portafilter()
    adminRouter.Use(httpmiddleware.BasicAuthMiddleware(adminAuthConfig))
    adminRouter.Get("/admin/users", espresso.Doppio(listUsers))
    
    // Combine routes manually or use different ports
    // ...
}
```

### Middleware Chain

```go
router := espresso.Portafilter().
    Use(httpmiddleware.RequestIDMiddleware()).
    Use(httpmiddleware.RecoverMiddleware()).
    Use(httpmiddleware.LoggingMiddleware()).
    Use(httpmiddleware.JWTMiddleware(jwtConfig)).
    Use(httpmiddleware.RateLimitMiddleware(limiter))

router.Get("/api/protected", espresso.Doppio(protectedHandler))

router.Brew()
```

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"
    
    "github.com/golang-jwt/jwt/v5"
    "github.com/suryakencana007/espresso"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
)

var jwtSecret = []byte("your-secret-key")

func main() {
    router := espresso.Portafilter()
    
    // Public routes
    router.Get("/health", espresso.Ristretto(healthHandler))
    router.Post("/login", espresso.Doppio(loginHandler))
    
    // Protected routes (JWT required)
    protected := espresso.Portafilter()
    protected.Use(httpmiddleware.JWTConfig{
        Secret: string(jwtSecret),
        SigningMethod: "HS256",
        TokenLookup: "header:Authorization",
        TokenHeader: "Bearer",
        ContextKey: "user",
        ClaimsExtractor: extractClaims,
    })
    
    protected.Get("/profile", espresso.Doppio(getProfile))
    
    // API Key routes
    apiRouter := espresso.Portafilter()
    apiRouter.Use(httpmiddleware.APIKeyMiddleware(httpmiddleware.APIKeyConfig{
        Keys: []string{"sk-test-key"},
        KeyLookup: "header:X-API-Key",
    }))
    apiRouter.Get("/api/data", espresso.Doppio(getData))
    
    fmt.Println("Server starting on :8080")
    router.Brew(espresso.WithAddr(":8080"))
}

func healthHandler() string {
    return "OK"
}

func extractClaims(token string) (map[string]any, error) {
    parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
        return jwtSecret, nil
    })
    if err != nil {
        return nil, err
    }
    return parsed.Claims.(jwt.MapClaims), nil
}

func loginHandler(ctx context.Context, req *espresso.JSON[LoginRequest]) (espresso.JSON[LoginResponse], error) {
    // Validate (use database in production)
    if req.Data.Email != "user@example.com" || req.Data.Password != "password" {
        return espresso.JSON[LoginResponse]{}, fmt.Errorf("invalid credentials")
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub":   "user123",
        "email": req.Data.Email,
        "exp":   time.Now().Add(24 * time.Hour).Unix(),
    })
    
    tokenString, _ := token.SignedString(jwtSecret)
    
    return espresso.JSON[LoginResponse]{
        Data: LoginResponse{Token: tokenString},
    }, nil
}

func getProfile(ctx context.Context, req *espresso.JSON[struct{}]) (espresso.JSON[ProfileResponse], error) {
    claims := httpmiddleware.GetClaims(ctx, "user")
    return espresso.JSON[ProfileResponse]{
        Data: ProfileResponse{
            UserID: claims["sub"].(string),
            Email:  claims["email"].(string),
        },
    }, nil
}

func getData(ctx context.Context, req *espresso.JSON[struct{}]) (espresso.JSON[map[string]any], error) {
    return espresso.JSON[map[string]any]{
        Data: map[string]any{
            "message": "Success",
            "time":    time.Now().Format(time.RFC3339),
        },
    }, nil
}

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type LoginResponse struct {
    Token string `json:"token"`
}

type ProfileResponse struct {
    UserID string `json:"user_id"`
    Email  string `json:"email"`
}
```

## Testing

### Test JWT Authentication

```bash
# Login
curl -X POST http://localhost:8080/login \
    -H "Content-Type: application/json" \
    -d '{"email":"user@example.com","password":"password"}'

# Access protected route
curl http://localhost:8080/profile \
    -H "Authorization: Bearer <token>"
```

### Test Basic Auth

```bash
# With credentials
curl -u admin:admin123 http://localhost:8080/api/data
```

### Test API Key

```bash
# With API key header
curl http://localhost:8080/api/data \
    -H "X-API-Key: sk-test-key"
```

## See Also

- [HTTP Middleware Guide](/guide/middleware/http) - All middleware types
- [Handlers Guide](/guide/handlers) - Handler patterns
- [Production Example](/examples/production) - Production setup