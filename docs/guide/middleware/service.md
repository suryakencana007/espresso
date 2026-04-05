# Service Layers

Service layers operate on typed `Req/Res` pairs after extraction. They provide fine-grained control over business logic execution.

## Overview

Service layers are middleware that operates at the service level, after request extraction:

```go
type Service[Req any, Res any] interface {
    Call(ctx context.Context, req Req) (Res, error)
}

type Layer[Req any, Res any] func(Service[Req, Res]) Service[Req, Res]
```

## Built-in Layers

### Timeout Layer

Enforce timeouts on service calls:

```go
import servicemiddleware "github.com/suryakencana007/espresso/middleware/service"

func main() {
    router := espresso.Portafilter()
    
    router.PostWith("/users", UserService{},
        servicemiddleware.TimeoutLayer[CreateUserReq, User](5*time.Second),
    )
    
    router.Brew()
}
```

### Retry Layer

Retry failed operations with backoff:

```go
router.PostWith("/api", ExternalService{},
    servicemiddleware.RetryLayer[Req, Res](
        3,                        // max retries
        100*time.Millisecond,     // initial backoff
        servicemiddleware.BackoffExponential, // strategy
    ),
)
```

Backoff strategies:

| Strategy | Behavior |
|----------|----------|
| `BackoffFixed` | Constant delay (100ms, 100ms, 100ms) |
| `BackoffLinear` | Linear increase (100ms, 200ms, 300ms) |
| `BackoffExponential` | Double each time (100ms, 200ms, 400ms) |

### Circuit Breaker Layer

Prevent cascading failures:

```go
cbConfig := servicemiddleware.CircuitBreakerConfig{
    ServiceName:      "user-service",
    FailureThreshold: 5,              // Open after 5 failures
    Timeout:          30*time.Second, // Wait before half-open
    SuccessThreshold: 3,              // Close after 3 successes
}

router.PostWith("/users", UserService{},
    servicemiddleware.CircuitBreakerLayer[CreateUserReq, User](cbConfig),
)
```

States:
- **Closed**: Normal operation, all requests pass through
- **Open**: Rejects all requests, returns `CircuitBreakerError`
- **Half-Open**: Allows limited requests to test recovery

```go
// Handle circuit breaker errors
func handler(ctx context.Context, req *espresso.JSON[Req]) (Response, error) {
    res, err := service.Call(ctx, req.Data)
    if servicemiddleware.IsCircuitBreakerError(err) {
        return espresso.JSON[ErrorRes]{
            StatusCode: http.StatusServiceUnavailable,
            Data: ErrorRes{Message: "Service temporarily unavailable"},
        }, nil
    }
    // ...
}
```

### Concurrency Limit Layer

Limit concurrent requests:

```go
router.PostWith("/expensive", ExpensiveService{},
    servicemiddleware.ConcurrencyLimitLayer[Req, Res](100), // Max 100 concurrent
)
```

Requests exceeding the limit wait in a queue.

### Validation Layer

Validate requests before processing:

```go
type UserValidator struct{}

func (v UserValidator) Validate(ctx context.Context, req CreateUserReq) error {
    if req.Name == "" {
        return errors.New("name is required")
    }
    if !isValidEmail(req.Email) {
        return errors.New("invalid email")
    }
    return nil
}

router.PostWith("/users", UserService{},
    servicemiddleware.ValidationLayer[CreateUserReq, User](UserValidator{}),
)
```

### Logging Layer

Log service execution:

```go
router.PostWith("/users", UserService{},
    servicemiddleware.LoggingLayer[CreateUserReq, User](log.Logger, "UserService"),
)

// Output:
// INFO service=UserService latency=15.234ms "Request processed"
```

### Metrics Layer

Collect metrics:

```go
type PrometheusCollector struct{}

func (c PrometheusCollector) RecordRequest(service string, duration time.Duration, err error) {
    // Record in Prometheus
    requestsTotal.WithLabelValues(service).Inc()
    requestDuration.WithLabelValues(service).Observe(duration.Seconds())
}

func (c PrometheusCollector) RecordActiveRequests(service string, delta int) {
    activeRequests.Add(float64(delta))
}

router.PostWith("/users", UserService{},
    servicemiddleware.MetricsLayer[CreateUserReq, User](collector, "UserService"),
)
```

## Combining Layers

Multiple layers compose naturally:

```go
func main() {
    cbConfig := servicemiddleware.DefaultCircuitBreakerConfig
    cbConfig.ServiceName = "user-service"
    
    router := espresso.Portafilter()
    
    router.PostWith("/users", UserService{},
        // Order: Timeout -> Retry -> CircuitBreaker -> Validation -> Service
        servicemiddleware.TimeoutLayer[CreateUserReq, User](30*time.Second),
        servicemiddleware.RetryLayer[CreateUserReq, User](3, 100*time.Millisecond, 
            servicemiddleware.BackoffExponential),
        servicemiddleware.CircuitBreakerLayer[CreateUserReq, User](cbConfig),
        servicemiddleware.ValidationLayer[CreateUserReq, User](validator),
    )
    
    router.Brew()
}
```

**Note**: Layers are applied in order (first = outermost).

## Custom Layers

Create custom service layers:

```go
func AuditLayer[Req any, Res any](auditLogger AuditLogger, action string) servicemiddleware.Layer[Req, Res] {
    return func(next servicemiddleware.Service[Req, Res]) servicemiddleware.Service[Req, Res] {
        return servicemiddleware.ServiceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
            // Call the underlying service
            res, err := next.Call(ctx, req)
            
            // Log audit trail (whether success or failure)
            auditLogger.Log(action, req, res, err)
            
            return res, err
        })
    }
}
```

### Request/Response Transformation

Transform requests or responses:

```go
func SanitizeInputLayer[Req any, Res any]() servicemiddleware.Layer[Req, Res] {
    return func(next servicemiddleware.Service[Req, Res]) servicemiddleware.Service[Req, Res] {
        return servicemiddleware.ServiceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
            // Sanitize input (if Req has string fields)
            sanitize(&req)
            
            return next.Call(ctx, req)
        })
    }
}
```

### Caching Layer

Cache responses:

```go
func CacheLayer[Req any, Res any](cache Cache, ttl time.Duration) servicemiddleware.Layer[Req, Res] {
    return func(next servicemiddleware.Service[Req, Res]) servicemiddleware.Service[Req, Res] {
        return servicemiddleware.ServiceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
            key := cacheKey(req)
            
            // Check cache
            if cached, ok := cache.Get(key); ok {
                return cached.(Res), nil
            }
            
            // Call service
            res, err := next.Call(ctx, req)
            if err != nil {
                var zero Res
                return zero, err
            }
            
            // Cache result
            cache.Set(key, res, ttl)
            
            return res, nil
        })
    }
}
```

## Reusing Layer Stacks

Define reusable layer stacks:

```go
// Common layers for external API calls
func ExternalServiceLayers[Req, Res any](serviceName string) []servicemiddleware.Layer[Req, Res] {
    return []servicemiddleware.Layer[Req, Res]{
        servicemiddleware.TimeoutLayer[Req, Res](10*time.Second),
        servicemiddleware.RetryLayer[Req, Res](3, 100*time.Millisecond, 
            servicemiddleware.BackoffExponential),
        servicemiddleware.CircuitBreakerLayer[Req, Res](
            servicemiddleware.CircuitBreakerConfig{
                ServiceName:      serviceName,
                FailureThreshold: 5,
                Timeout:          30*time.Second,
                SuccessThreshold: 3,
            }),
        servicemiddleware.LoggingLayer[Req, Res](log.Logger, serviceName),
    }
}

// usage
router.PostWith("/api/external", ExternalService{},
    ExternalServiceLayers[Req, Res]("external-api")...,
)
```

## Service Interface

Implement the Service interface:

```go
type UserService struct {
    db *sql.DB
}

func (s UserService) Call(ctx context.Context, req CreateUserReq) (User, error) {
    // Business logic here
    user := User{
        ID:    generateID(),
        Name:  req.Name,
        Email: req.Email,
    }
    
    if err := s.db.CreateUser(ctx, user); err != nil {
        return User{}, err
    }
    
    return user, nil
}
```

Or use a function:

```go
type ServiceFunc[Req any, Res any] func(ctx context.Context, req Req) (Res, error)

func (f ServiceFunc[Req, Res]) Call(ctx context.Context, req Req) (Res, error) {
    return f(ctx, req)
}
```

## Best Practices

1. **Layer order matters**: Place validation before retry
2. **Use appropriate timeouts**: Service timeout < HTTP timeout
3. **Configure circuit breakers**: Tune thresholds to your service
4. **Don't over-layer**: Only add layers you need
5. **Measure performance**: Layers add overhead
6. **Handle errors gracefully**: Return typed errors