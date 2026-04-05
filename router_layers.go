package espresso

// Layered wraps a Service with layers and returns a handler for Router.Post/Put/Delete.
// This enables type-safe layer composition with the existing Router API.
//
// Example:
//
//	app.Post("/users", espresso.Layered(
//	    UserService{},
//	    LoggingLayer[*CreateUserReq, JSON[UserRes]](logger, "users"),
//	    TimeoutLayer[*CreateUserReq, JSON[UserRes]](5*time.Second)))
func Layered[Req, Res any](svc Service[Req, Res], layers ...Layer[Req, Res]) any {
	// Apply layers in reverse order (last added = outermost)
	wrapped := svc
	for i := len(layers) - 1; i >= 0; i-- {
		wrapped = layers[i](wrapped)
	}
	return wrapped
}
