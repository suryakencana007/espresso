# Basic REST API

This example demonstrates a simple REST API with CRUD operations.

## Project Setup

```bash
mkdir myapi && cd myapi
go mod init myapi
go get github.com/suryakencana007/espresso
```

## Directory Structure

```
myapi/
├── main.go
├── handlers/
│   └── user.go
├── models/
│   └── user.go
└── go.mod
```

## Models

```go
// models/user.go
package models

type User struct {
    ID    int64  `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type CreateUserReq struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

type UpdateUserReq struct {
    Name  string `json:"name,omitempty"`
    Email string `json:"email,omitempty"`
}

type UserPath struct {
    ID int64 `path:"id,required"`
}

type ListQuery struct {
    Page    int    `query:"page"`
   PerPage  int    `query:"per_page"`
    Search  string `query:"search"`
}
```

## Handlers

```go
// handlers/user.go
package handlers

import (
    "context"
    "net/http"
    
    "github.com/suryakencana007/espresso"
    "myapp/models"
)

type UserHandler struct {
    users map[int64]*models.User
    nextID int64
}

func NewUserHandler() *UserHandler {
    return &UserHandler{
        users: make(map[int64]*models.User),
        nextID: 1,
    }
}

// List returns all users
func (h *UserHandler) List(ctx context.Context, query *espresso.Query[models.ListQuery]) (espresso.JSON[[]models.User], error) {
    page := query.Data.Page
    if page < 1 {
        page = 1
    }
    perPage := query.Data.PerPage
    if perPage < 1 {
        perPage = 10
    }
    
    users := make([]models.User, 0, len(h.users))
    for _, u := range h.users {
        if query.Data.Search != "" {
            // Filter by search term
        }
        users = append(users, *u)
    }
    
    return espresso.JSON[[]models.User]{Data: users}, nil
}

// Get returns a single user
func (h *UserHandler) Get(ctx context.Context, path *espresso.Path[models.UserPath]) (espresso.JSON[models.User], error) {
    user, ok := h.users[path.Data.ID]
    if !ok {
        return espresso.JSON[models.User]{}, &espresso.Error{
            StatusCode: http.StatusNotFound,
            Message:    "user not found",
        }
    }
    return espresso.JSON[models.User]{Data: *user}, nil
}

// Create creates a new user
func (h *UserHandler) Create(ctx context.Context, req *espresso.JSON[models.CreateUserReq]) (espresso.JSON[models.User], error) {
    user := &models.User{
        ID:    h.nextID,
        Name:  req.Data.Name,
        Email: req.Data.Email,
    }
    h.users[h.nextID] = user
    h.nextID++
    
    return espresso.JSON[models.User]{
        StatusCode: http.StatusCreated,
        Data:       *user,
    }, nil
}

// Update updates an existing user
func (h *UserHandler) Update(ctx context.Context, path *espresso.Path[models.UserPath], req *espresso.JSON[models.UpdateUserReq]) (espresso.JSON[models.User], error) {
    user, ok := h.users[path.Data.ID]
    if !ok {
        return espresso.JSON[models.User]{}, &espresso.Error{
            StatusCode: http.StatusNotFound,
            Message:    "user not found",
        }
    }
    
    if req.Data.Name != "" {
        user.Name = req.Data.Name
    }
    if req.Data.Email != "" {
        user.Email = req.Data.Email
    }
    
    return espresso.JSON[models.User]{Data: *user}, nil
}

// Delete removes a user
func (h *UserHandler) Delete(ctx context.Context, path *espresso.Path[models.UserPath]) (espresso.Status, error) {
    if _, ok := h.users[path.Data.ID]; !ok {
        return 0, &espresso.Error{
            StatusCode: http.StatusNotFound,
            Message:    "user not found",
        }
    }
    
    delete(h.users, path.Data.ID)
    return espresso.Status(http.StatusNoContent), nil
}
```

## Main Application

```go
// main.go
package main

import (
    "github.com/suryakencana007/espresso"
    "myapp/handlers"
    "myapp/models"
)

func main() {
    // Initialize handlers
    users := handlers.NewUserHandler()
    
    // Create router
    router := espresso.Portafilter()
    
    // Health check
    router.Get("/health", func() string { return "OK" })
    
    // User routes
    router.Get("/users", espresso.Doppio(users.List))
    router.Get("/users/{id}", espresso.Doppio(users.Get))
    router.Post("/users", espresso.Doppio(users.Create))
    router.Put("/users/{id}", espresso.Lungo(users.Update))// Requires path + body
    router.Delete("/users/{id}", espresso.Doppio(users.Delete))
    
    // Start server
    router.Brew()
}
```

## Using Multiple Extractors

For handlers that need multiple extractors, use Lungo or manual extraction:

```go
// Using Lungo (path + JSON body)
func (h *UserHandler) Update(
    ctx context.Context,
    path *espresso.Path[models.UserPath],
    req *espresso.JSON[models.UpdateUserReq],
) (espresso.JSON[models.User], error) {
    // path.Data.ID contains path parameter
    // req.Data contains request body
}

// Register with Lungo
router.Put("/users/{id}", espresso.Lungo(users.Update))
```

## Error Handling

Define custom error types:

```go
// error.go
package main

import "net/http"

type APIError struct {
    StatusCode int    `json:"-"`
    Code       string `json:"code"`
    Message    string `json:"message"`
}

func (e *APIError) Error() string {
    return e.Message
}

func (e *APIError) WriteResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(e.StatusCode)
    return json.NewEncoder(w).Encode(map[string]string{
        "code":    e.Code,
        "message": e.Message,
    })
}

// Helper functions
func NotFound(message string) *APIError {
    return &APIError{
        StatusCode: http.StatusNotFound,
        Code:       "NOT_FOUND",
        Message:    message,
    }
}

func BadRequest(message string) *APIError {
    return &APIError{
        StatusCode: http.StatusBadRequest,
        Code:       "BAD_REQUEST",
        Message:    message,
    }
}
```

## Request Validation

Validate request data:

```go
func (h *UserHandler) Create(ctx context.Context, req *espresso.JSON[models.CreateUserReq]) (espresso.JSON[models.User], error) {
    // Validate
    if req.Data.Name == "" {
        return espresso.JSON[models.User]{}, BadRequest("name is required")
    }
    if req.Data.Email == "" {
        return espresso.JSON[models.User]{}, BadRequest("email is required")
    }
    if !isValidEmail(req.Data.Email) {
        return espresso.JSON[models.User]{}, BadRequest("invalid email format")
    }
    
    // Create user...
}
```

For more sophisticated validation, see [Production Setup](/examples/production).

## Testing

Write tests for your handlers:

```go
// handlers/user_test.go
package handlers

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    
    "github.com/suryakencana007/espresso"
)

func TestUserHandler_Create(t *testing.T) {
    handler := NewUserHandler()
    router := espresso.Portafilter()
    router.Post("/users", espresso.Doppio(handler.Create))
    
    body := `{"name":"John","email":"john@example.com"}`
    req := httptest.NewRequest("POST", "/users", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    if w.Code != http.StatusCreated {
        t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
    }
}
```

## Running

```bash
# Development
go run main.go

# Production
go build -o server .
./server
```

The server will start on `:3000` by default. Use `espresso.WithAddr(":8080")` to customize:

```go
router.Brew(espresso.WithAddr(":8080"))
```

## Adding OpenAPI Documentation

Add automatic OpenAPI documentation with `OpenAPIRouter`:

```go
package main

import (
    "reflect"
    
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/openapi"
    "myapp/handlers"
    "myapp/models"
)

func main() {
    // Create OpenAPI generator
    gen := openapi.New("My API", "1.0.0").
        Description("Basic REST API with CRUD operations").
        Server("http://localhost:3000", "Development")
    
    // Register schemas
    gen.Schema("User", reflect.TypeOf(models.User{}))
    
    // Use OpenAPIRouter for automatic documentation
    espresso.OpenAPI(gen).
        Get("/health", func() string { return "OK" }, openapi.Tags("health")).
        
        // User routes with documentation
        Get("/users", espresso.Doppio(users.List), 
            openapi.Tags("users"), 
            openapi.Summary("List all users")).
        Get("/users/{id}", espresso.Doppio(users.Get), 
            openapi.Tags("users"), 
            openapi.Summary("Get user by ID")).
        Post("/users", espresso.Doppio(users.Create), 
            openapi.Tags("users"), 
            openapi.Summary("Create a new user")).
        Put("/users/{id}", espresso.Lungo(users.Update), 
            openapi.Tags("users")).
        Delete("/users/{id}", espresso.Doppio(users.Delete), 
            openapi.Tags("users")).
        
        // Serve OpenAPI spec and documentation
        ServeOpenAPI("/openapi.json").
        ServeDocs("/docs", "/openapi.json").
        
        Brew(espresso.WithAddr(":3000"))
}
```

### Access Documentation

Once running, access:

- **API**: `http://localhost:3000/users`
- **OpenAPI Spec**: `http://localhost:3000/openapi.json`
- **Interactive Docs**: `http://localhost:3000/docs`

### Handler Introspection

`OpenAPIRouter` automatically detects parameter types:

```go
// Path parameter detected from extractor.Path[T]
func Get(ctx context.Context, path *espresso.Path[UserPath]) (espresso.JSON[User], error)

// Query parameters detected from extractor.Query[T]
func List(ctx context.Context, query *espresso.Query[ListQuery]) (espresso.JSON[[]User], error)

// Request body detected from espresso.JSON[T]
func Create(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error)
```

No manual path registration needed - just tag your handlers:

```go
.Get("/users/{id}", handler, openapi.Tags("users"))
```