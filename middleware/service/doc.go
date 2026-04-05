// Package servicemiddleware provides service-level middleware (layers) for the espresso framework.
//
// Service layers operate on typed request/response objects after extraction.
// This includes logging, timeout, retry, circuit breaker, metrics, validation, etc.
//
// Example:
//
//	import (
//	    "github.com/suryakencana007/espresso"
//	    "github.com/suryakencana007/espresso/middleware/service"
//	)
//
//	layer := servicemiddleware.TimeoutLayer[CreateUserReq, UserRes](5 * time.Second)
//	service := BuildService[CreateUserReq, UserRes]().
//	    Layer(layer).
//	    Service(UserService{})
package servicemiddleware
