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
	"github.com/suryakencana007/espresso/openapi"
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

// UpdateUserReq is used for updating user via JSON body.
// Demonstrates JSON extraction with Doppio and Lungo handlers.
type UpdateUserReq struct {
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

	// Initialize Application State
	db := initDB()
	config := Config{Port: 38080, Env: "development"}
	appState := AppState{
		DB:     db,
		Config: config,
		Logger: log.Logger,
	}

	// Setup OpenAPI generator with fluent API
	gen := openapi.New("Espresso API", "1.0.0").
		Description("Production-grade HTTP routing framework for Go").
		Server("http://localhost:38080", "Development").
		Server("https://api.example.com", "Production")

	// Use OpenAPIRouter for automatic documentation generation
	espresso.OpenAPI(gen).
		Use(httpmiddleware.RequestIDMiddleware()).
		Use(httpmiddleware.RecoverMiddleware()).
		Use(httpmiddleware.LoggingMiddleware()).
		WithState(appState).
		// Health endpoints
		Get("/api/health", healthCheck, openapi.Tags("health")).
		Get("/api/ping", ping, openapi.Summary("Ping endpoint")).
		// User endpoints
		Post("/api/users", createUserJSON, openapi.Tags("users"), openapi.Summary("Create user")).
		Get("/api/users/{id}", getUserWithState, openapi.Tags("users"), openapi.Summary("Get user by ID")).
		Put("/api/users/{id}", updateUserLungo, openapi.Tags("users")).
		Post("/api/users/update", updateUserDoppio, openapi.Tags("users")).
		// Search and config endpoints
		Get("/api/search", searchQuery, openapi.Tags("search")).
		Get("/api/config", configHandler, openapi.Tags("config")).
		Get("/api/db-status", dbStatusHandler, openapi.Tags("system")).
		// Auth endpoints
		Post("/api/auth", authHeader, openapi.Tags("auth")).
		// Brew the server
		Brew(espresso.WithAddr(":38080"))

	// Serve OpenAPI spec and documentation
	http.Handle("/openapi.json", gen.Handler())
	http.Handle("/docs", openapi.ScalarUIHandler("/openapi.json"))
	log.Info().Msg("OpenAPI spec: http://localhost:38080/openapi.json")
	log.Info().Msg("API docs: http://localhost:38080/docs")
}

func initDB() *sql.DB {
	return nil
}

// Handler Examples Using OpenAPIRouter

// Note: OpenAPIRouter automatically introspects handler signatures
// and generates OpenAPI documentation. No manual setup needed!
// Handler Examples using State (NEW!)
// ============================================

// createUserJSON demonstrates JSON[T] extractor with OpenAPIRouter.
func createUserJSON(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
	user := req.Data
	log.Info().Str("name", user.Name).Msg("creating user")
	return espresso.JSON[UserRes]{
		StatusCode: http.StatusCreated,
		Data:       UserRes{Message: fmt.Sprintf("User '%s' created", user.Name)},
	}, nil
}

// getUserWithState demonstrates state access in handler.
func getUserWithState(ctx context.Context, req *extractor.Path[UserPathReq]) (espresso.JSON[UserRes], error) {
	state := espresso.MustGetState[AppState](ctx)
	_ = state.Logger
	_ = state.DB
	userID := req.Data.ID

	return espresso.JSON[UserRes]{
		Data: UserRes{
			Message: fmt.Sprintf("User %d (env: %s)", userID, state.Config.Env),
			UserID:  userID,
		},
	}, nil
}

// updateUserLungo demonstrates Lungo handler with path + JSON body extraction.
func updateUserLungo(ctx context.Context, path *extractor.Path[UserPathReq], body *espresso.JSON[UpdateUserReq]) (espresso.JSON[UserRes], error) {
	state := espresso.MustGetState[AppState](ctx)
	userID := path.Data.ID
	req := body.Data

	log.Info().
		Int("user_id", userID).
		Str("name", req.Name).
		Str("email", req.Email).
		Msg("updating user")

	return espresso.JSON[UserRes]{
		StatusCode: http.StatusOK,
		Data: UserRes{
			Message: fmt.Sprintf("User %d updated: name=%s, email=%s (env: %s)",
				userID, req.Name, req.Email, state.Config.Env),
			UserID: userID,
		},
	}, nil
}

// updateUserDoppio demonstrates Doppio handler with JSON body only.
func updateUserDoppio(ctx context.Context, body *espresso.JSON[UpdateUserReq]) (espresso.JSON[UserRes], error) {
	state := espresso.MustGetState[AppState](ctx)
	req := body.Data

	log.Info().
		Str("name", req.Name).
		Str("email", req.Email).
		Msg("updating user (body only)")

	return espresso.JSON[UserRes]{
		StatusCode: http.StatusOK,
		Data: UserRes{
			Message: fmt.Sprintf("User updated: name=%s, email=%s (env: %s)",
				req.Name, req.Email, state.Config.Env),
		},
	}, nil
}

// configHandler demonstrates state access without request body.
func configHandler(ctx context.Context) (espresso.JSON[struct {
	Env  string
	Port int
}], error) {
	state, ok := espresso.GetState[AppState](ctx)
	if !ok {
		return espresso.JSON[struct {
			Env  string
			Port int
		}]{}, fmt.Errorf("state not found in context")
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
func searchQuery(ctx context.Context, req *extractor.Query[SearchReq]) (espresso.JSON[SearchRes], error) {
	params := req.Data
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

// Error Handling Examples (for reference)

// Error Handling Examples (for reference)

// createUserWithError demonstrates structured error responses.
//
//nolint:unused,unparam
func createUserWithError(_ context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
	user := req.Data

	if user.Name == "" {
		return espresso.JSON[UserRes]{}, espresso.ValidationErrors([]espresso.ValidationError{
			{Field: "name", Message: "name is required"},
		})
	}

	if user.Email == "exists@example.com" {
		return espresso.JSON[UserRes]{}, espresso.Conflict("user with this email already exists")
	}

	if user.Email == "notfound@example.com" {
		return espresso.JSON[UserRes]{}, espresso.NotFound("user not found")
	}

	if user.Email == "unauthorized@example.com" {
		return espresso.JSON[UserRes]{}, espresso.Unauthorized("invalid credentials")
	}

	return espresso.JSON[UserRes]{
		StatusCode: http.StatusCreated,
		Data:       UserRes{Message: fmt.Sprintf("User '%s' created successfully", user.Name)},
	}, nil
}

// circuitBreakerExample demonstrates circuit breaker error handling.
//
//nolint:unused,unparam
func circuitBreakerExample(_ context.Context, req *extractor.Path[UserPathReq]) (espresso.JSON[UserRes], error) {
	userID := req.Data.ID

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
