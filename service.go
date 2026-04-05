package espresso

import "context"

// Service is the core interface representing a request handler.
// It receives a request and returns a response, following the hexagonal architecture pattern.
// Services can be composed with Layers (middleware) to add cross-cutting concerns.
//
// The generic parameters allow type-safe request/response handling:
//   - Req: The request type (must implement FromRequest for automatic extraction)
//   - Res: The response type (should implement IntoResponse for automatic serialization)
//
// Example:
//
//	type UserService struct{}
//
//	func (s UserService) Call(ctx context.Context, req *CreateUserReq) (JSON[UserRes], error) {
//	    return JSON[UserRes]{Data: UserRes{Message: "created"}}, nil
//	}
type Service[Req any, Res any] interface {
	// Call executes the service logic with the given context and request.
	// It returns the response and any error that occurred during processing.
	Call(ctx context.Context, req Req) (Res, error)
}

// serviceFunc is an adapter type that converts a function into a Service.
// This allows using ordinary functions as Service implementations.
type serviceFunc[Req any, Res any] func(context.Context, Req) (Res, error)

// Call implements the Service interface by invoking the underlying function.
func (f serviceFunc[Req, Res]) Call(ctx context.Context, req Req) (Res, error) {
	return f(ctx, req)
}

// BuildService creates a new ServiceBuilder for composing services with layers.
// The builder pattern allows fluent chaining of middleware layers.
//
// Example:
//
//	svc := BuildService[*CreateUserReq, JSON[UserRes]]().
//	    Layer(LoggingLayer(logger, "userService")).
//	    Layer(TimeoutLayer(5 * time.Second)).
//	    Service(UserService{})
func BuildService[Req any, Res any]() *ServiceBuilder[Req, Res] {
	return &ServiceBuilder[Req, Res]{}
}

// ServiceBuilder provides a fluent API for building services with middleware layers.
// Layers are applied in reverse order (last added, first executed) to match
// the traditional middleware stack behavior.
type ServiceBuilder[Req any, Res any] struct {
	layers []Layer[Req, Res]
}

// Layer adds a middleware layer to the builder and returns the builder for chaining.
// Layers are executed in reverse order: the last added layer is executed first.
func (s *ServiceBuilder[Req, Res]) Layer(layer Layer[Req, Res]) *ServiceBuilder[Req, Res] {
	s.layers = append(s.layers, layer)
	return s
}

// Service wraps the given service with all added layers and returns the final service.
// Layers are applied from outermost (last added) to innermost (first added).
func (s *ServiceBuilder[Req, Res]) Service(svc Service[Req, Res]) Service[Req, Res] {
	for i := len(s.layers) - 1; i >= 0; i-- {
		svc = s.layers[i](svc)
	}
	return svc
}
