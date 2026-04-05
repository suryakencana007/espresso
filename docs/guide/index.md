# Introduction

<script setup>
import { VPIcon } from 'vitepress/theme'
</script>

<div style="text-align: center; margin-bottom: 2rem;">
  <img src="/logo.svg" alt="Espresso Logo" style="width: 200px; height: 200px; margin: 0 auto;" />
</div>

**Espresso** is a production-grade HTTP routing framework for Go, inspired by [Axum](https://github.com/tokio-rs/axum) (Rust) and [Tower](https://github.com/tower-rs/tower) (Rust).

Like a perfectly pulled espresso shot, this framework delivers:

- **Fast** — Zero-allocation handlers with sync.Pool for request objects
- **Strong** — Production-ready with battle-tested patterns
- **Pure** — No magic, just clean Go code with explicit types
- **Aromatic** — Rich type-safe extractors without manual implementation

## Why Espresso?

### Type-Safe from the Start

```go
func handler(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[UserRes], error) {
    user := req.Data  // Type-safe access, automatically decoded
    return espresso.JSON[UserRes]{Data: user}, nil
}
```

### Coffee-Themed API

The API uses coffee terminology to make routing intuitive:

| Term | Purpose |
|------|---------|
| **Portafilter** | Creates the router (`espresso.Portafilter()`) |
| **Ristretto** | 0-param handler (simplest, like a restricted shot) |
| **Solo** | 1-param handler (single shot) |
| **Doppio** | 2-param handler (double shot, full power) |
| **Brew** | Starts the server |

### Modular Architecture

```
espresso/
├── core              # Handlers, Router, Server
├── extractor/        # JSON, Query, Path, Header, Form, XML
├── middleware/
│   ├── http/         # RequestID, CORS, RateLimit, Compress
│   └── service/      # Timeout, Retry, CircuitBreaker, Logging
└── pool/             # Buffer pools for zero-allocation
```

## Quick Example

```go
package main

import (
    "context"
    "github.com/suryakencana007/espresso"
    httpmiddleware "github.com/suryakencana007/espresso/middleware/http"
)

type CreateUserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    espresso.Portafilter().
        Use(httpmiddleware.RequestIDMiddleware()).
        Use(httpmiddleware.RecoverMiddleware()).
        
        // Simple health check
        Get("/health", espresso.Ristretto(func() espresso.Text {
            return espresso.Text{Body: "OK"}
        })).
        
        // JSON body extraction with full context
        Post("/users", espresso.Doppio(createUser)).
        
        // Start server
        Brew(espresso.WithAddr(":8080"))
}

func createUser(ctx context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[User], error) {
    return espresso.JSON[User]{
        StatusCode: http.StatusCreated,
        Data:       User{Name: req.Data.Name},
    }, nil
}
```

## Architecture Overview

<div class="mermaid-wrapper">

```mermaid
graph TB
    subgraph "HTTP Layer"
        Request[HTTP Request]
        MW[Middleware Use]
        Extract[Extractor]
    end
    
    subgraph "Service Layer"
        Layers[WithLayers]
        Handler[Handler]
    end
    
    subgraph "Response"
        Response[IntoResponse]
    end
    
    Request --> MW
    MW --> Extract
    Extract --> Layers
    Layers --> Handler
    Handler --> Response
```

</div>

## Next Steps

<div class="vp-grid" style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1rem; margin: 1rem 0;">
  <div class="vp-card" style="padding: 1.5rem; border: 1px solid var(--vp-c-divider); border-radius: 8px;">
    <h3>🚀 Getting Started</h3>
    <p>Install Espresso and create your first API endpoint.</p>
    <a href="/guide/installation">Start here →</a>
  </div>
  
  <div class="vp-card" style="padding: 1.5rem; border: 1px solid var(--vp-c-divider); border-radius: 8px;">
    <h3>📚 Core Concepts</h3>
    <p>Learn about handlers, extractors, and middleware.</p>
    <a href="/guide/core-concepts">Learn more →</a>
  </div>
  
  <div class="vp-card" style="padding: 1.5rem; border: 1px solid var(--vp-c-divider); border-radius: 8px;">
    <h3>📖 Examples</h3>
    <p>Real-world examples and best practices.</p>
    <a href="/examples/">View examples →</a>
  </div>
</div>

## Community

- [GitHub](https://github.com/suryakencana007/espresso) - Report issues and contribute
- [License](https://github.com/suryakencana007/espresso/blob/main/LICENSE) - MIT License