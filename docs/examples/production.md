# Production Setup

Complete production-ready Espresso application configuration.

## Project Structure

```
myapp/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── handlers/
│   │   ├── health.go
│   │   └── user.go
│   ├── models/
│   │   └── user.go
│   ├── services/
│   │   └── user.go
│   ├── middleware/
│   │   └── auth.go
│   └── repository/
│       └── user.go
├── pkg/
│   └── validator/
│       └── validator.go
├── api/
│   └── openapi.yaml
├── deployments/
│   ├── Dockerfile
│   └── kubernetes/
├── configs/
│   ├── config.local.yaml
│   └── config.prod.yaml
├── go.mod
└── go.sum
```

## Configuration

```go
// internal/config/config.go
package config

import (
    "os"
    "time"
    
    "github.com/spf13/viper"
)

type Config struct {
    App      AppConfig
    Server   ServerConfig
    Database DatabaseConfig
    Redis    RedisConfig
    Auth     AuthConfig
    RateLimit RateLimitConfig
    CORS     CORSConfig
}

type AppConfig struct {
    Name    string `mapstructure:"name"`
    Version string `mapstructure:"version"`
    Debug   bool   `mapstructure:"debug"`
}

type ServerConfig struct {
    Host         string        `mapstructure:"host"`
    Port         int           `mapstructure:"port"`
    ReadTimeout  time.Duration `mapstructure:"read_timeout"`
    WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
    URL             string        `mapstructure:"url"`
   MaxOpenConns     int           `mapstructure:"max_open_conns"`
   MaxIdleConns     int           `mapstructure:"max_idle_conns"`
    ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
    Addr     string `mapstructure:"addr"`
    Password string `mapstructure:"password"`
    DB       int    `mapstructure:"db"`
}

type AuthConfig struct {
    JWTSecret string        `mapstructure:"jwt_secret"`
    TokenTTL time.Duration `mapstructure:"token_ttl"`
}

type RateLimitConfig struct {
    RequestsPerMinute int `mapstructure:"requests_per_minute"`
}

type CORSConfig struct {
    AllowOrigins     []string `mapstructure:"allow_origins"`
    AllowMethods     []string `mapstructure:"allow_methods"`
    AllowHeaders     []string `mapstructure:"allow_headers"`
    AllowCredentials bool     `mapstructure:"allow_credentials"`
}

func Load(configPath string) (*Config, error) {
    v := viper.New()
    
    v.SetConfigFile(configPath)
    v.AutomaticEnv()
    
    if err := v.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var config Config
    if err := v.Unmarshal(&config); err != nil {
        return nil, err
    }
    
    // Override with env vars
    if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
        config.Database.URL = dbURL
    }
    
    return &config, nil
}
```

## Application State

```go
// internal/app/app.go
package app

import (
    "context"
    "database/sql"
    
    "github.com/suryakencana007/espresso"
    "github.com/redis/go-redis/v9"
    "myapp/internal/config"
)

type State struct {
    Config  *config.Config
    DB      *sql.DB
    Redis   *redis.Client
    // Services
    UserService *UserService
    // Utilities
    Validator  *Validator
}

func NewState(cfg *config.Config) (*State, error) {
    // Initialize database
    db, err := sql.Open("postgres", cfg.Database.URL)
    if err != nil {
        return nil, err
    }
    db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
    db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
    db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
    
    // Initialize Redis
    redis := redis.NewClient(&redis.Options{
        Addr:     cfg.Redis.Addr,
        Password: cfg.Redis.Password,
        DB:       cfg.Redis.DB,
    })
    
    // Initialize services
    userRepo := NewUserRepository(db)
    userService := NewUserService(userRepo, redis)
    
    // Initialize utilities
    validator := NewValidator()
    
    return &State{
        Config:     cfg,
        DB:         db,
        Redis:      redis,
        UserService: userService,
        Validator:  validator,
    }, nil
}

func (s *State) Close() error {
    if s.DB != nil {
        s.DB.Close()
    }
    if s.Redis != nil {
        s.Redis.Close()
    }
    return nil
}
```

## Main Entry Point

```go
// cmd/server/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/rs/zerolog"
    "github.com/suryakencana007/espresso"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
    
    "myapp/internal/app"
    "myapp/internal/config"
    "myapp/internal/handlers"
    "myapp/internal/middleware"
)

func main() {
    // Load configuration
    cfg, err := config.Load("configs/config.prod.yaml")
    if err != nil {
        log.Fatal("Failed to load config:", err)
    }
    
    // Initialize state
    state, err := app.NewState(cfg)
    if err != nil {
        log.Fatal("Failed to initialize state:", err)
    }
    defer state.Close()
    
    // Configure logging
    logger := configureLogger(cfg)
    
    // Create router
    router := espresso.Portafilter().
        // HTTP middleware
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.RecoverMiddleware()).
        Use(httpmiddleware.LoggingMiddleware()).
        Use(httpmiddleware.CORSMiddleware(httpmiddleware.CORSConfig{
            AllowOrigins: cfg.CORS.AllowOrigins,
            AllowMethods: cfg.CORS.AllowMethods,
            AllowHeaders: cfg.CORS.AllowHeaders,
        })).
        Use(httpmiddleware.CompressMiddleware()).
        Use(httpmiddleware.RateLimitMiddleware(
            httpmiddleware.NewSlidingWindowLimiter(
                time.Minute,
                cfg.RateLimit.RequestsPerMinute,
            ),
        )).
        Use(middleware.AuthMiddleware(state)).
        // Application state
        WithState(state)
    
    // Register routes
    registerRoutes(router, state)
    
    // Start server with graceful shutdown
    startServer(router, cfg, logger)
}

func registerRoutes(router *espresso.Router, state *app.State) {
    // Health check (no auth)
    router.Get("/health", handlers.Health)
    router.Get("/ready", handlers.Ready(state))
    
    // API routes
    router.Get("/api/users", espresso.Doppio(handlers.ListUsers(state)))
    router.Get("/api/users/{id}", espresso.Doppio(handlers.GetUser(state)))
    router.Post("/api/users", espresso.Doppio(handlers.CreateUser(state)))
    router.Put("/api/users/{id}", espresso.Lungo(handlers.UpdateUser(state)))
    router.Delete("/api/users/{id}", espresso.Doppio(handlers.DeleteUser(state)))
}

func startServer(router *espresso.Router, cfg *config.Config, logger zerolog.Logger) {
    // Run server in goroutine
    go func() {
        addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
        logger.Info().Str("addr", addr).Msg("Starting server")
        
        if err := router.Brew(espresso.WithAddr(addr)); err != nil {
            logger.Fatal().Err(err).Msg("Server failed")
        }
    }()
    
    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    logger.Info().Msg("Shutting down server...")
    // Router handles graceful shutdown internally
}
```

## Request Validation

```go
// pkg/validator/validator.go
package validator

import (
    "github.com/go-playground/validator/v10"
)

type Validator struct {
    validate *validator.Validate
}

func New() *Validator {
    return &Validator{
        validate: validator.New(),
    }
}

func (v *Validator) Struct(s any) error {
    return v.validate.Struct(s)
}

// Custom validation types
type CreateUserReq struct {
    Name     string `json:"name" validate:"required,min=2,max=100"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,max=72"`
}

type UpdateUserReq struct {
    Name  string `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
    Email string `json:"email,omitempty" validate:"omitempty,email"`
}
```

## Error Handling

```go
// internal/errors/errors.go
package errors

import (
    "encoding/json"
    "net/http"
)

type APIError struct {
    StatusCode int      `json:"-"`
    Code       string   `json:"code"`
    Message    string   `json:"message"`
    Details    []Detail `json:"details,omitempty"`
}

type Detail struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

func (e *APIError) Error() string {
    return e.Message
}

func (e *APIError) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(e.StatusCode)
    return json.NewEncoder(w).Encode(e)
}

// Error constructors
func BadRequest(message string, details ...Detail) *APIError {
    return &APIError{
        StatusCode: http.StatusBadRequest,
        Code:       "BAD_REQUEST",
        Message:    message,
        Details:    details,
    }
}

func NotFound(message string) *APIError {
    return &APIError{
        StatusCode: http.StatusNotFound,
        Code:       "NOT_FOUND",
        Message:    message,
    }
}

func Internal(message string) *APIError {
    return &APIError{
        StatusCode: http.StatusInternalServerError,
        Code:       "INTERNAL_ERROR",
        Message:    message,
    }
}
```

## Handlers

```go
// internal/handlers/user.go
package handlers

import (
    "context"
    "net/http"
    
    "github.com/suryakencana007/espresso"
    
    "myapp/internal/app"
    "myapp/internal/errors"
    "myapp/internal/models"
)

func ListUsers(state *app.State) func(ctx context.Context, query *espresso.Query[models.ListQuery]) (espresso.JSON[models.ListResponse], error) {
    return func(ctx context.Context, query *espresso.Query[models.ListQuery]) (espresso.JSON[models.ListResponse], error) {
        users, total, err := state.UserService.List(ctx, query.Data)
        if err != nil {
            return espresso.JSON[models.ListResponse]{}, errors.Internal("failed to list users")
        }
        
        return espresso.JSON[models.ListResponse]{
            Data: models.ListResponse{
                Users: users,
                Total: total,
                Page:  query.Data.Page,
            },
        }, nil
    }
}
```

## Dockerfile

```dockerfile
# Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/configs ./configs

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

USER nobody

ENTRYPOINT ["./server"]
```

## Kubernetes Deployment

```yaml
# deployments/kubernetes/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: myapp
        image: myapp:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: myapp-secrets
              key: database-url
        - name: REDIS_ADDR
          value: "redis:6379"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: myapp
spec:
  selector:
    app: myapp
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

## Monitoring

```yaml
# Prometheus configuration
scrape_configs:
  - job_name: 'myapp'
    static_configs:
      - targets: ['myapp:80']
    metrics_path: /metrics
```

## Best Practices Summary

1. **Configuration**: Use environment variables for secrets
2. **Logging**: Structured logging with zerolog
3. **Middleware**: Request ID, Recovery, Logging, CORS, Compression, Rate Limit
4. **State**: Dependency injection via application state
5. **Error Handling**: Typed API errors with proper status codes
6. **Validation**: Request validation before processing
7. **Graceful Shutdown**: Handle SIGINT/SIGTERM
8. **Health Checks**: /health and /ready endpoints
9. **Docker**: Multi-stage build, minimal image
10. **Kubernetes**: Resource limits, probes, secrets