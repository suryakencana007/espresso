# Examples

Welcome to the Espresso examples gallery. These examples demonstrate common patterns and best practices.

## Quick Links

| Example | Description |
|---------|-------------|
| [Basic REST API](/examples/basic-api) | Simple CRUD API with handlers and extractors |
| [File Upload](/examples/file-upload) | Handle file uploads with Multipart extractor |
| [SSE Streaming](/examples/sse-streaming) | Real-time updates with Server-Sent Events |
| [Authentication](/examples/authentication) | JWT, Basic Auth, and API Key authentication |
| [Middleware Stack](/examples/middleware-stack) | Complete middleware configuration for production |
| [State Management](/examples/state-management) | Dependency injection with application state |
| [Production Setup](/examples/production) | Full production-ready configuration (includes OpenAPI docs) |

## Getting Started

All examples assume you have Go 1.22+ installed and have initialized a Go module:

```bash
go mod init myapp
go get github.com/suryakencana007/espresso
```

## Project Structure

A typical Espresso project:

```
myapp/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── handlers/
│   │   └── user.go
│   ├── models/
│   │   └── user.go
│   ├── services/
│   │   └── user.go
│   └── config/
│       └── config.go
├── pkg/
│   └── middleware/
│       └── auth.go
├── go.mod
└── go.sum
```

## Minimal Example

```go
package main

import "github.com/suryakencana007/espresso"

func main() {
    router := espresso.Portafilter()
    
    router.Get("/health", func() string {
        return "OK"
    })
    
    router.Brew()
}
```

## Next Steps

- [Basic REST API](/examples/basic-api) - Learn handlers and extractors
- [Middleware Stack](/examples/middleware-stack) - Configure HTTP and service middleware
- [State Management](/examples/state-management) - Share state across handlers
- [Production Setup](/examples/production) - Full production configuration with OpenAPI docs

## API Documentation

All examples include optional OpenAPI documentation support:

- **OpenAPI 3.0 Spec**: Auto-generated from your handlers
- **Scalar UI**: Modern, beautiful API documentation
- See [Production Setup](/examples/production#openapi-documentation-optional) for full example