# Routing

Espresso uses Go 1.22+ ServeMux routing with path parameters and wildcards support.

## Basic Routes

```go
router := espresso.Portafilter()

// Simple routes
router.Get("/health", func() string { return "ok" })
router.Post("/users", createUser)
router.Put("/users/{id}", updateUser)
router.Delete("/users/{id}", deleteUser)
router.Patch("/users/{id}", patchUser)

// Start server
router.Brew()
```

## Path Parameters

Go 1.22+ supports path parameters using `{name}` syntax:

```go
type UserPath struct {
    ID int64 `path:"id,required"`
}

func getUser(ctx context.Context, req *espresso.Path[UserPath]) (espresso.JSON[User], error) {
    // req.Data.ID contains the path parameter
    user := findUser(req.Data.ID)
    return espresso.JSON[User]{Data: user}, nil
}

// Register with path parameter
router.Get("/users/{id}", espresso.Doppio(getUser))
```

### Path Parameter Types

Supported types for path parameters:

| Type | Example | Notes |
|------|---------|-------|
| `string` | `/files/{path}` | Default - matches any string |
| `int` | `/users/{id}` | Parsed automatically |
| `int64` | `/items/{id}` | 64-bit integer |
| `uint` | `/ports/{num}` | Unsigned integer |

```go
type PathParams struct {
    ID     int64  `path:"id,required"`
    Slug   string `path:"slug"`
    Status int    `path:"status"`
}
```

## Wildcards

Use `{...}` or `*` for wildcard matching:

```go
// Match any path under /files/
router.Get("/files/{path...}", fileHandler)

// Equivalent using *
router.Get("/static/*", staticHandler)
```

### Wildcard Handler

```go
func fileHandler(ctx context.Context, req *espresso.Path[struct{}]) (espresso.Text, error) {
    // Access the wildcard portion
    path := req.Data.(map[string]string)["path..."]
    // or use r.PathValue("path...") directly
    return espresso.Text{Body: "File: " + path}, nil
}
```

## Route Groups

Group routes using method chaining:

```go
router := espresso.Portafilter()

// User routes
router.Get("/users", listUsers).
      Get("/users/{id}", getUser).
      Post("/users", createUser).
      Put("/users/{id}", updateUser).
      Delete("/users/{id}", deleteUser)

// Blog routes
router.Get("/posts", listPosts).
      Get("/posts/{slug}", getPost).
      Post("/posts", createPost)
```

## HTTP Methods

Espresso supports all standard HTTP methods:

| Method | Function | Description |
|--------|----------|-------------|
| `GET` | `router.Get()` | Retrieve resources |
| `POST` | `router.Post()` | Create resources |
| `PUT` | `router.Put()` | Replace resources |
| `PATCH` | `router.Patch()` | Partial update |
| `DELETE` | `router.Delete()` | Remove resources |
| `HEAD` | `router.Head()` | Get headers only |
| `OPTIONS` | `router.Options()` | CORS preflight |

## Route Matching Priority

Go 1.22+ ServeMux uses longest-prefix matching:

1. Exact match: `/users/profile` (highest priority)
2. Path parameter: `/users/{id}`
3. Wildcard: `/users/{path...}` (lowest priority)

```go
// More specific routes have higher priority
router.Get("/users/me", meHandler)           // Exact match
router.Get("/users/{id}", userHandler)       // Path param
router.Get("/users/{path...}", otherHandler) // Wildcard

// Request: GET /users/me     -> meHandler
// Request: GET /users/123    -> userHandler
// Request: GET /users/foo/bar-> otherHandler
```

## Method Not Allowed

Handle unsupported methods by checking the request:

```go
func handler() espresso.JSON[Response] {
    return espresso.JSON[Response]{
        StatusCode: http.StatusMethodNotAllowed,
        Data: Response{Error: "method not allowed"},
    }
}
```

For automatic 404/405 handling, use middleware:

```go
router.Use(httpmiddleware.RecoverMiddleware())
```

## Best Practices

1. **Use path parameters** for resource IDs
2. **Mark required parameters** with `path:"id,required"`
3. **Use wildcards sparingly** - prefer explicit routes
4. **Group related routes** by resource
5. **Apply middleware** before route registration