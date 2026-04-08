package espresso

import (
	"net/http"
	"reflect"

	"github.com/suryakencana007/espresso/openapi"
)

// OpenAPIOptions configures OpenAPI generation for a route.
type OpenAPIOptions struct {
	Summary     string
	Description string
	Tags        []string
	Deprecated  bool
}

// OpenAPIRouter wraps Router with OpenAPI generation support.
// Allows automatic OpenAPI spec generation from route handlers.
//
// Example:
//
//	gen := openapi.New("My API", "1.0.0").
//	    Description("REST API").
//	    Server("http://localhost:8080", "Development")
//
//	router := OpenAPIRouter(gen)
//	router.Get("/users", getUsers, openapi.Tags("users"))
//	router.Post("/users", createUser, openapi.Summary("Create user"))
//
//	http.Handle("/openapi.json", gen.Handler())
//	http.Handle("/docs", openapi.ScalarUI(gen.Handler()))
type OpenAPIRouter struct {
	router *Router
	gen    *openapi.Generator
}

// OpenAPI creates an OpenAPI-enabled router.
func OpenAPI(gen *openapi.Generator) *OpenAPIRouter {
	return &OpenAPIRouter{
		router: Portafilter(),
		gen:    gen,
	}
}

// Use adds HTTP-level middleware.
func (r *OpenAPIRouter) Use(mw ...func(http.Handler) http.Handler) *OpenAPIRouter {
	r.router.Use(mw...)
	return r
}

// WithState adds application state.
func (r *OpenAPIRouter) WithState(state any) *OpenAPIRouter {
	r.router.WithState(state)
	return r
}

// Get registers a GET handler with OpenAPI documentation.
func (r *OpenAPIRouter) Get(path string, handler any, opts ...openapi.OperationOption) *OpenAPIRouter {
	r.registerPath(http.MethodGet, path, handler, opts...)
	r.router.Get(path, handler)
	return r
}

// Post registers a POST handler with OpenAPI documentation.
func (r *OpenAPIRouter) Post(path string, handler any, opts ...openapi.OperationOption) *OpenAPIRouter {
	r.registerPath(http.MethodPost, path, handler, opts...)
	r.router.Post(path, handler)
	return r
}

// Put registers a PUT handler with OpenAPI documentation.
func (r *OpenAPIRouter) Put(path string, handler any, opts ...openapi.OperationOption) *OpenAPIRouter {
	r.registerPath(http.MethodPut, path, handler, opts...)
	r.router.Put(path, handler)
	return r
}

// Delete registers a DELETE handler with OpenAPI documentation.
func (r *OpenAPIRouter) Delete(path string, handler any, opts ...openapi.OperationOption) *OpenAPIRouter {
	r.registerPath(http.MethodDelete, path, handler, opts...)
	r.router.Delete(path, handler)
	return r
}

// Patch registers a PATCH handler with OpenAPI documentation.
func (r *OpenAPIRouter) Patch(path string, handler any, opts ...openapi.OperationOption) *OpenAPIRouter {
	r.registerPath(http.MethodPatch, path, handler, opts...)
	r.router.Patch(path, handler)
	return r
}

// Options registers an OPTIONS handler with OpenAPI documentation.
func (r *OpenAPIRouter) Options(path string, handler any, opts ...openapi.OperationOption) *OpenAPIRouter {
	r.registerPath(http.MethodOptions, path, handler, opts...)
	r.router.Options(path, handler)
	return r
}

// Head registers a HEAD handler with OpenAPI documentation.
func (r *OpenAPIRouter) Head(path string, handler any, opts ...openapi.OperationOption) *OpenAPIRouter {
	r.registerPath(http.MethodHead, path, handler, opts...)
	r.router.Head(path, handler)
	return r
}

// registerPath introspects the handler and generates OpenAPI documentation.
func (r *OpenAPIRouter) registerPath(method, path string, handler any, opts ...openapi.OperationOption) {
	info, err := openapi.Introspect(handler)
	if err != nil {
		return
	}

	op := openapi.BuildOperation(info, opts...)

	if len(op.Tags) == 0 {
		op.Tags = []string{"default"}
	}

	if len(op.Responses) == 0 {
		op.Responses = map[string]openapi.Response{
			"200": {
				Description: "Success",
			},
		}
	}

	for i, reqType := range info.RequestTypes {
		if i >= len(info.ExtractorKinds) {
			continue
		}

		kind := info.ExtractorKinds[i]
		switch kind {
		case openapi.KindPath:
			params := openapi.GeneratePathParams(reqType)
			op.Parameters = append(op.Parameters, params...)
		case openapi.KindQuery:
			params := openapi.GenerateQueryParams(reqType)
			op.Parameters = append(op.Parameters, params...)
		case openapi.KindJSONBody:
			if op.RequestBody == nil {
				op.RequestBody = openapi.GenerateRequestBody(reqType, r.gen)
			}
		}
	}

	if info.ResponseType != nil {
		schema := openapi.GenerateSchemaFromType(info.ResponseType)
		schemaName := info.ResponseType.Name()
		if schemaName != "" {
			r.gen.Schema(schemaName, info.ResponseType)
		}

		if _, ok := op.Responses["200"]; ok {
			op.Responses["200"] = openapi.Response{
				Description: "Success",
				Content: map[string]openapi.MediaType{
					"application/json": {
						Schema: schema,
					},
				},
			}
		}
	}

	r.gen.AddPath(method, path, *op)
}

// Brew starts the server.
func (r *OpenAPIRouter) Brew(opts ...ServerOption) {
	r.router.Brew(opts...)
}

// Router returns the underlying Router.
func (r *OpenAPIRouter) Router() *Router {
	return r.router
}

// Generator returns the OpenAPI generator.
func (r *OpenAPIRouter) Generator() *openapi.Generator {
	return r.gen
}

// ServeHTTP implements http.Handler.
func (r *OpenAPIRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

// AutoRegister registers all routes from a Router to OpenAPI.
// Useful when you want to separate route definition from OpenAPI generation.
//
// Example:
//
//	router := espresso.Portafilter().
//	    Get("/users", getUsers).
//	    Post("/users", createUser)
//
//	gen := openapi.New("My API", "1.0.0")
//	espresso.AutoRegister(gen, router, map[string][]openapi.OperationOption{
//	    "GET /users":    {openapi.Tags("users")},
//	    "POST /users":   {openapi.Tags("users"), openapi.Summary("Create user")},
//	})
func AutoRegister(gen *openapi.Generator, router *Router, optsMap map[string][]openapi.OperationOption) {
}

// RegisterHandler registers a single handler to OpenAPI with explicit options.
// Use this when automatic introspection is not enough.
//
// Example:
//
//	gen := openapi.New("My API", "1.0.0")
//	err := espresso.RegisterHandler(gen, "GET", "/users/{id}", getUserHandler,
//	    openapi.Tags("users"),
//	    openapi.Summary("Get user by ID"),
//	    openapi.Status("200", openapi.Response{
//	        Description: "User found",
//	    }),
//	)
func RegisterHandler(gen *openapi.Generator, method, path string, handler any, opts ...openapi.OperationOption) error {
	info, err := openapi.Introspect(handler)
	if err != nil {
		return err
	}

	op := openapi.BuildOperation(info, opts...)

	if len(op.Tags) == 0 {
		op.Tags = []string{"default"}
	}

	if len(op.Responses) == 0 {
		op.Responses = map[string]openapi.Response{
			"200": {
				Description: "Success",
			},
		}
	}

	for i, reqType := range info.RequestTypes {
		if i >= len(info.ExtractorKinds) {
			continue
		}

		kind := info.ExtractorKinds[i]
		switch kind {
		case openapi.KindPath:
			params := openapi.GeneratePathParams(reqType)
			op.Parameters = append(op.Parameters, params...)
		case openapi.KindQuery:
			params := openapi.GenerateQueryParams(reqType)
			op.Parameters = append(op.Parameters, params...)
		case openapi.KindJSONBody:
			if op.RequestBody == nil {
				op.RequestBody = openapi.GenerateRequestBody(reqType, gen)
			}
		}
	}

	gen.AddPath(method, path, *op)
	return nil
}

// InferTypeFromStruct generates an OpenAPI schema from a struct type.
// Useful for manually documenting request/response types.
//
// Example:
//
//	type User struct {
//	    ID    int    `json:"id" doc:"User ID"`
//	    Name  string `json:"name" doc:"User name"`
//	    Email string `json:"email,omitempty" doc:"User email"`
//	}
//
//	gen := openapi.New("My API", "1.0.0")
//	espresso.InferTypeFromStruct(gen, "User", User{})
func InferTypeFromStruct(gen *openapi.Generator, name string, v any) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	gen.Schema(name, t)
}
