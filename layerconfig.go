package espresso

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/rs/zerolog"
	servicemiddleware "github.com/suryakencana007/espresso/v2/middleware/service"
)

// LayerConfig holds layer configuration without type parameters.
// Types are resolved when the layer is applied to a specific handler.
// This allows layers to be defined once and reused across multiple handlers.
//
// Example:
//
//	commonLayers := espresso.Layers(
//	    espresso.Timeout(5*time.Second),
//	    espresso.Logging(logger, "api"),
//	)
//
//	app.Post("/users", espresso.WithLayers(createUser, commonLayers...))
//	app.Post("/posts", espresso.WithLayers(createPost, commonLayers...))
type LayerConfig interface {
	layerConfig()
}

// ============================================
// Timeout Layer Config
// ============================================

type timeoutConfig struct {
	duration time.Duration
}

func (c *timeoutConfig) layerConfig() {}

// Timeout creates a timeout layer configuration.
// Enforces a deadline on request processing.
//
// Example:
//
//	espresso.Timeout(5*time.Second)
func Timeout(duration time.Duration) LayerConfig {
	return &timeoutConfig{duration: duration}
}

// ============================================
// Logging Layer Config
// ============================================

type loggingConfig struct {
	logger      zerolog.Logger
	serviceName string
}

func (c *loggingConfig) layerConfig() {}

// Logging creates a logging layer configuration.
// Logs request processing time and errors.
//
// Example:
//
//	espresso.Logging(logger, "UserService")
func Logging(logger zerolog.Logger, serviceName string) LayerConfig {
	return &loggingConfig{logger: logger, serviceName: serviceName}
}

// ============================================
// Retry Layer Config
// ============================================

type retryConfig struct {
	maxRetries     int
	initialBackoff time.Duration
	strategy       servicemiddleware.BackoffStrategy
}

func (c *retryConfig) layerConfig() {}

// Retry creates a retry layer configuration with configurable backoff.
// Retries failed requests up to maxRetries times.
//
// Example:
//
//	espresso.Retry(3, 100*time.Millisecond, servicemiddleware.BackoffExponential)
func Retry(maxRetries int, initialBackoff time.Duration, strategy servicemiddleware.BackoffStrategy) LayerConfig {
	return &retryConfig{
		maxRetries:     maxRetries,
		initialBackoff: initialBackoff,
		strategy:       strategy,
	}
}

// ============================================
// Circuit Breaker Layer Config
// ============================================

type circuitBreakerConfig struct {
	config servicemiddleware.CircuitBreakerConfig
}

func (c *circuitBreakerConfig) layerConfig() {}

// CircuitBreaker creates a circuit breaker layer configuration.
// Prevents cascade failures by opening after failure threshold.
//
// Example:
//
//	espresso.CircuitBreaker(servicemiddleware.DefaultCircuitBreakerConfig)
func CircuitBreaker(config servicemiddleware.CircuitBreakerConfig) LayerConfig {
	return &circuitBreakerConfig{config: config}
}

// ============================================
// Concurrency Limit Layer Config
// ============================================

type concurrencyLimitConfig struct {
	maxConcurrent int
}

func (c *concurrencyLimitConfig) layerConfig() {}

// ConcurrencyLimit creates a concurrency limit layer configuration.
// Limits the number of concurrent requests.
//
// Example:
//
//	espresso.ConcurrencyLimit(100)
func ConcurrencyLimit(maxConcurrent int) LayerConfig {
	return &concurrencyLimitConfig{maxConcurrent: maxConcurrent}
}

// ============================================
// Metrics Layer Config
// ============================================

type metricsConfig struct {
	collector   servicemiddleware.MetricsCollector
	serviceName string
}

func (c *metricsConfig) layerConfig() {}

// Metrics creates a metrics collection layer configuration.
// Records request duration, errors, and active requests.
//
// Example:
//
//	espresso.Metrics(collector, "UserService")
func Metrics(collector servicemiddleware.MetricsCollector, serviceName string) LayerConfig {
	return &metricsConfig{collector: collector, serviceName: serviceName}
}

// ============================================
// Validation Layer Config
// ============================================

// validationConfigLike is a non-generic marker interface implemented by every
// validationConfig[Req]. It exists so buildLayer can detect a mismatched
// validation config (one whose Req differs from the handler's Req) in the
// default case and panic with a descriptive message instead of silently
// skipping validation.
type validationConfigLike interface {
	LayerConfig
	validatorReqType() reflect.Type
}

type validationConfig[Req any] struct {
	validator servicemiddleware.Validator[Req]
}

func (c *validationConfig[Req]) layerConfig() {}

// validatorReqType returns the validator's Req type for diagnostic messages.
func (c *validationConfig[Req]) validatorReqType() reflect.Type {
	return reflect.TypeFor[Req]()
}

// Validation creates a validation layer configuration. Validates requests
// before the handler runs; on validation failure the pipeline returns the
// validator's error without invoking the handler.
//
// Since v2.0 the function is generic: the type parameter is the request
// type the validator accepts. Go infers it from the validator argument in
// most call sites, so existing code rarely needs a syntactic change.
//
// Example:
//
//	validator := servicemiddleware.ValidatorFunc[*JSON[CreateUserReq]](
//	    func(ctx context.Context, req *JSON[CreateUserReq]) error {
//	        return validator.Struct(req.Data)
//	    },
//	)
//	app.Post("/users", WithLayers(createUser, Layers(
//	    Validation(validator),  // Req inferred as *JSON[CreateUserReq]
//	)...))
//
// Mismatch detection: if the validator's Req does not match the handler's
// request type at WithLayers binding time, registration panics with a
// descriptive message naming both types — better than silently skipping
// validation, which is what the v1.x untyped form did when the type
// assertion in buildLayer failed.
func Validation[Req any](validator servicemiddleware.Validator[Req]) LayerConfig {
	return &validationConfig[Req]{validator: validator}
}

// ============================================
// Custom Layer Config
// ============================================

type customConfig struct {
	build func() any
}

func (c *customConfig) layerConfig() {}

// CustomLayer creates a custom layer from a builder function.
// Allows user-defined layers with full control.
//
// Example:
//
//	espresso.CustomLayer(func() any {
//	    return MyCustomLayer[Req, Res]{...}
//	})
func CustomLayer(buildFunc func() any) LayerConfig {
	return &customConfig{build: buildFunc}
}

// ============================================
// Internal: Build typed layers from configs
// ============================================

func buildLayer[Req any, Res any](cfg LayerConfig) Layer[Req, Res] {
	switch c := cfg.(type) {
	case *timeoutConfig:
		return adaptServiceLayer(servicemiddleware.TimeoutLayer[Req, Res](c.duration))
	case *loggingConfig:
		return adaptServiceLayer(servicemiddleware.LoggingLayer[Req, Res](c.logger, c.serviceName))
	case *retryConfig:
		return adaptServiceLayer(servicemiddleware.RetryLayer[Req, Res](c.maxRetries, c.initialBackoff, c.strategy))
	case *circuitBreakerConfig:
		return adaptServiceLayer(servicemiddleware.CircuitBreakerLayer[Req, Res](c.config))
	case *concurrencyLimitConfig:
		return adaptServiceLayer(servicemiddleware.ConcurrencyLimitLayer[Req, Res](c.maxConcurrent))
	case *metricsConfig:
		return adaptServiceLayer(servicemiddleware.MetricsLayer[Req, Res](c.collector, c.serviceName))
	case *validationConfig[Req]:
		return adaptServiceLayer(servicemiddleware.ValidationLayer[Req, Res](c.validator))
	case *customConfig:
		if layer, ok := c.build().(Layer[Req, Res]); ok {
			return layer
		}
		panic("espresso: custom layer builder did not return Layer[Req, Res]")
	default:
		// Mismatched typed validation config: a *validationConfig[X] applied
		// to a pipeline whose Req is not X. The case-match above failed
		// because Go generics types are nominal; surface the mismatch so the
		// user sees a registration-time panic instead of silently skipping
		// validation.
		if vc, ok := cfg.(validationConfigLike); ok {
			panic(fmt.Sprintf(
				"espresso: validation layer's request type (%v) does not match handler's request type (%v); "+
					"either pass a Validator[%v] or remove the validation layer",
				vc.validatorReqType(), reflect.TypeFor[Req](), reflect.TypeFor[Req](),
			))
		}
		panic("espresso: unknown layer config type")
	}
}

type rootToServiceAdapter[Req any, Res any] struct {
	next Service[Req, Res]
}

func (a rootToServiceAdapter[Req, Res]) Call(ctx context.Context, req Req) (Res, error) {
	return a.next.Call(ctx, req)
}

type serviceToRootAdapter[Req any, Res any] struct {
	next servicemiddleware.Service[Req, Res]
}

func (a serviceToRootAdapter[Req, Res]) Call(ctx context.Context, req Req) (Res, error) {
	return a.next.Call(ctx, req)
}

func adaptServiceLayer[Req any, Res any](layer servicemiddleware.Layer[Req, Res]) Layer[Req, Res] {
	return func(next Service[Req, Res]) Service[Req, Res] {
		wrapped := layer(rootToServiceAdapter[Req, Res]{next: next})
		return serviceToRootAdapter[Req, Res]{next: wrapped}
	}
}
