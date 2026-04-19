# Task 9: Documentation Update

**Priority:** 📦 Meta
**Estimated Effort:** 2 days
**Dependencies:** Tasks 1, 2, 3, 4 must be complete (so APIs are stable)

## Context

Espresso v1.3 adds several significant features. Users need clear documentation to adopt them. This task ensures the README and deep-dive documentation are complete, accurate, and discoverable.

Good documentation for a framework serves two audiences:

1. **Newcomers evaluating the framework** — they read the README first
2. **Existing users adopting new features** — they search for specific guides

We serve both with a layered approach: README has overview + quick start for all features, with links to deeper per-feature guides.

## Acceptance Criteria

- [ ] README has new sections: "WebSocket", "Streaming (SSE)", "Error Handling", "Graceful Shutdown"
- [ ] Existing sections updated to mention new features where relevant
- [ ] `docs/websocket.md` — deep-dive on WebSocket usage and patterns
- [ ] `docs/streaming.md` — deep-dive on SSE usage and patterns
- [ ] `docs/error-handling.md` — deep-dive on structured errors
- [ ] `docs/performance.md` — long-lived connection behavior, benchmarks
- [ ] All examples in documentation are runnable as-is
- [ ] All new public APIs have godoc comments with examples
- [ ] Documentation linked from the main README navigation

## Technical Approach

### Step 9.1: Update Main README

The README should maintain its current structure but add new top-level sections. Here's the proposed structure:

```
# Espresso ☕
> Production-grade HTTP routing framework for Go

## Why Espresso?
## Table of Contents
## Installation
## Quick Start
## Package Structure
## Core Concepts
## Routing
## Handlers & Coffee-Themed Aliases
## Axum-Style Extractors
## Response Types
## Middleware
## Service Layers
## Object Pooling

--- NEW SECTIONS ---
## WebSocket                 (link to docs/websocket.md)
## Streaming (SSE)          (link to docs/streaming.md)
## Error Handling           (link to docs/error-handling.md)
## Graceful Shutdown        (this section, inline)

## State Management (Dependency Injection)
## OpenAPI Documentation
## Complete Example
## Benchmarks
## API Reference
## Contributing
```

Each new section in the README should:

- Start with a one-sentence purpose
- Show a minimal working example (copy-pasteable)
- Link to the deep-dive doc for more details

Example for the WebSocket section:

````markdown
## WebSocket

Espresso supports WebSocket handlers with the same type-safety and state
injection as JSON handlers.

```go
type Req struct {
    Room string `path:"room"`
}

func chatHandler(ctx context.Context, req *extractor.Path[Req], ws *espresso.WS) error {
    for {
        _, msg, err := ws.Read(ctx)
        if err != nil {
            return nil
        }
        if err := ws.WriteText(ctx, "Echo: "+string(msg)); err != nil {
            return err
        }
    }
}

router.Get("/ws/{room}", espresso.WebSocket(chatHandler))
```

**Features:**

- Text and binary frames
- Automatic ping/pong keepalive
- State injection via `MustGetState[T]`
- Context cancellation on disconnect
- Graceful close on server shutdown

See [docs/websocket.md](./docs/websocket.md) for more details, including:

- Advanced configuration options
- Concurrency patterns
- Error handling
- Authentication patterns
- Testing WebSocket handlers
````

### Step 9.2: Create docs/websocket.md

Structure:

```markdown
# WebSocket Handlers

## Overview

## Basic Usage

### Simple WebSocket (No Extractor)
### WebSocket with Path Extractor
### WebSocket with Query Extractor

## Configuration Options

### Ping/Pong Keepalive
### Message Size Limits
### Subprotocols
### Origin Patterns (CORS)
### Read/Write Timeouts

## State Injection

## Concurrency Patterns

### Reader and Writer Goroutines
### Fan-out to Multiple Clients

## Error Handling

### Handler Errors
### Network Errors
### Graceful Disconnects

## Authentication

### JWT over WebSocket
### Initial HTTP Auth + WebSocket Upgrade

## Testing WebSocket Handlers

### Unit Tests with httptest
### Client Connection Patterns

## Performance Considerations

### Avoiding Allocation in Hot Paths
### Backpressure for Slow Clients

## Troubleshooting

### 426 Upgrade Required
### Connection Closes Unexpectedly
### Ping Not Working Through Proxy

## Advanced Topics

### Custom Subprotocols
### Compression (permessage-deflate)
### Migration from gorilla/websocket
```

Each section should have code examples that can be copied into a working project.

### Step 9.3: Create docs/streaming.md

Structure:

```markdown
# Server-Sent Events (SSE)

## Overview

## Basic Usage

### Simple Stream
### Stream with Extractor
### JSON Events

## Configuration Options

### Keep-Alive
### Initial Retry Hint
### Last-Event-ID

## Event Format

## Concurrency

### Safe Concurrent Sends
### Streaming from Multiple Sources

## Reconnection Handling

### Last-Event-ID Pattern
### Client-Side EventSource

## Integration Patterns

### Kubernetes Pod Logs
### Database Change Streams
### Broadcast to Multiple Clients

## Testing Stream Handlers

## Performance Considerations

### Slow Clients
### Memory Usage for Buffered Streams

## SSE vs WebSocket

## Troubleshooting

### Events Not Received
### Buffering Issues Behind Proxies
### Browser Connection Limits
```

### Step 9.4: Create docs/error-handling.md

Structure:

```markdown
# Error Handling

## Overview

## Returning Structured Errors

### Using Constructors
### Custom Error Codes
### Adding Details
### Wrapping Internal Errors

## Error Response Format

## Standard HTTP Status Codes

## Error Codes in Barista (Case Study)

## Error Logging

### Internal vs External Errors
### Request ID Integration

## Panic Recovery

## Testing Error Paths

## Migration from Plain Errors

## Best Practices

### Error Code Naming Convention
### When to Use Details
### Sensitive Information in Errors
```

### Step 9.5: Create docs/performance.md

This was partially covered in Task 5, but this task expands it:

```markdown
# Performance Characteristics

## Overview

## Benchmarks

### Handler Throughput
### Memory Allocations
### Zero-Allocation Paths

## Long-Lived Connections

### SSE Memory Profile
### WebSocket Memory Profile
### Concurrent Connection Limits

## Large Payloads

### Upload Streaming
### Download Streaming

## Context Cancellation

### Propagation Latency
### Goroutine Cleanup

## Tuning

### GOMAXPROCS
### sync.Pool Usage
### HTTP Server Settings

## Profiling Espresso Apps

### CPU Profiling
### Memory Profiling
### Trace Analysis

## Known Limitations

## Comparison to Other Frameworks

(Optional: benchmark comparisons to Gin, Echo, Chi — fair and objective)
```

### Step 9.6: Godoc Comments

Every public API must have a godoc comment. Run this to find missing ones:

```bash
# Check for exported symbols without godoc
go vet ./... 2>&1 | grep "exported"

# Or use revive
revive -config revive.toml ./...
```

Requirements:

- Every type, function, method, and constant must have a comment
- Comments start with the name of the symbol
- Comments explain **what** the symbol does, and **when/why** to use it
- Include runnable examples for complex APIs using `Example` functions:

```go
func ExampleWebSocket() {
    handler := func(ctx context.Context, req *extractor.Path[Req], ws *espresso.WS) error {
        return ws.WriteText(ctx, "hello")
    }

    router := espresso.Portafilter().
        Get("/ws/{room}", espresso.WebSocket(handler))

    _ = router
    // Output:
}
```

### Step 9.7: Example Folder README

Each example folder should have its own README.md explaining:

- What the example demonstrates
- How to run it
- How to test it (curl commands, wscat commands)
- Links to relevant documentation

Template:

```markdown
# [Feature Name] Example

Demonstrates [feature] in Espresso.

## What This Shows

- Feature 1
- Feature 2
- Feature 3

## Run

    go run ./cmd/example/[name]

## Test

### With [tool]:

    [command]

## See Also

- [Feature documentation](../../docs/[feature].md)
- [API reference](https://pkg.go.dev/github.com/suryakencana007/espresso)
```

### Step 9.8: Cross-Linking

Every new document should link to:

- The main README
- Related documents (WebSocket ↔ Streaming, etc.)
- API reference on pkg.go.dev
- Example code in `cmd/example/`

Main README's "Table of Contents" should include links to all new docs.

### Step 9.9: Verify Examples Compile and Run

Every code block in documentation must be valid Go. Create a script to extract and compile them:

```bash
# scripts/verify-docs.sh
#!/usr/bin/env bash
set -euo pipefail

# Extract Go code blocks from markdown and try to compile
# (This is a simple version; a real implementation might use goldmark)

echo "Verifying README examples..."
# ... extraction and compilation logic ...
```

At minimum, manually test each example by copy-pasting into a scratch project.

## Tests Required

This task has no unit tests, but has these quality gates:

- [ ] All code examples in documentation compile (manual verification)
- [ ] All links in documentation work (can use a link checker)
- [ ] godoc renders correctly on pkg.go.dev (check after v1.3.0 tag)
- [ ] Markdown renders correctly on GitHub

## Link Checking Tool

Run this to find broken links:

```bash
# Install markdown-link-check (requires Node.js)
npm install -g markdown-link-check

# Check all markdown files
find . -name "*.md" -not -path "./node_modules/*" \
    -exec markdown-link-check {} \;
```

## Definition of Done

- [ ] All sections of README updated or added
- [ ] `docs/websocket.md` complete with all subsections
- [ ] `docs/streaming.md` complete with all subsections
- [ ] `docs/error-handling.md` complete with all subsections
- [ ] `docs/performance.md` complete with benchmarks and findings
- [ ] Every example folder has its own README.md
- [ ] All public APIs have godoc comments
- [ ] Runnable examples (`ExampleXxx` functions) for complex APIs
- [ ] No broken links in documentation
- [ ] All code examples compile and run correctly
- [ ] Main README's TOC links to all new documents
- [ ] `CHANGELOG.md` entry for documentation improvements

## Style Guide for Documentation

Keep documentation:

- **Action-oriented** — start with "How to..." or "When to..."
- **Example-heavy** — show don't tell
- **Honest** — note limitations and trade-offs
- **Linked** — cross-reference related content
- **Maintained** — update when API changes

Avoid:

- Marketing language ("blazing fast", "next-generation")
- Promises that may not hold ("always", "never")
- Vague claims without numbers ("fast enough")
- Unexplained jargon

## Tips for AI Agents Writing Documentation

1. Look at the existing README for tone and style — match it
2. Use the same formatting conventions (code blocks, tables, bullet lists)
3. When in doubt, err toward more examples rather than more prose
4. Every feature should have a "when to use" discussion, not just "how to use"
5. Link liberally — help readers find related information
6. Include troubleshooting sections for common issues
