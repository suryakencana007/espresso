---
title: Service Middleware API Reference
description: Service layer types and functions
---

# Service Middleware API Reference

Package `middleware/service` provides service-level middleware (layers).

```go
import servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
```

## Core Types

### Service

Typed request/response service:

```go
type Service[Req any, Res any] interface {
    Call(ctx context.Context, req Req) (Res, error)
}
```

### Layer

Middleware that wraps a service:

```go
type Layer[Req any, Res any] func(Service[Req, Res]) Service[Req, Res]
```

## Built-in Layers

### TimeoutLayer

Enforce timeouts:

```go
func TimeoutLayer[Req any, Res any](timeout time.Duration) Layer[Req, Res]
```

### RetryLayer

Retry with backoff:

```go
type BackoffStrategy int

const (
    BackoffFixed       BackoffStrategy = iota
    BackoffExponential
    BackoffLinear
)

func RetryLayer[Req any, Res any](maxRetries int, initialBackoff time.Duration, strategy BackoffStrategy) Layer[Req, Res]
```

### CircuitBreakerLayer

Circuit breaker pattern:

```go
type CircuitState int32

const (
    StateClosed   CircuitState = 0
    StateOpen     CircuitState = 1
    StateHalfOpen CircuitState = 2
)

type CircuitBreakerConfig struct {
    ServiceName      string
    FailureThreshold int
    Timeout          time.Duration
    SuccessThreshold int
}

var DefaultCircuitBreakerConfig CircuitBreakerConfig

func CircuitBreakerLayer[Req any, Res any](config CircuitBreakerConfig) Layer[Req, Res]

func IsCircuitBreakerError(err error) bool
func NewCircuitBreakerError(serviceName string, state CircuitState, message string) *CircuitBreakerError
```

### ConcurrencyLimitLayer

Limit concurrent requests:

```go
func ConcurrencyLimitLayer[Req any, Res any](maxConcurrent int) Layer[Req, Res]
```

### MetricsLayer

Collect metrics:

```go
type MetricsCollector interface {
    RecordRequest(serviceName string, duration time.Duration, err error)
    RecordActiveRequests(serviceName string, delta int)
}

func MetricsLayer[Req any, Res any](collector MetricsCollector, serviceName string) Layer[Req, Res]
```

### LoggingLayer

Log service calls:

```go
func LoggingLayer[Req any, Res any](logger zerolog.Logger, serviceName string) Layer[Req, Res]
```

### ValidationLayer

Validate requests:

```go
type Validator[Req any] interface {
    Validate(ctx context.Context, req Req) error
}

func ValidationLayer[Req any, Res any](validator Validator[Req]) Layer[Req, Res]
```

## Error Types

### CircuitBreakerError

```go
type CircuitBreakerError struct {
    ServiceName string
    State       CircuitState
    Message     string
}
```

### ErrValidation

```go
type ErrValidation struct {
    Err error
}
```

## Example

```go
func main() {
    cbConfig := servicemiddleware.CircuitBreakerConfig{
        ServiceName:      "user-service",
        FailureThreshold: 5,
        Timeout:          30 * time.Second,
        SuccessThreshold: 3,
    }
    
    router := espresso.Portafilter()
    
    router.PostWith("/users", UserService{},
        servicemiddleware.TimeoutLayer[CreateUserReq, User](30*time.Second),
        servicemiddleware.RetryLayer[CreateUserReq, User](3, 100*time.Millisecond, 
            servicemiddleware.BackoffExponential),
        servicemiddleware.CircuitBreakerLayer[CreateUserReq, User](cbConfig),
    )
    
    router.Brew()
}
```

## See Also

- [Service Layers Guide](/guide/middleware/service) - Detailed usage
- [Middleware Overview](/guide/middleware/) - Architecture