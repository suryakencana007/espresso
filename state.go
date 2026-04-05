package espresso

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
)

type stateKey struct{}

// StateNotFoundError is returned when state cannot be found in context.
type StateNotFoundError struct {
	Type reflect.Type
}

func (e *StateNotFoundError) Error() string {
	return fmt.Sprintf("espresso: state of type %v not found in context", e.Type)
}

// GetState retrieves state from context with type safety.
// Returns the state and true if found, or zero value and false if not found.
//
// Example:
//
//	func handler(ctx context.Context, req *espresso.JSON[Req]) (Res, error) {
//	    state, ok := espresso.GetState[AppState](ctx)
//	    if !ok {
//	        return Res{}, errors.New("state not found")
//	    }
//	    // use state...
//	}
func GetState[T any](ctx context.Context) (T, bool) {
	var zero T
	state, ok := ctx.Value(stateKey{}).(T)
	if !ok {
		return zero, false
	}
	return state, true
}

// MustGetState retrieves state or panics if not found.
// Use when state is guaranteed to be present (e.g., after WithState middleware).
//
// Example:
//
//	func handler(ctx context.Context, req *espresso.JSON[Req]) (Res, error) {
//	    state := espresso.MustGetState[AppState](ctx)
//	    // use state - panics if not found
//	}
func MustGetState[T any](ctx context.Context) T {
	state, ok := GetState[T](ctx)
	if !ok {
		var zero T
		panic(fmt.Sprintf("espresso: state of type %T not found in context", zero))
	}
	return state
}

// WithStateMiddleware creates a middleware that injects state into the request context.
// Use this to provide application-wide state to all handlers.
//
// Example:
//
//	appState := AppState{DB: db, Config: config}
//	router.Use(espresso.WithStateMiddleware(appState))
func WithStateMiddleware[T any](state T) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), stateKey{}, state)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// State is an extractor that provides type-safe access to application state.
// Similar to Axum's State<S> extractor, but designed for Go.
//
// Example:
//
//	type AppState struct {
//	    DB     *sql.DB
//	    Config Config
//	}
//
//	func handler(ctx context.Context, req *espresso.JSON[Req], state espresso.State[AppState]) (Res, error) {
//	    db := state.Data.DB
//	    // use db...
//	}
//
//	// Route registration:
//	router.Post("/users", espresso.Doppio(createUser))
type State[T any] struct {
	Data T
}

// Extract implements FromRequest to populate state from context.
func (s *State[T]) Extract(r *http.Request) error {
	state, ok := GetState[T](r.Context())
	if !ok {
		return &StateNotFoundError{Type: reflect.TypeFor[T]()}
	}
	s.Data = state
	return nil
}

// Reset clears the state data for reuse.
func (s *State[T]) Reset() {
	var zero T
	s.Data = zero
}

// FromState extracts a sub-component from parent state using a getter function.
// Useful for substate pattern - extracting specific components from application state.
//
// Example:
//
//	type AppState struct {
//	    DB     *sql.DB
//	    Config Config
//	}
//
//	// Extract only DB from AppState
//	db, ok := espresso.FromState[AppState, *sql.DB](ctx, func(s AppState) *sql.DB {
//	    return s.DB
//	})
func FromState[S any, T any](ctx context.Context, getter func(S) T) (T, bool) {
	var zero T
	state, ok := GetState[S](ctx)
	if !ok {
		return zero, false
	}
	return getter(state), true
}

// FromMustState extracts a sub-component or panics if state not found.
func FromMustState[S any, T any](ctx context.Context, getter func(S) T) T {
	state := MustGetState[S](ctx)
	return getter(state)
}
