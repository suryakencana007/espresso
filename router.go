package espresso

import "net/http"

// Router wraps http.ServeMux with fluent API for route registration.
// Implements http.Handler for use with http.ListenAndServe or Brew.
//
// Middleware is applied in two levels:
//   - HTTP Middleware (Use): Runs before extraction, operates on raw HTTP
//   - Service Layers (PostWith, GetWith, etc.): Runs after extraction, operates on typed Req/Res
//
// Example (traditional style):
//
//	router := Portafilter()
//	router.Use(httpmiddleware.RequestIDMiddleware(), httpmiddleware.RecoverMiddleware())
//	router.Get("/health", func() string { return "ok" })
//	router.Post("/users", UserService{})
//	router.Brew()
//
// Example (chain pattern):
//
//	Portafilter().
//		Use(httpmiddleware.RequestIDMiddleware(), httpmiddleware.RecoverMiddleware()).
//		Get("/health", func() string { return "ok" }).
//		Post("/users", CreateUser).
//		Put("/users/{id}", UpdateUser).
//		Delete("/users/{id}", DeleteUser).
//		Brew(espresso.WithAddr(":3000"))
type Router struct {
	mux        *http.ServeMux
	middleware []func(http.Handler) http.Handler
	state      any
}

// Portafilter creates a new Router with an initialized ServeMux.
// Named after the portafilter in espresso machines - the component that holds
// the filter basket and directs the brewing process.
//
// Example:
//
//	router := espresso.Portafilter()
//	router.Get("/health", Ristretto(healthCheck))
//	router.Brew()
func Portafilter() *Router {
	return &Router{mux: http.NewServeMux()}
}

// Use adds HTTP-level middleware that runs before request extraction.
// Middleware is applied to all routes registered after Use() is called.
// Middleware is applied in the order it's added (first = outermost).
//
// Example:
//
//	router.Use(httpmiddleware.RequestIDMiddleware())
//	router.Use(httpmiddleware.RecoverMiddleware())
//	router.Use(httpmiddleware.CORSMiddleware(httpmiddleware.DefaultCORSConfig))
func (r *Router) Use(mw ...func(http.Handler) http.Handler) *Router {
	r.middleware = append(r.middleware, mw...)
	return r
}

// WithState adds application state to the router context.
// State is immutable and available to all handlers via GetState[T] or State[T] extractor.
// This is the recommended way to provide application-wide dependencies (DB, config, etc.).
//
// Example:
//
//	appState := AppState{DB: db, Config: config}
//	router := espresso.Portafilter().
//	    WithState(appState).
//	    Get("/users", espresso.Doppio(getUsers))
//
//	// In handler:
//	func getUsers(ctx context.Context, req *espresso.JSON[Req]) (Res, error) {
//	    state := espresso.MustGetState[AppState](ctx)
//	    users := state.DB.FindAllUsers()
//	    return Res{Data: users}, nil
//	}
func (r *Router) WithState(state any) *Router {
	r.state = state
	r.middleware = append([]func(http.Handler) http.Handler{WithStateMiddleware(state)}, r.middleware...)
	return r
}

// Get registers a handler for GET requests.
// Returns *Router for method chaining.
func (r *Router) Get(path string, f any) *Router {
	handler := r.applyMiddleware(r.Handle(f))
	r.mux.HandleFunc("GET "+path, handler)
	return r
}

// Post registers a handler for POST requests.
// Returns *Router for method chaining.
func (r *Router) Post(path string, f any) *Router {
	handler := r.applyMiddleware(r.Handle(f))
	r.mux.HandleFunc("POST "+path, handler)
	return r
}

// Put registers a handler for PUT requests.
// Returns *Router for method chaining.
func (r *Router) Put(path string, f any) *Router {
	handler := r.applyMiddleware(r.Handle(f))
	r.mux.HandleFunc("PUT "+path, handler)
	return r
}

// Delete registers a handler for DELETE requests.
// Returns *Router for method chaining.
func (r *Router) Delete(path string, f any) *Router {
	handler := r.applyMiddleware(r.Handle(f))
	r.mux.HandleFunc("DELETE "+path, handler)
	return r
}

// Patch registers a handler for PATCH requests.
// Returns *Router for method chaining.
func (r *Router) Patch(path string, f any) *Router {
	handler := r.applyMiddleware(r.Handle(f))
	r.mux.HandleFunc("PATCH "+path, handler)
	return r
}

// Options registers a handler for OPTIONS requests.
// Returns *Router for method chaining.
func (r *Router) Options(path string, f any) *Router {
	handler := r.applyMiddleware(r.Handle(f))
	r.mux.HandleFunc("OPTIONS "+path, handler)
	return r
}

// Head registers a handler for HEAD requests.
// Returns *Router for method chaining.
func (r *Router) Head(path string, f any) *Router {
	handler := r.applyMiddleware(r.Handle(f))
	r.mux.HandleFunc("HEAD "+path, handler)
	return r
}

// applyMiddleware wraps the handler with all registered middleware.
func (r *Router) applyMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	result := http.Handler(handler)
	for i := len(r.middleware) - 1; i >= 0; i-- {
		result = r.middleware[i](result)
	}
	return result.ServeHTTP
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// Handle converts handler types to http.HandlerFunc.
// Supported types:
//   - func() T - returns T (auto-wrapped as Text or JSON)
//   - func(*Req) T - extracts Req via FromRequest interface
//   - func(context.Context, *Req) (T, error) - full control
//   - Service[T, R] interface - calls Service.Call method
//
// Path patterns support Go 1.22+ ServeMux syntax (static, wildcards).
// Handler analysis occurs at registration time for performance.
func (r *Router) Handle(f any) http.HandlerFunc {
	return Handler(f)
}

// Routes returns all registered routes.
// Note: This is a best-effort implementation as ServeMux doesn't expose routes.
func (r *Router) Routes() []Route {
	return nil // ServeMux doesn't expose routes
}

// Route represents a registered route.
type Route struct {
	Method  string
	Path    string
	Handler any
}
