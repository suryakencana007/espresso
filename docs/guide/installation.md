# Installation

## Requirements

- **Go 1.22+** — Required for path parameter support (`r.PathValue()`)
- **CGO enabled** — Required for Sonic JSON library (`github.com/bytedance/sonic`)

## Install

```bash
go get github.com/suryakencana007/espresso
```

## Verify Installation

Create a simple `main.go`:

```go
package main

import "github.com/suryakencana007/espresso"

func main() {
    espresso.Portafilter().
        Get("/health", espresso.Ristretto(func() espresso.Text {
            return espresso.Text{Body: "OK"}
        })).
        Brew(espresso.WithAddr(":8080"))
}
```

Run:

```bash
go run main.go
```

Test:

```bash
curl http://localhost:8080/health
# Output: OK
```

## Import Paths

Espresso uses a modular package structure:

```go
import (
    // Core - handlers, router, server, response types
    "github.com/suryakencana007/espresso"
    
    // HTTP Middleware - CORS, rate limiting, compression, etc.
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
    
    // Service Layers - timeout, retry, circuit breaker, etc.
    servicemiddleware "github.com/suryakencana007/espresso/middleware/service"
    
    // Request Extractors - JSON, Query, Path, Header, Form, XML
    "github.com/suryakencana007/espresso/extractor"
    
    // Object Pooling - buffer pools for performance
    "github.com/suryakencana007/espresso/pool"
)
```

## Package Overview

| Package | Purpose | Key Types |
|---------|---------|-----------|
| `espresso` | Core framework | `Router`, `Handler`, `JSON`, `Text`, `Status` |
| `middleware/http` | HTTP-level middleware | `CORS`, `RateLimit`, `Compress`, `RequestID` |
| `middleware/service` | Service-level layers | `Timeout`, `Retry`, `CircuitBreaker`, `Validation` |
| `extractor` | Request extractors | `JSON`, `Query`, `Path`, `Header`, `Form`, `XML` |
| `pool` | Object pooling | `BufferPool`, `ByteSlicePool`, `StringSlicePool` |

## Dependencies

Espresso uses these production dependencies:

| Package | Purpose |
|---------|---------|
| `github.com/bytedance/sonic` | High-performance JSON |
| `github.com/rs/zerolog` | Structured logging |

## Go Modules

If you're using Go modules (recommended), your `go.mod` will include:

```go
module your-project

go 1.22

require (
    github.com/suryakencana007/espresso v1.0.2
    github.com/bytedance/sonic v1.11.0
    github.com/rs/zerolog v1.32.0
)
```

## Troubleshooting

### CGO Required

If you see an error about CGO:

```bash
# Enable CGO
export CGO_ENABLED=1

# Or for macOS
export CGO_ENABLED=1
```

### Go Version

```bash
# Check your Go version
go version

# Should be 1.22 or higher
```

## Next Steps

- [Quick Start](/guide/quick-start) — Build your first API
- [Core Concepts](/guide/core-concepts) — Understand the architecture
- [Examples](/examples/) — Real-world examples