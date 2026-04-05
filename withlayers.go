package espresso

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"
)

// WithLayers applies layers to any handler function with type inference.
// Supports all handler styles: Doppio, Solo, Ristretto, Service, http.HandlerFunc.
//
// Types are inferred from the handler function signature:
//   - func(context.Context, *Req) (Res, error)  // Doppio
//   - func(*Req) (Res, error)                    // Solo
//   - func() Res                                 // Ristretto
//   - Service[Req, Res]                          // Service interface
//   - http.HandlerFunc                           // Raw HTTP handler (no layers applied)
//
// Example:
//
//	commonLayers := espresso.Layers(
//	    espresso.Timeout(5*time.Second),
//	    espresso.Logging(logger, "api"),
//	)
//
//	app.Post("/users", espresso.WithLayers(createUser, commonLayers...))
//	app.Get("/users/{id}", espresso.WithLayers(getUser, commonLayers...))
//
// For edge cases where type inference fails, use WithLayersTyped.
func WithLayers(handler any, layers ...LayerConfig) http.HandlerFunc {
	// Try to infer types and create typed version
	typedHandler, err := inferAndWrap(handler, layers...)
	if err != nil {
		panic(fmt.Sprintf(`
espresso: WithLayers cannot infer Req/Res types for handler (signature: %T).

This can happen when:
  - Handler returns any (ambiguous type)
  - Handler is an http.HandlerFunc (raw HTTP, use Use() for middleware)
  - Handler signature is invalid

Use WithLayersTyped[Req, Res]() for explicit types:
  app.Post("/path", espresso.WithLayersTyped[*MyReq, MyRes](handler, layers...))

Supported handler patterns:
  - func(context.Context, *Req) (Res, error)    // Doppio (recommended)
  - func(context.Context, *Req) Res            // HandlerCtxReq
  - func(*Req) (Res, error)                      // Solo
  - func(*Req) Res                               // HandlerReq
  - func(context.Context) (Res, error)          // HandlerCtx
  - func(context.Context) Res                   // HandlerCtxNoErr
  - func() (Res, error)                          // HandlerNoReq
  - func() Res                                   // Ristretto
  - Service[Req, Res]                            // Service interface

Error: %v
`, handler, err))
	}
	return typedHandler
}

// WithLayersTyped applies layers to a handler with explicit type parameters.
// Use this when WithLayers cannot infer types correctly.
//
// Example:
//
//	app.Post("/users", espresso.WithLayersTyped[*CreateUserReq, JSON[UserRes]](
//	    createUser,
//	    espresso.Timeout(5*time.Second),
//	    espresso.Logging(logger, "users"),
//	))
func WithLayersTyped[Req FromRequest, Res IntoResponse](handler any, layers ...LayerConfig) http.HandlerFunc {
	svc := handlerToService[Req, Res](handler)
	return applyLayersAndConvert(svc, layers)
}

// inferAndWrap attempts to infer Req/Res types and creates a typed handler.
func inferAndWrap(handler any, layers ...LayerConfig) (http.HandlerFunc, error) {
	// Use reflection to infer types
	reqType, resType, err := inferTypes(handler)
	if err != nil {
		return nil, err
	}

	// Convert to typed handler using reflection
	return createTypedHandler(handler, reqType, resType, layers)
}

// inferTypes extracts Req and Res types from handler signature.
func inferTypes(handler any) (reflect.Type, reflect.Type, error) {
	v := reflect.ValueOf(handler)
	t := v.Type()

	// Check if it's already http.HandlerFunc - can't apply service layers
	if t == reflect.TypeOf(http.HandlerFunc(nil)) {
		return nil, nil, fmt.Errorf("http.HandlerFunc cannot have service layers applied; use router.Use() for HTTP middleware")
	}

	// Check if it's a Service interface
	if _, hasCall := t.MethodByName("Call"); hasCall {
		// Try to extract Req/Res from Service interface
		return inferFromService(t)
	}

	// Must be a function
	if t.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("handler must be a function or Service interface, got %T", handler)
	}

	return inferFromFunction(t)
}

// inferFromService extracts Req/Res types from Service interface.
func inferFromService(t reflect.Type) (reflect.Type, reflect.Type, error) {
	// Service interface has Call(ctx, Req) (Res, error) method
	callMethod, ok := t.MethodByName("Call")
	if !ok {
		return nil, nil, fmt.Errorf("Service interface missing Call method")
	}

	callType := callMethod.Type

	// Call(ctx, Req) (Res, error)
	// Input[0] = context.Context, Input[1] = Req
	// Output[0] = Res, Output[1] = error
	if callType.NumIn() < 2 {
		return nil, nil, fmt.Errorf("Service.Call must have at least 2 parameters (ctx, Req)")
	}

	reqType := callType.In(1)

	if callType.NumOut() < 1 {
		return nil, nil, fmt.Errorf("Service.Call must return at least Res")
	}

	resType := callType.Out(0)

	return reqType, resType, nil
}

// inferFromFunction extracts Req/Res types from function signature.
func inferFromFunction(t reflect.Type) (reflect.Type, reflect.Type, error) {
	numIn := t.NumIn()
	numOut := t.NumOut()

	// Validate return types
	if numOut == 0 || numOut > 2 {
		return nil, nil, fmt.Errorf("handler must return 1 or 2 values (Res) or (Res, error)")
	}

	if numOut == 2 && !t.Out(1).Implements(errorType) {
		return nil, nil, fmt.Errorf("second return value must be error")
	}

	resType := t.Out(0)

	// Determine Req type based on input parameters
	var reqType reflect.Type

	switch numIn {
	case 0:
		// Ristretto or HandlerNoReq: func() Res or func() (Res, error)
		// No Req type - use empty struct
		reqType = reflect.TypeOf(struct{}{})

	case 1:
		// Could be:
		// - Solo: func(*Req) (Res, error)
		// - HandlerReq: func(*Req) Res
		// - HandlerCtx: func(context.Context) (Res, error)
		// - HandlerCtxNoErr: func(context.Context) Res
		paramType := t.In(0)

		if paramType.Implements(contextType) {
			// HandlerCtx or HandlerCtxNoErr: No Req type
			reqType = reflect.TypeOf(struct{}{})
		} else if implementsFromRequest(paramType) {
			// Solo or HandlerReq
			reqType = paramType
		} else {
			return nil, nil, fmt.Errorf("single parameter must be context.Context or implement FromRequest, got %v", paramType)
		}

	case 2:
		// Doppio or HandlerCtxReq: func(context.Context, *Req) (Res, error) or func(context.Context, *Req) Res
		ctxType := t.In(0)
		paramType := t.In(1)

		if !ctxType.Implements(contextType) {
			return nil, nil, fmt.Errorf("first parameter must be context.Context, got %v", ctxType)
		}

		if !implementsFromRequest(paramType) {
			return nil, nil, fmt.Errorf("second parameter must implement FromRequest, got %v", paramType)
		}

		reqType = paramType

	default:
		return nil, nil, fmt.Errorf("handler must have 0, 1, or 2 input parameters")
	}

	return reqType, resType, nil
}

// implementsFromRequest checks if type implements FromRequest.
func implementsFromRequest(t reflect.Type) bool {
	if t.Implements(fromRequestType) {
		return true
	}
	// Check pointer type
	if t.Kind() == reflect.Pointer && t.Elem().Implements(fromRequestType) {
		return true
	}
	return false
}

// createTypedHandler creates a typed handler using reflection.
// It supports layer application for type-inference mode using a dynamic
// Service[any, any] bridge. Validation and custom layers require explicit
// request types and should use WithLayersTyped.
//
//nolint:gocyclo // reflection-based dispatch and layer adaptation naturally increase branching
func createTypedHandler(handler any, reqType reflect.Type, _ reflect.Type, layers []LayerConfig) (http.HandlerFunc, error) {
	if len(layers) == 0 {
		return Handler(handler), nil
	}

	for _, layer := range layers {
		switch layer.(type) {
		case *validationConfig, *customConfig:
			return nil, fmt.Errorf("layer %T requires explicit request type; use WithLayersTyped", layer)
		}
	}

	v := reflect.ValueOf(handler)
	t := v.Type()
	if t.Kind() != reflect.Func {
		callMethod := v.MethodByName("Call")
		if !callMethod.IsValid() {
			return nil, fmt.Errorf("handler must be a function or Service interface, got %T", handler)
		}
		v = callMethod
		t = callMethod.Type()
	}

	wrapped := Service[any, any](serviceFunc[any, any](func(ctx context.Context, req any) (any, error) {
		args := make([]reflect.Value, 0, t.NumIn())
		for i := range t.NumIn() {
			inType := t.In(i)
			if inType.Implements(contextType) {
				args = append(args, reflect.ValueOf(ctx))
				continue
			}

			if req == nil {
				args = append(args, reflect.Zero(inType))
				continue
			}

			rv := reflect.ValueOf(req)
			switch {
			case rv.Type().AssignableTo(inType):
				args = append(args, rv)
			case rv.Kind() == reflect.Pointer && rv.Elem().Type().AssignableTo(inType):
				args = append(args, rv.Elem())
			default:
				return nil, fmt.Errorf("cannot use request type %T as %v", req, inType)
			}
		}

		results := v.Call(args)
		if len(results) == 2 {
			if e := results[1].Interface(); e != nil {
				handlerErr, ok := e.(error)
				if !ok {
					return nil, fmt.Errorf("handler returned non-error as second value: %T", e)
				}
				return results[0].Interface(), handlerErr
			}
		}
		return results[0].Interface(), nil
	}))

	for i := len(layers) - 1; i >= 0; i-- {
		wrapped = buildLayer[any, any](layers[i])(wrapped)
	}

	noReqType := reflect.TypeOf(struct{}{})
	return func(w http.ResponseWriter, r *http.Request) {
		var req any
		if reqType != noReqType {
			var reqVal reflect.Value
			if reqType.Kind() == reflect.Pointer {
				reqVal = reflect.New(reqType.Elem())
			} else {
				reqVal = reflect.New(reqType)
			}

			extractor, ok := reqVal.Interface().(FromRequest)
			if !ok {
				http.Error(w, "invalid request extractor type", http.StatusInternalServerError)
				return
			}

			if err := extractor.Extract(r); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if reqType.Kind() == reflect.Pointer {
				req = reqVal.Interface()
			} else {
				req = reqVal.Elem().Interface()
			}
		}

		res, err := wrapped.Call(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := writeResponse(w, res); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}, nil
}

// handlerToService converts various handler types to Service[Req, Res].
func handlerToService[Req FromRequest, Res IntoResponse](handler any) Service[Req, Res] {
	switch h := handler.(type) {
	case Service[Req, Res]:
		return h

	case func(context.Context, Req) (Res, error):
		return serviceFunc[Req, Res](h)

	case func(Req) (Res, error):
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			return h(req)
		})

	case func(context.Context) (Res, error):
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			return h(ctx)
		})

	case func() (Res, error):
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			return h()
		})

	case func(context.Context, Req) Res:
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			res := h(ctx, req)
			return res, nil
		})

	case func(Req) Res:
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			res := h(req)
			return res, nil
		})

	case func(context.Context) Res:
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			res := h(ctx)
			return res, nil
		})

	case func() Res:
		return serviceFunc[Req, Res](func(ctx context.Context, req Req) (Res, error) {
			res := h()
			return res, nil
		})

	default:
		panic(fmt.Sprintf("espresso: unsupported handler type: %T", handler))
	}
}

// applyLayersAndConvert applies layers and converts to http.HandlerFunc.
func applyLayersAndConvert[Req FromRequest, Res IntoResponse](svc Service[Req, Res], layers []LayerConfig) http.HandlerFunc {
	// Apply layers in reverse order (last added = outermost)
	wrapped := svc
	for i := len(layers) - 1; i >= 0; i-- {
		layer := buildLayer[Req, Res](layers[i])
		wrapped = layer(wrapped)
	}

	// Create pool for request objects
	var zero Req
	reqType := reflect.TypeOf(zero)
	if reqType == nil {
		panic("espresso: Req must be a concrete type, not any")
	}

	pool := &sync.Pool{
		New: func() any {
			if reqType.Kind() == reflect.Pointer {
				return reflect.New(reqType.Elem()).Interface()
			}
			return reflect.New(reqType).Interface()
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Get request from pool
		req, ok := pool.Get().(Req)
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		defer func() {
			resetReq(req)
			pool.Put(req)
		}()

		// Extract data from HTTP request
		if ext, ok := any(req).(FromRequest); ok {
			if err := ext.Extract(r); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		// Call service with layers applied
		res, err := wrapped.Call(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Write response
		if err := res.WriteResponse(w); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}
