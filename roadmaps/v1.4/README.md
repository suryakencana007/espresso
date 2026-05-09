# Espresso v1.4.0 — Hardening, Validation, and Comparable Numbers

This directory contains the roadmap and task specifications for Espresso v1.4.0, a **minor** release that focused on:

- closing TODOS #7 (struct-tag validator)
- closing TODOS #10 (head-to-head framework benchmarks)
- hardening streaming and WebSocket internals against races and leaks
- bringing the extractor error path into line with the handler error path (structured JSON)
- documenting and scoping known limitations (handler-cache growth, v2.0 roadmap)

This roadmap is recorded **retrospectively** — v1.4.0 shipped on 2026-04-20. Acceptance criteria are already met; the document exists for archival parity with `roadmaps/v1.3/`.

## Why v1.4?

v1.3 closed the foundation work for Barista: WebSocket, typed SSE, structured errors, graceful shutdown. What it left behind:

- **`Validation(any)`** in the service-layer surface, but no library-supplied validator implementation. Users had to bring their own.
- **No comparable performance numbers.** The README claimed "fast" without head-to-head data against Gin/Echo/Fiber.
- **A handful of streaming/WebSocket race conditions** caught only by `go test -race` after long-lived connection tests landed in v1.3.
- **Mismatched error shapes** — handler errors were structured JSON, extractor errors fell back to `http.Error()` text/plain.

v1.4 closes those gaps without breaking the v1.3 surface.

## Design Principles

Carried forward unchanged from v1.3:

1. **Consistency with existing patterns** — coffee metaphor, generics over `any`, `sync.Pool` for hot paths.
2. **Type-safety first** — leverage generics; `Validator[T]`-shaped APIs over `interface{}`.
3. **Zero-allocation when possible.**
4. **State injection compatibility** — new handler/extractor types must support `MustGetState[T]`.
5. **Context-first** on I/O paths.
6. **Middleware composability.**
7. **Backward compatible** — v1.4 is a minor bump. v1.3 callers must continue to compile.

The one wire-format change (extractor errors → structured JSON) is documented under **Migration Notes** in `CHANGELOG.md`; clients parsing 4xx as an error regardless of body shape are unaffected.

## Task Index

### 🔴 P0 — Features

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 1 | Struct-tag validator subpackage | [task-01-validator-subpackage.md](./tasks/task-01-validator-subpackage.md) | 3 days |
| 2 | Framework-comparison benchmark module | [task-02-bench-comparison-module.md](./tasks/task-02-bench-comparison-module.md) | 2 days |

### 🟡 P1 — Hardening

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 3 | Streaming concurrency hardening | [task-03-streaming-concurrency-hardening.md](./tasks/task-03-streaming-concurrency-hardening.md) | 2 days |
| 4 | Structured JSON error responses for extractor failures | [task-04-structured-extractor-errors.md](./tasks/task-04-structured-extractor-errors.md) | 1 day |

### 🔵 Verification Tasks

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 5 | Shutdown and state-injection coverage tests | [task-05-shutdown-and-state-tests.md](./tasks/task-05-shutdown-and-state-tests.md) | 1 day |

### ⚪ Cleanup

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 6 | Remove dead APIs (`Routes`, `closeErr`) | [task-06-remove-dead-apis.md](./tasks/task-06-remove-dead-apis.md) | 0.5 day |
| 7 | Lower go directive to 1.23 + gosec G115 fix | [task-07-go-directive-and-gosec.md](./tasks/task-07-go-directive-and-gosec.md) | 0.5 day |
| 8 | Handler-cache growth documentation | [task-08-handler-cache-docs.md](./tasks/task-08-handler-cache-docs.md) | 0.5 day |

### 📦 Meta Tasks

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 9 | v2.0 roadmap scaffolding | [task-09-v2-roadmap.md](./tasks/task-09-v2-roadmap.md) | 1 day |
| 10 | CHANGELOG & v1.4.0 release | [task-10-changelog-release.md](./tasks/task-10-changelog-release.md) | 0.5 day |

## Recommended Execution Order

See [EXECUTION_ORDER.md](./EXECUTION_ORDER.md) for the actual 2-week schedule.

## For AI Agents

See [AGENT_GUIDELINES.md](./AGENT_GUIDELINES.md). v1.3 rules carry over; this file lists what changed for v1.4.

## Project Information

- **Repository:** `github.com/suryakencana007/espresso`
- **Previous Version:** v1.3.0 (2026-04-19)
- **Target Version:** v1.4.0
- **Released:** 2026-04-20
- **Downstream Project:** Barista (self-hosted PaaS)

## Existing Package Layout (post-v1.3)

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
├── sse.go
├── state.go
├── websocket.go
├── withlayers.go
├── bench/                ← new in v1.4 (separate Go module)
├── extractor/
├── middleware/
│   ├── http/
│   └── service/
├── openapi/
├── pool/
├── tests/integration/
└── validator/            ← new in v1.4
```

## Global Conventions

- Unit tests with minimum 80% coverage on touched lines.
- `cmd/example/` is updated whenever user-facing API changes.
- All public APIs have godoc comments with examples.
- Benchmarks for performance-critical paths.
- `go test ./... -race` passes before a task is considered done.
- `golangci-lint run ./...` clean before a task is considered done.
