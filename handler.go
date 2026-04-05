package espresso

import (
	"context"
	"net/http"
	"reflect"
	"strconv"
	"sync"
)

var (
	contextType     = reflect.TypeFor[context.Context]()
	fromRequestType = reflect.TypeFor[FromRequest]()
	errorType       = reflect.TypeFor[error]()
)

// handlerInfo caches reflection analysis results for handler functions.
// This improves performance by avoiding repeated reflection calls during registration.
type handlerInfo struct {
	numIn      int
	numOut     int
	hasContext bool
	ctxIndex   int
	reqPool    *sync.Pool
	reqType    reflect.Type
	reqIndex   int
}

// handlerCache stores parsed handler information to avoid repeated reflection.
var handlerCache sync.Map // map[reflect.Type]*handlerInfo

// Handler converts various handler types into http.HandlerFunc using reflection.
// For better performance, use the typed Handler* functions (HandlerCtxReqErr, etc).
//
// Supported handler signatures:
//
//	func() T
//		- No request extraction, returns T (auto-wrapped to Text or JSON)
//
//	func(*Req) T
//		- Extracts Req using FromRequest interface, returns T
//		- Req is pooled for memory efficiency
//
//	func(context.Context, *Req) (T, error)
//		- Receives context and extracted Req, returns T and optional error
//		- Most flexible signature for production use
//
//	func(context.Context, *Req) T
//		- Receives context and extracted Req, returns T
//
// Service interface:
//
//	Any struct implementing Service[*Req, Res] will have its Call method invoked
//	with context and extracted request. The Req type must implement FromRequest
//	with a POINTER receiver for proper data population.
//
// IMPORTANT: Request types must implement FromRequest with a POINTER receiver:
//
//	// Correct - pointer receiver
//	func (r *CreateUserReq) Extract(req *http.Request) error { ... }
//
//	// WRONG - value receiver (modifies a copy, original stays empty)
//	func (r CreateUserReq) Extract(req *http.Request) error { ... }
//
// Response auto-wrapping:
//
//   - IntoResponse: Used directly
//   - string: Wrapped as Text{Body: s}
//   - other types: Wrapped as JSON[any]{Data: v}
//
// For maximum performance, use the typed Handler* functions instead:
//
//	HandlerCtxReqErr(handler) // func(ctx, *Req) (Res, error)
//	HandlerCtxReq(handler)    // func(ctx, *Req) Res
//	HandlerReqErr(handler)    // func(*Req) (Res, error)
//	HandlerReq(handler)       // func(*Req) Res
//
// Panics:
//
//	This function panics at registration time (not request time) if the handler
//	signature is invalid. This ensures failures are caught during application startup.
func Handler(fn any) http.HandlerFunc {
	// Fast path: already an http.HandlerFunc
	if hf, ok := fn.(http.HandlerFunc); ok {
		return hf
	}
	// Also check for http.Handler interface
	if h, ok := fn.(http.Handler); ok {
		return h.ServeHTTP
	}

	v := reflect.ValueOf(fn)
	t := v.Type()

	if t.Kind() == reflect.Func {
		return handlerFunc(v, t)
	}

	callMethod := v.MethodByName("Call")
	if callMethod.IsValid() && isServiceSignature(callMethod.Type()) {
		return handlerFunc(callMethod, callMethod.Type())
	}

	panic("espresso: handler must be a function or implement Service interface, got " + t.Kind().String())
}

// HandlerCtxReqErr creates a handler for func(context.Context, *Req) (Res, error).
// This is the most performant handler for production use.
//
// Example:
//
//	app.Post("/users", HandlerCtxReqErr(createUser))
//
//	func createUser(ctx context.Context, req *CreateUserReq) (JSON[UserRes], error) {
//	    return JSON[UserRes]{Data: UserRes{ID: 1}}, nil
//	}
func HandlerCtxReqErr[Req FromRequest, Res IntoResponse](fn func(context.Context, Req) (Res, error)) http.HandlerFunc {
	pool := &sync.Pool{
		New: func() any {
			var zero Req
			return newReq(zero)
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req := pool.Get().(Req) //nolint:errcheck // poolNew returns correct type
		defer func() {
			resetReq(req)
			pool.Put(req)
		}()

		if err := req.Extract(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := fn(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := res.WriteResponse(w); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// HandlerCtxReq creates a handler for func(context.Context, *Req) Res.
//
// Example:
//
//	app.Post("/users", HandlerCtxReq(createUser))
//
//	func createUser(ctx context.Context, req *CreateUserReq) JSON[UserRes] {
//	    return JSON[UserRes]{Data: UserRes{ID: 1}}
//	}
func HandlerCtxReq[Req FromRequest, Res IntoResponse](fn func(context.Context, Req) Res) http.HandlerFunc {
	pool := &sync.Pool{
		New: func() any {
			var zero Req
			return newReq(zero)
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req := pool.Get().(Req) //nolint:errcheck // poolNew returns correct type
		defer func() {
			resetReq(req)
			pool.Put(req)
		}()

		if err := req.Extract(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		res := fn(r.Context(), req)
		if err := res.WriteResponse(w); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// HandlerReqErr creates a handler for func(*Req) (Res, error).
//
// Example:
//
//	app.Post("/users", HandlerReqErr(createUser))
//
//	func createUser(req *CreateUserReq) (JSON[UserRes], error) {
//	    return JSON[UserRes]{Data: UserRes{ID: 1}}, nil
//	}
func HandlerReqErr[Req FromRequest, Res IntoResponse](fn func(Req) (Res, error)) http.HandlerFunc {
	pool := &sync.Pool{
		New: func() any {
			var zero Req
			return newReq(zero)
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req := pool.Get().(Req) //nolint:errcheck // poolNew returns correct type
		defer func() {
			resetReq(req)
			pool.Put(req)
		}()

		if err := req.Extract(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := fn(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := res.WriteResponse(w); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// HandlerReq creates a handler for func(*Req) Res.
//
// Example:
//
//	app.Post("/users", HandlerReq(createUser))
//
//	func createUser(req *CreateUserReq) JSON[UserRes] {
//	    return JSON[UserRes]{Data: UserRes{ID: 1}}
//	}
func HandlerReq[Req FromRequest, Res IntoResponse](fn func(Req) Res) http.HandlerFunc {
	pool := &sync.Pool{
		New: func() any {
			var zero Req
			return newReq(zero)
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req := pool.Get().(Req) //nolint:errcheck // poolNew returns correct type
		defer func() {
			resetReq(req)
			pool.Put(req)
		}()

		if err := req.Extract(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		res := fn(req)
		if err := res.WriteResponse(w); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// HandlerCtx creates a handler for func(context.Context) (Res, error).
// Use for handlers that don't need request body extraction.
//
// Example:
//
//	app.Get("/health", HandlerCtx(healthCheck))
//
//	func healthCheck(ctx context.Context) (Text, error) {
//	    return Text{Body: "OK"}, nil
//	}
func HandlerCtx[Res IntoResponse](fn func(context.Context) (Res, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := fn(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := res.WriteResponse(w); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// HandlerCtxNoErr creates a handler for func(context.Context) Res.
// Use for handlers that don't need request body extraction and don't return errors.
//
// Example:
//
//	app.Get("/health", HandlerCtxNoErr(healthCheck))
//
//	func healthCheck(ctx context.Context) Text {
//	    return Text{Body: "OK"}
//	}
func HandlerCtxNoErr[Res IntoResponse](fn func(context.Context) Res) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := fn(r.Context())
		if err := res.WriteResponse(w); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// HandlerNoReq creates a handler for func() (Res, error).
// Use for simple handlers with no inputs.
//
// Example:
//
//	app.Get("/health", HandlerNoReq(healthCheck))
//
//	func healthCheck() (Text, error) {
//	    return Text{Body: "OK"}, nil
//	}
func HandlerNoReq[Res IntoResponse](fn func() (Res, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := fn()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := res.WriteResponse(w); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// HandlerNoReqNoErr creates a handler for func() Res.
// Use for simple handlers with no inputs and no errors.
//
// Example:
//
//	app.Get("/health", HandlerNoReqNoErr(healthCheck))
//
//	func healthCheck() Text {
//	    return Text{Body: "OK"}
//	}
func HandlerNoReqNoErr[Res IntoResponse](fn func() Res) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := fn()
		if err := res.WriteResponse(w); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// ============================================
// Coffee-Themed Aliases for Performance-Critical Paths
// ============================================
//
// Named after espresso shot sizes:
//   - Ristretto: "Restricted" shot - smallest, most concentrated (0 params)
//   - Solo: Single shot - standard espresso (1 param: request)
//   - Doppio: Double shot - full-powered (2 params: context + request)

// Ristretto creates a handler for func() Res.
// Named after the "restricted" espresso shot - smallest, most concentrated.
// Use for health checks and simple responses with no inputs.
//
// Example:
//
//	app.Get("/health", Ristretto(healthCheck))
//
//	func healthCheck() Text {
//	    return Text{Body: "OK"}
//	}
func Ristretto[Res IntoResponse](fn func() Res) http.HandlerFunc {
	return HandlerNoReqNoErr(fn)
}

// Solo creates a handler for func(*Req) (Res, error).
// Named after the single espresso shot - standard, one parameter.
// Use when context is not needed.
//
// Example:
//
//	app.Post("/users", Solo(createUser))
//
//	func createUser(req *CreateUserReq) (JSON[UserRes], error) {
//	    return JSON[UserRes]{Data: UserRes{ID: 1}}, nil
//	}
func Solo[Req FromRequest, Res IntoResponse](fn func(Req) (Res, error)) http.HandlerFunc {
	return HandlerReqErr(fn)
}

// Doppio creates a handler for func(context.Context, *Req) (Res, error).
// Named after the double espresso shot - two parameters, full-powered.
// Use for most common production handlers.
//
// Example:
//
//	app.Post("/users", Doppio(createUser))
//
//	func createUser(ctx context.Context, req *CreateUserReq) (JSON[UserRes], error) {
//	    return JSON[UserRes]{Data: UserRes{ID: 1}}, nil
//	}
func Doppio[Req FromRequest, Res IntoResponse](fn func(context.Context, Req) (Res, error)) http.HandlerFunc {
	return HandlerCtxReqErr(fn)
}

// newReq creates a new request object for the pool.
// Handles both pointer and value types correctly.
// Uses reflect.TypeOf to avoid nil Value issues.
func newReq[Req FromRequest](_ Req) Req {
	var zero Req
	reqType := reflect.TypeOf(zero)
	if reqType == nil {
		panic("espresso: FromRequest types must be concrete types")
	}
	if reqType.Kind() == reflect.Pointer {
		return reflect.New(reqType.Elem()).Interface().(Req) //nolint:errcheck // type guaranteed
	}
	return reflect.New(reqType).Interface().(Req) //nolint:errcheck // type guaranteed
}

// resetReq resets a request object using Resettable interface if available,
// otherwise uses reflection to zero the value.
func resetReq[Req FromRequest](req Req) {
	if resettable, ok := any(req).(Resettable); ok {
		resettable.Reset()
		return
	}
	// Fallback: use reflection
	rv := reflect.ValueOf(req)
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		rv.Elem().Set(reflect.Zero(rv.Elem().Type()))
	}
}

// isServiceSignature validates that a method signature matches Service.Call pattern.
func isServiceSignature(t reflect.Type) bool {
	if t.NumIn() < 1 || t.NumIn() > 2 {
		return false
	}
	if !t.In(0).Implements(contextType) {
		return false
	}
	if t.NumOut() < 1 || t.NumOut() > 2 {
		return false
	}
	if t.NumOut() == 2 && !t.Out(1).Implements(errorType) {
		return false
	}
	return true
}

//nolint:gocyclo // complexity is inherent to reflection-based handler creation
func handlerFunc(v reflect.Value, t reflect.Type) http.HandlerFunc {
	// Check cache first
	cached, ok := handlerCache.Load(t)
	if ok {
		info := cached.(*handlerInfo) //nolint:errcheck // type guaranteed by previous Store
		return createHandlerFromInfo(v, t, info)
	}

	// Analyze handler signature (expensive operation)
	numIn := t.NumIn()
	numOut := t.NumOut()

	if numOut == 0 || numOut > 2 {
		panic("espresso: handler must return 1 or 2 values, got " + strconv.Itoa(numOut))
	}

	if numOut == 2 && !t.Out(1).Implements(errorType) {
		panic("espresso: second return value must be error")
	}

	var (
		hasContext bool
		ctxIndex   int
		reqPool    *sync.Pool
		reqType    reflect.Type
		reqIndex   int
	)

	for i := range numIn {
		argType := t.In(i)
		if argType.Implements(contextType) {
			hasContext = true
			ctxIndex = i
		} else {
			implementsFromRequest := argType.Implements(fromRequestType)
			if argType.Kind() == reflect.Pointer {
				implementsFromRequest = implementsFromRequest || argType.Elem().Implements(fromRequestType)
			}
			if implementsFromRequest {
				reqPool = &sync.Pool{New: func() any {
					if argType.Kind() == reflect.Pointer {
						return reflect.New(argType.Elem()).Interface()
					}
					return reflect.New(argType).Interface()
				}}
				reqType = argType
				reqIndex = i
			} else {
				// Argument is neither context nor FromRequest
				panic("espresso: handler argument " + strconv.Itoa(i) + " must be context.Context or implement FromRequest")
			}
		}
	}

	// Create handler info
	info := &handlerInfo{
		numIn:      numIn,
		numOut:     numOut,
		hasContext: hasContext,
		ctxIndex:   ctxIndex,
		reqPool:    reqPool,
		reqType:    reqType,
		reqIndex:   reqIndex,
	}

	// Cache the analysis
	handlerCache.Store(t, info)

	return createHandlerFromInfo(v, t, info)
}

// createHandlerFromInfo creates a handler function from cached metadata.
//
//nolint:gocyclo // complexity is inherent to reflection-based handler creation
func createHandlerFromInfo(v reflect.Value, _ reflect.Type, info *handlerInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		args := make([]reflect.Value, info.numIn)

		var req any

		if info.reqPool != nil {
			req = info.reqPool.Get()
			defer func() {
				if req != nil && info.reqType != nil {
					rv := reflect.ValueOf(req)
					if rv.Kind() == reflect.Pointer && !rv.IsNil() {
						if resettable, ok := req.(Resettable); ok {
							resettable.Reset()
						} else {
							rv.Elem().Set(reflect.Zero(rv.Elem().Type()))
						}
					}
					info.reqPool.Put(req)
				}
			}()

			if ext, ok := req.(FromRequest); ok {
				if err := ext.Extract(r); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
		}

		for i := range info.numIn {
			if info.hasContext && i == info.ctxIndex {
				args[i] = reflect.ValueOf(r.Context())
			} else if info.reqPool != nil && i == info.reqIndex {
				rv := reflect.ValueOf(req)
				if info.reqType.Kind() != reflect.Pointer {
					args[i] = rv.Elem()
				} else {
					args[i] = rv
				}
			} else {
				// This should never happen due to validation above
				panic("espresso: invalid handler argument - this is a bug")
			}
		}

		results := v.Call(args)

		var res any
		var handlerErr error

		switch info.numOut {
		case 2:
			res = results[0].Interface()
			if e := results[1].Interface(); e != nil {
				handlerErr = e.(error) //nolint:errcheck // type checked at registration
			}
		case 1:
			res = results[0].Interface()
		}

		if handlerErr != nil {
			http.Error(w, handlerErr.Error(), http.StatusInternalServerError)
			return
		}

		if err := writeResponse(w, res); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

func writeResponse(w http.ResponseWriter, res any) error {
	switch v := res.(type) {
	case IntoResponse:
		return v.WriteResponse(w)
	case string:
		return Text{Body: v}.WriteResponse(w)
	default:
		return JSON[any]{Data: v}.WriteResponse(w)
	}
}
