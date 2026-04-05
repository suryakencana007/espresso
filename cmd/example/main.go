package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/suryakencana007/espresso"
	"github.com/suryakencana007/espresso/extractor"
	httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
	servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
)

// ============================================
// Application State (Dependency Injection)
// ============================================

// AppState holds application-wide dependencies.
// State is immutable and thread-safe - use sync primitives for mutable data.
type AppState struct {
	DB     *sql.DB
	Config Config
	Logger zerolog.Logger
}

type Config struct {
	Port int
	Env  string
}

// ============================================
// Axum-style Extractors - NO manual Extract() needed!
// ============================================

// CreateUserReq is automatically extracted from JSON body.
// Just use JSON[CreateUserReq] in your handler - no Extract method needed!
type CreateUserReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// SearchReq demonstrates query parameter extraction.
// Use struct tags with `query:"name"` - supports required fields with ",required".
type SearchReq struct {
	Query string `query:"q,required"`
	Page  int    `query:"page"`
	Limit int    `query:"limit"`
}

// UserPathReq demonstrates path parameter extraction.
// Router must call SetPathParams() before handler. Use `path:"param_name"`.
type UserPathReq struct {
	ID int `path:"id"`
}

// AuthReq demonstrates header extraction.
type AuthReq struct {
	Token string `header:"Authorization,required"`
}

// CreateUserWithRoleReq demonstrates custom extraction - still works! Combine multiple sources.
type CreateUserWithRoleReq struct {
	Role    string // extracted from query param
	Payload struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
}

func (req *CreateUserWithRoleReq) Extract(r *http.Request) error {
	req.Role = r.URL.Query().Get("role")
	if req.Role == "" {
		req.Role = "user"
	}
	return espresso.DecodeSafeJSON(r, &req.Payload)
}

func (req *CreateUserWithRoleReq) Reset() {
	req.Role = ""
	req.Payload.Name = ""
	req.Payload.Email = ""
}

// UserRes is the response type for user operations.
type UserRes struct {
	Message string `json:"message"`
	UserID  int    `json:"user_id,omitempty"`
}

type SearchRes struct {
	Results []string `json:"results"`
	Query   string   `json:"query"`
	Page    int      `json:"page"`
}

// UserService handles user-related business logic.
type UserService struct{}

func (s UserService) Call(_ context.Context, req *CreateUserWithRoleReq) (espresso.JSON[UserRes], error) { //nolint:unparam
	msg := fmt.Sprintf("Created user '%s' (%s) with role: %s",
		req.Payload.Name, req.Payload.Email, req.Role)
	return espresso.JSON[UserRes]{
		StatusCode: http.StatusCreated,
		Data:       UserRes{Message: msg},
	}, nil
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// ============================================
	// Initialize Application State
	// ============================================
	db := initDB() // your DB initialization
	config := Config{Port: 38080, Env: "development"}

	appState := AppState{
		DB:     db,
		Config: config,
		Logger: log.Logger,
	}

	// ============================================
	// Reusable Layer Stacks (NEW!)
	// ============================================
	// Define type-erased layers once, reuse across handlers

	commonLayers := espresso.Layers(
		espresso.Timeout(5*time.Second),
		espresso.Logging(log.Logger, "api"),
	)

	// Route-specific layers
	userLayers := espresso.Layers(
		espresso.Timeout(10*time.Second),
		espresso.Logging(log.Logger, "users"),
	)

	// ============================================
	// Chain Pattern with WithState (NEW!)
	// ============================================
	// WithState injects application state into all handlers

	espresso.Portafilter().
		// Middleware
		Use(httpmiddleware.RequestIDMiddleware()).
		Use(httpmiddleware.RecoverMiddleware()).
		Use(httpmiddleware.LoggingMiddleware()).

		// State Injection (NEW!)
		WithState(appState).

		// ============================================
		// NEW: WithLayers - Type Inference
		// ============================================
		// Types are inferred from handler signature

		Post("/api/users", espresso.WithLayers(createUserJSON, userLayers...)).
		Get("/api/users/{id}", espresso.WithLayers(getUserWithState, userLayers...)).
		Get("/api/search", espresso.WithLayers(searchQuery, commonLayers...)).

		// ============================================
		// State Access Example
		// ============================================
		// Handler uses GetState to access injected dependencies

		Get("/api/config", espresso.Handler(configHandler)).
		Get("/api/db-status", espresso.Handler(dbStatusHandler)).

		// ============================================
		// Explicit Types (Fallback)
		// ============================================
		// Use WithLayersTyped when inference fails

		Post("/api/auth", espresso.WithLayersTyped[*extractor.Header[AuthReq], espresso.Text](
			authHeader,
			commonLayers...,
		)).

		// ============================================
		// NEW: WithLayers with Service
		// ============================================
		// Apply layers to Service structs

		Post("/api/users/custom", espresso.WithLayersTyped[*CreateUserWithRoleReq, espresso.JSON[UserRes]](
			UserService{},
			espresso.Layers(
				espresso.Logging(log.Logger, "users"),
				espresso.Timeout(5*time.Second),
			)...,
		)).

		// ============================================
		// Simple handlers (no layers)
		// ============================================

		Get("/api/health", espresso.Handler(healthCheck)).
		Get("/api/ping", espresso.Ristretto(ping)).
		Brew(espresso.WithAddr(":38080"))
}

// initDB initializes database connection (placeholder).
func initDB() *sql.DB {
	// In real app: connect to actual database
	// db, err := sql.Open("postgres", dsn)
	// This is just a placeholder for the example
	return nil
}

// ============================================
// Handler Examples using State (NEW!)
// ============================================

// createUserJSON demonstrates JSON[T] extractor.
// Note: Use pointer type *JSON[T] to satisfy FromRequest interface.
func createUserJSON(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
	user := req.Data // Data contains the decoded CreateUserReq
	log.Info().Str("name", user.Name).Msg("creating user")

	return espresso.JSON[UserRes]{
		StatusCode: http.StatusCreated,
		Data: UserRes{
			Message: fmt.Sprintf("User '%s' created", user.Name),
		},
	}, nil
}

// getUserWithState demonstrates state access in handler.
// Use MustGetState[T] when state is guaranteed to be present.
func getUserWithState(ctx context.Context, req *extractor.Path[UserPathReq]) (espresso.JSON[UserRes], error) {
	// Access application state (NEW!)
	state := espresso.MustGetState[AppState](ctx)

	// Use dependencies from state
	_ = state.Logger // Use logger
	_ = state.DB     // Use database
	userID := req.Data.ID

	// Example: state.DB.Query("SELECT * FROM users WHERE id = ?", userID)
	// This shows how to access DB from state

	return espresso.JSON[UserRes]{
		Data: UserRes{
			Message: fmt.Sprintf("User %d (env: %s)", userID, state.Config.Env),
			UserID:  userID,
		},
	}, nil
}

// configHandler demonstrates state access without request body.
func configHandler(ctx context.Context) (espresso.JSON[struct {
	Env  string
	Port int
}], error) {
	// GetState returns (state, ok) - use when state might not be present
	state, ok := espresso.GetState[AppState](ctx)
	if !ok {
		return espresso.JSON[struct {
				Env  string
				Port int
			}]{},
			fmt.Errorf("state not found in context")
	}

	return espresso.JSON[struct {
		Env  string
		Port int
	}]{
		Data: struct {
			Env  string
			Port int
		}{
			Env:  state.Config.Env,
			Port: state.Config.Port,
		},
	}, nil
}

// dbStatusHandler demonstrates state with nil check for DB.
func dbStatusHandler(ctx context.Context) (espresso.JSON[struct{ Status string }], error) {
	state := espresso.MustGetState[AppState](ctx)

	status := "connected"
	if state.DB == nil {
		status = "disconnected (nil)"
	}

	return espresso.JSON[struct{ Status string }]{
		Data: struct{ Status string }{Status: status},
	}, nil
}

// searchQuery demonstrates Query[T] extractor.
// Struct tags define query parameter mapping: `query:"name"` or `query:"name,required"`.
func searchQuery(ctx context.Context, req *extractor.Query[SearchReq]) (espresso.JSON[SearchRes], error) {
	params := req.Data // Data contains the decoded query params

	// defaults
	if params.Page == 0 {
		params.Page = 1
	}
	if params.Limit == 0 {
		params.Limit = 10
	}

	return espresso.JSON[SearchRes]{
		Data: SearchRes{
			Results: []string{"result1", "result2"},
			Query:   params.Query,
			Page:    params.Page,
		},
	}, nil
}

// authHeader demonstrates Header[T] extractor.
func authHeader(ctx context.Context, req *extractor.Header[AuthReq]) (espresso.Text, error) {
	token := req.Data.Token
	log.Info().Str("token", token).Msg("auth request")

	return espresso.Text{Body: "Authenticated"}, nil
}

// healthCheck demonstrates a handler with context but no request body.
func healthCheck(ctx context.Context) (espresso.JSON[struct{ Status string }], error) {
	return espresso.JSON[struct{ Status string }]{Data: struct{ Status string }{Status: "healthy"}}, nil
}

// ping demonstrates the simplest handler - no inputs, no errors.
func ping() espresso.Text {
	return espresso.Text{Body: "pong"}
}

// ============================================
// Structured Error Handling Examples
// ============================================

// createUserWithError demonstrates structured error responses.
// Use espresso.BadRequest(), NotFound(), etc. for consistent API errors.
//
//nolint:unused,unparam // Example function for documentation
func createUserWithError(_ context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
	user := req.Data

	// Validation error with details
	if user.Name == "" {
		return espresso.JSON[UserRes]{}, espresso.ValidationErrors([]espresso.ValidationError{
			{Field: "name", Message: "name is required"},
		})
	}

	// Business logic error
	if user.Email == "exists@example.com" {
		return espresso.JSON[UserRes]{}, espresso.Conflict("user with this email already exists")
	}

	// Simulated not found error
	if user.Email == "notfound@example.com" {
		return espresso.JSON[UserRes]{}, espresso.NotFound("user not found")
	}

	// Unauthorized error
	if user.Email == "unauthorized@example.com" {
		return espresso.JSON[UserRes]{}, espresso.Unauthorized("invalid credentials")
	}

	return espresso.JSON[UserRes]{
		StatusCode: http.StatusCreated,
		Data: UserRes{
			Message: fmt.Sprintf("User '%s' created successfully", user.Name),
		},
	}, nil
}

// circuitBreakerExample demonstrates circuit breaker with custom error handling.
// Note: CircuitBreakerError is returned when circuit is open.
//
//nolint:unused,unparam // Example function for documentation
func circuitBreakerExample(_ context.Context, req *extractor.Path[UserPathReq]) (espresso.JSON[UserRes], error) {
	userID := req.Data.ID

	// Simulate circuit breaker error
	// In real usage, this would come from a service call wrapped in circuit breaker
	if userID == 999 {
		return espresso.JSON[UserRes]{}, espresso.NewCircuitBreakerError(
			"UserService",
			servicemiddleware.StateOpen,
			"service temporarily unavailable",
		)
	}

	return espresso.JSON[UserRes]{
		Data: UserRes{Message: fmt.Sprintf("User %d processed", userID)},
	}, nil
}
