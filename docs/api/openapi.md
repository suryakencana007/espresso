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

### NewGenerator

Create a new OpenAPI generator:

```go
func NewGenerator(title, version string) *Generator
```

Example:

```go
gen := openapi.NewGenerator("My API", "1.0.0")
```

## Configuration

### SetDescription

Set API description:

```go
gen.SetDescription("REST API for my application")
```

### AddServer

Add server to the spec:

```go
gen.AddServer("http://localhost:8080", "Local development")
gen.AddServer("https://api.example.com", "Production")
```

## Building Specs

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

### AddSchema

Add schema to components:

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
gen := openapi.NewGenerator("My API", "1.0.0")
// ... add paths and schemas

http.Handle("/openapi.json", gen.Handler())
http.ListenAndServe(":8080", nil)
```

### ScalarUI

Serve Scalar UI:

```go
gen := openapi.NewGenerator("My API", "1.0.0")

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
    Components map[string]interface{} `json:"components,omitempty"`
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
    AdditionalProperties interface{}        `json:"additionalProperties,omitempty"`
    Example             interface{}         `json:"example,omitempty"`
    Ref                 string             `json:"$ref,omitempty"`
}
```

## Example

Complete example with Espresso:

```go
package main

import (
    "net/http"
    "reflect"
    
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/openapi"
)

type User struct {
    ID    int    `json:"id" doc:"User ID"`
    Name  string `json:"name" doc:"User name"`
    Email string `json:"email,omitempty" doc:"User email"`
}

func main() {
    // Create OpenAPI generator
    gen := openapi.NewGenerator("User API", "1.0.0")
    gen.SetDescription("API for managing users")
    gen.AddServer("http://localhost:8080", "Development")
    
    // Generate schema from type
    userSchema := openapi.GenerateSchemaFromType(reflect.TypeOf(User{}))
    gen.AddSchema("User", userSchema)
    
    // Add paths
    gen.AddPath("GET", "/users", openapi.Operation{
        Summary: "List all users",
        Tags:    []string{"users"},
        Responses: map[string]openapi.Response{
            "200": {
                Description: "List of users",
                Content: map[string]openapi.MediaType{
                    "application/json": {
                        Schema: &openapi.Schema{
                            Type: "array",
                            Items: &openapi.Schema{Ref: "#/components/schemas/User"},
                        },
                    },
                },
            },
        },
    })
    
    // Setup router
    router := espresso.Portafilter()
    router.Get("/users", getUsers)
    
// Serve OpenAPI spec
    http.Handle("/openapi.json", gen.Handler())
    
    // Serve Scalar UI
    http.Handle("/docs", openapi.ScalarUIHandler("/openapi.json"))
    
    // Serve API
    router.Brew(espresso.WithAddr(":8080"))
}

func getUsers() ([]User, error) {
    return []User{{ID: 1, Name: "John", Email: "john@example.com"}}, nil
}
```

## See Also

- [Routing Guide](/guide/routing) - Route registration
- [Handlers Guide](/guide/handlers) - Handler patterns