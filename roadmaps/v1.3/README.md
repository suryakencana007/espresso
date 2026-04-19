# Espresso v1.3.0 — Foundation for Barista

This directory contains the roadmap and task specifications for Espresso v1.3.0, a minor release focused on preparing the framework to power **Barista**, a self-hosted Kubernetes-based PaaS that will serve as Espresso's flagship application.

## Why v1.3?

Espresso v1.2 provides solid foundations for REST APIs with type-safe extractors, middleware, and service layers. However, Barista requires several capabilities that v1.2 doesn't yet provide first-class support for:

- **WebSocket** support for web terminal (browser → Kubernetes pod exec)
- **Typed SSE streaming** for live logs, build progress, and deployment status
- **Structured error responses** for consistent API error handling
- **Graceful shutdown hooks** for clean termination of long-lived connections

These capabilities are the focus of v1.3. Each feature has been selected based on concrete requirements from the Barista project.

## Design Principles

All tasks must follow these principles:

1. **Consistency with existing patterns** — Follow the "coffee" naming convention (`Portafilter`, `Ristretto`, `Doppio`, `Brew`). New features must fit this metaphor.
2. **Type-safety first** — Leverage Go generics as seen in `JSON[T]`, `Query[T]`. Avoid `interface{}` unless absolutely necessary.
3. **Zero-allocation when possible** — Follow the `sync.Pool` pattern already established in the handler root.
4. **State injection compatibility** — All new handler types must support `espresso.GetState[T]` and `espresso.MustGetState[T]`.
5. **Context-first** — All long-running operations must accept `context.Context` and respect cancellation.
6. **Middleware composability** — New features must work seamlessly with the existing middleware chain.
7. **Backward compatible** — v1.3 is a minor version bump. Code written for v1.2 must continue to work.

## Task Index

### 🔴 P0 — Must Have Before Barista

These tasks block Barista development. They must be completed and released before Barista's first sprint.

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 1 | WebSocket Handler Support | [task-01-websocket.md](./tasks/task-01-websocket.md) | 5-7 days |
| 2 | Typed Streaming Response (SSE) | [task-02-sse-streaming.md](./tasks/task-02-sse-streaming.md) | 3-5 days |

### 🟡 P1 — Should Have

These tasks should be completed alongside P0 tasks. They are not strictly blockers, but will cause friction in Barista development if missing.

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 3 | Structured Error Response | [task-03-structured-errors.md](./tasks/task-03-structured-errors.md) | 2-3 days |
| 4 | Graceful Shutdown Hooks | [task-04-graceful-shutdown.md](./tasks/task-04-graceful-shutdown.md) | 2-3 days |

### 🔵 Verification Tasks

These are testing and validation tasks. No new features — they verify existing and new features behave correctly under real-world conditions.

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 5 | Long-Lived Connection Stress Test | [task-05-longlived-test.md](./tasks/task-05-longlived-test.md) | 2 days |
| 6 | Context Cancellation Propagation Test | [task-06-context-cancellation-test.md](./tasks/task-06-context-cancellation-test.md) | 1 day |
| 7 | Streaming Upload Memory Test | [task-07-upload-memory-test.md](./tasks/task-07-upload-memory-test.md) | 1 day |

### 📦 Meta Tasks

Release engineering and documentation tasks.

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 8 | CHANGELOG & Version Bump | [task-08-changelog-release.md](./tasks/task-08-changelog-release.md) | 0.5 day |
| 9 | Documentation Update | [task-09-documentation.md](./tasks/task-09-documentation.md) | 2 days |
| 10 | Blog Post Draft | [task-10-blog-post.md](./tasks/task-10-blog-post.md) | 1 day |

## Recommended Execution Order

See [EXECUTION_ORDER.md](./EXECUTION_ORDER.md) for the recommended 3-week schedule.

## For AI Agents

See [AGENT_GUIDELINES.md](./AGENT_GUIDELINES.md) for rules and conventions that apply to all tasks.

## Project Information

- **Repository:** `github.com/suryakencana007/espresso`
- **Current Version:** v1.2.0
- **Target Version:** v1.3.0
- **Downstream Project:** Barista (self-hosted PaaS, using Espresso as HTTP framework)

## Existing Package Layout

```
espresso/
├── context.go
├── core.go
├── error.go
├── handler.go
├── http.go
├── layerconfig.go
├── layerstack.go
├── response.go
├── router.go
├── router_layers.go
├── router_openapi.go
├── server.go
├── service.go
├── state.go
├── withlayers.go
├── extractor/
├── middleware/
│   ├── http/
│   └── service/
├── openapi/
└── pool/
```

## Global Conventions

- All tasks require unit tests with minimum 80% coverage
- All tasks must add or update `cmd/example/` if user-facing API changes
- All public APIs must have godoc comments with examples
- Benchmarks are required for performance-critical paths
- Run `go test ./... -race` before considering a task complete
- Run `golangci-lint run ./...` before considering a task complete
