---
title: OpenAPI API Reference
description: OpenAPI specification generator
---

# OpenAPI API Reference

Package `openapi` provides OpenAPI 3.0 specification generation.

```go
import "github.com/suryakencana007/espresso/openapi"
```

## Generator

### New (Recommended)

Create a new OpenAPI generator with fluent API:

```go
func New(title, version string) *Generator
```

Example:

```go
gen := openapi.New("My API", "1.0.0").
    Description("REST API for my application").
    Server("http://localhost:8080", "Development")
```

### NewGenerator (Deprecated)

Legacy function - use `New()` instead:

```go
func NewGenerator(title, version string) *Generator
```

## Configuration

### Description (Recommended)

Set API description with fluent API:

```go
gen := openapi.New("My API", "1.0.0").
    Description("REST API for user management").
    Server("http://localhost:8080", "Dev")
```

### SetDescription (Deprecated)

Legacy method - use `Description()` instead:

```go
gen.SetDescription("REST API for my application")
```

### Server (Recommended)

Add server with fluent API:

```go
gen := openapi.New("My API", "1.0.0").
    Server("http://localhost:8080", "Development").
    Server("https://api.example.com", "Production")
```

### AddServer (Deprecated)

Legacy method - use `Server()` instead:

```go
gen.AddServer("http://localhost:8080", "Local development")
```

## Building Specs

### Schema (Recommended)

Add schema with fluent API:

```go
type User struct {
    ID    int    `json:"id" doc:"User ID"`
    Name  string `json:"name" doc:"User name"`
    Email string `json:"email" doc:"User email"`
}

gen := openapi.New("My API", "1.0.0").
    Schema("User", reflect.TypeOf(User{})).
    Schema("Post", reflect.TypeOf(Post{}))
```

### AddSchema

Legacy method for adding schema:

```go
userSchema := &openapi.Schema{
    Type: "object",
    Properties: map[string]*openapi.Schema{
        "id":    {Type: "integer"},
        "name":  {Type: "string"},
        "email": {Type: "string", Format: "email"},
    },
    Required: []string{"id", "name"},
}

gen.AddSchema("User", userSchema)
```

### AddPath

Add a path to the spec:

```go
op := openapi.Operation{
    Summary: "Get users",
    Responses: map[string]openapi.Response{
        "200": {
            Description: "Success",
            Content: map[string]openapi.MediaType{
                "application/json": {
                    Schema: &openapi.Schema{
                        Type: "array",
                        Items: &openapi.Schema{
                            Type: "object",
                            Properties: map[string]*openapi.Schema{
                                "id":   {Type: "integer"},
                                "name": {Type: "string"},
                            },
                        },
                    },
                },
            },
        },
    },
}

gen.AddPath("GET", "/users", op)
```

## Generating Schemas

### GenerateSchemaFromType

Generate OpenAPI schema from Go type:

```go
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty" doc:"User email address"`
}

schema := openapi.GenerateSchemaFromType(reflect.TypeOf(User{}))
// {
//   "type": "object",
//   "properties": {
//     "id": {"type": "integer"},
//     "name": {"type": "string"},
//     "email": {"type": "string", "description": "User email address"}
//   },
//   "required": ["id", "name"]
// }
```

## Serving

### Handler

Serve OpenAPI spec as JSON:

```go
gen := openapi.New("My API", "1.0.0").
    Description("REST API").
    Server("http://localhost:8080", "Dev")

http.Handle("/openapi.json", gen.Handler())
http.ListenAndServe(":8080", nil)
```

### ServeOpenAPI (Recommended)

Use OpenAPIRouter for integrated routing and documentation:

```go
gen := openapi.New("My API", "1.0.0").
    Description("REST API").
    Server("http://localhost:8080", "Dev")

espresso.OpenAPI(gen).
    Get("/api/users", getUsers, openapi.Tags("users")).
    Post("/api/users", createUser, openapi.Summary("Create user")).
    ServeOpenAPI("/openapi.json").
    ServeDocs("/docs", "/openapi.json").
    Brew(espresso.WithAddr(":8080"))
```

This integrates the spec into your router - no separate `http.Handle()` calls needed!

### JSON (Convenience)

Get spec as JSON bytes:

```go
data, err := gen.JSON()
if err != nil {
    log.Fatal(err)
}
os.WriteFile("openapi.json", data, 0644)
```

### ScalarUI

Serve Scalar UI:

```go
gen := openapi.New("My API", "1.0.0")

// Serve spec
http.Handle("/openapi.json", gen.Handler())

// Serve Scalar UI
http.Handle("/docs", openapi.ScalarUIHandler("/openapi.json"))
// or with options
http.Handle("/docs", openapi.ScalarUI(openapi.ScalarOpts{
    Title:   "My API Documentation",
    SpecURL: "/openapi.json",
}))

http.ListenAndServe(":8080", nil)
```

## Types

### Spec

```go
type Spec struct {
    OpenAPI    string                 `json:"openapi"`
    Info       Info                   `json:"info"`
    Servers    []Server               `json:"servers,omitempty"`
    Paths      map[string]PathItem    `json:"paths"`
    Components map[string]any `json:"components,omitempty"`
}
```

### Operation

```go
type Operation struct {
    Summary     string                 `json:"summary,omitempty"`
    Description string                 `json:"description,omitempty"`
    Tags        []string               `json:"tags,omitempty"`
    Parameters  []Parameter            `json:"parameters,omitempty"`
    RequestBody *RequestBody           `json:"requestBody,omitempty"`
    Responses   map[string]Response    `json:"responses"`
    Security    []map[string][]string  `json:"security,omitempty"`
}
```

### Schema

```go
type Schema struct {
    Type                 string             `json:"type,omitempty"`
    Format               string             `json:"format,omitempty"`
    Description          string             `json:"description,omitempty"`
    Properties           map[string]*Schema `json:"properties,omitempty"`
    Required             []string           `json:"required,omitempty"`
    Items               *Schema            `json:"items,omitempty"`
AdditionalProperties any `json:"additionalProperties,omitempty"`
    Example             any `json:"example,omitempty"`
    Ref                 string             `json:"$ref,omitempty"`
}
```

## Example

Complete example with OpenAPIRouter integration:

```go
package main

import (
    "context"
    "net/http"
    "reflect"
    
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/openapi"
)

type User struct {
    ID    int    `json:"id" doc:"User ID"`
    Name  string `json:"name" doc:"User name"`
    Email string `json:"email,omitempty" doc:"User email address"`
}

type UserPath struct {
    ID int `path:"id"`
}

type CreateUserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func getUsers(ctx context.Context) (espresso.JSON[[]User], error) {
    return espresso.JSON[[]User]{Data: []User{{ID: 1, Name: "John"}}}, nil
}

func getUser(ctx context.Context, path *espresso.Path[UserPath]) (espresso.JSON[User], error) {
    return espresso.JSON[User]{Data: User{ID: path.Data.ID, Name: "John"}}, nil
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    return espresso.JSON[User]{
        StatusCode: http.StatusCreated,
        Data:       User{ID: 1, Name: req.Data.Name},
    }, nil
}

func main() {
    // Create OpenAPI generator
    gen := openapi.New("User API", "1.0.0").
        Description("API for managing users").
        Server("http://localhost:8080", "Development").
        Schema("User", reflect.TypeOf(User{}))
    
    // Use OpenAPIRouter for automatic documentation
    espresso.OpenAPI(gen).
        Get("/api/users", getUsers, openapi.Tags("users")).
        Post("/api/users", createUser, openapi.Tags("users")).
        Get("/api/users/{id}", getUser, openapi.Tags("users")).
        ServeOpenAPI("/openapi.json").
        ServeDocs("/docs", "/openapi.json").
        Brew(espresso.WithAddr(":8080"))
}
```

### Manual Path Registration (Alternative)

If you need manual control over OpenAPI spec:

```go
gen := openapi.New("User API", "1.0.0").
    Description("API for managing users")

// Register schemas
gen.Schema("User", reflect.TypeOf(User{}))

// Manually define paths
gen.AddPath("GET", "/api/users", openapi.Operation{
    Summary: "List all users",
    Tags:    []string{"users"},
    Responses: map[string]openapi.Response{
        "200": {
            Description: "List of users",
            Content: map[string]openapi.MediaType{
                "application/json": {
                    Schema: &openapi.Schema{
                        Type:  "array",
                        Items: &openapi.Schema{Ref: "#/components/schemas/User"},
                    },
                },
            },
        },
    },
})

// Setup router separately
router := espresso.Portafilter()
router.Get("/users", getUsers)

// Serve spec separately (not recommended)
http.Handle("/openapi.json", gen.Handler())
http.Handle("/docs", openapi.ScalarUIHandler("/openapi.json"))
router.Brew(espresso.WithAddr(":8080"))
```

## See Also

- [Routing Guide](/guide/routing) - Route registration
- [Handlers Guide](/guide/handlers) - Handler patterns