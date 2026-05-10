# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Espresso is a production-grade, Axum/Tower-inspired HTTP routing framework for Go (`github.com/suryakencana007/espresso/v2`). Module declares `go 1.23`; `mise.toml` pins the dev toolchain to Go 1.25.6 + `gh latest`. Path-parameter routing depends on Go 1.22+ `ServeMux`. JSON encoding uses `bytedance/sonic`; logging uses `rs/zerolog`; WebSocket uses `coder/websocket`.

## Multi-module layout

The repo is **two Go modules**:

- Root module — the framework itself.
- `bench/` — a separate module (so Gin/Echo/Fiber comparison deps don't leak into the framework). It has its own `go.mod` with a `replace` back to the parent.

`go test ./...` from the root does NOT cover `bench/`. Run `bench/` benchmarks from inside that directory.

## Common commands

```bash
# Unit tests (root module only)
go test ./...
go test ./... -cover
go test -race ./...

# Single test / package
go test -run TestName ./...
go test ./middleware/http -run TestCORS

# Micro-benchmarks (framework internals)
go test -bench=. -benchmem -benchtime=3s ./...

# Framework comparison benchmarks (separate module — must cd)
cd bench && go test -bench . -benchmem -benchtime=3s -count=1

# Long-lived integration tests (gated by build tag; default 30s, set timeout for 1-hour runs)
go test -tags=integration ./tests/integration/...
go test -tags=integration ./tests/integration/... -timeout=2h

# Lint (golangci-lint v2 config in .golangci.yml; gocyclo min 15)
golangci-lint run ./...
golangci-lint run --fix ./...

# Run example server
go run ./cmd/example

# Docs site (VitePress; bun-based — see package.json)
bun install
bun run docs:dev      # local preview at default port
bun run docs:build    # CI build (workflows/docs.yml)
```

## Architecture

### Two-level middleware

This is the load-bearing distinction in the codebase, and the mistake to avoid is conflating the levels.

- **HTTP middleware** (`router.Use(...)`, `middleware/http/`) — `func(http.Handler) http.Handler`. Runs on raw `http.Request` **before** extraction. CORS, auth, rate-limit, recover, request-id, compress, logging.
- **Service layers** (`middleware/service/`, `WithLayers`/`WithLayersTyped`) — `Layer[Req,Res] = func(Service[Req,Res]) Service[Req,Res]`. Runs on the typed `(ctx, *Req) → (Res, error)` boundary **after** extraction. Timeout, retry, circuit breaker, validation, metrics.

`LayerConfig` (`layerconfig.go`) is the type-erased form (`Timeout(d)`, `Logging(l,name)`, etc.) so the same layer set can be reused across handlers with different `Req`/`Res`. Concrete `Layer[Req,Res]` is materialized at `WithLayers` binding time.

### Handler dispatch

Two paths into the same destination — `http.HandlerFunc`:

1. **Reflection path** — `Handler(any)` / `router.Handle(any)` / `router.Get/Post/...(path, fn)`. Inspects the function via reflection at registration time, caches results in `handlerCache` (a process-global `sync.Map` keyed by `reflect.Type`). Supports `func() T`, `func(*Req) T`, `func(ctx, *Req) (T, error)`, `func(ctx, *Req1, *Req2) (T, error)`, and `Service[Req,Res]`. Panics at registration (not request) for invalid signatures.
2. **Typed path** — generic `HandlerCtxReqErr[Req,Res]`, `HandlerCtxReq`, `HandlerReqErr`, `HandlerReq`, `HandlerCtxReq1Req2Err`, etc. Skip the reflection cache; use these for hot paths or when avoiding the cache matters.

`handlerCache` has **no eviction** — fine for static route registration at startup, but applications that register dynamically-generated function types per request will accumulate entries forever. Eviction is on the v2.0 roadmap (`roadmaps/v2.0/tasks/task-03-handler-cache-eviction.md`). The doc comment on `handlerCache` in `handler.go` is canonical.

### Coffee-themed aliases

These are the user-facing entry points; all wrap the underlying typed handlers:

| Alias | Signature | Underlying |
|-------|-----------|------------|
| `Portafilter()` | constructs `Router` | `&Router{mux: http.NewServeMux()}` |
| `Ristretto(f)` | `func(context.Context) T` | `HandlerCtxNoErr` |
| `Solo(f)` | `func(*Req) (T, error)` | `HandlerReqErr` |
| `Doppio(f)` | `func(ctx, *Req) (T, error)` | `HandlerCtxReqErr` |
| `Lungo(f)` | `func(ctx, *Req1, *Req2) (T, error)` | `HandlerCtxReq1Req2Err` |
| `Brew(opts...)` | starts server + graceful shutdown | `(*Router).Brew` |

### Extraction & response contracts

- **`FromRequest`** (`core.go`) — extractors. **Must use a POINTER receiver** for `Extract(*http.Request) error`; value receivers silently no-op because they mutate a copy. Extractors: `JSON[T]` (root + `extractor`), `extractor.{Query,Path,Header,Form,Cookie,Multipart,File,Files,XML,RawBody}`. Custom extractors implement `Extract` directly.
- **`Resettable`** (`core.go`) — optional. If implemented, the handler uses it to reset pooled request structs; otherwise it falls back to reflection-based zeroing. Implement `Reset()` on hot paths.
- **`IntoResponse`** (`response.go`) — responses. Built-ins: `JSON[T]`, `Text`, `Status`. Custom responses set their own status/headers/body in `WriteResponse`.

### State injection

State is **immutable, context-carried**. Three usage patterns:

1. `MustGetState[T](ctx)` / `GetState[T](ctx)` — context-based (most idiomatic).
2. `State[T]` extractor — Axum-style explicit handler parameter.
3. `FromState[S,T](ctx, getter)` — substate pattern.

`router.WithState(s)` prepends `WithStateMiddleware(s)` to the middleware chain so it runs before user middleware. There is **no built-in synchronization for mutable state** — use `atomic.*`, `sync.Map`, or `sync.RWMutex` inside the state struct yourself.

### Long-lived connections (SSE & WebSocket)

Two **process-global registries** (`defaultSSERegistry`, `defaultRegistry`) track active streams. They exist so `gracefulShutdown` (`server.go`) can close all SSE streams and send WebSocket close frames (code 1001) before `http.Server.Shutdown`. Per-Router registries are on the v2.0 roadmap (`task-01-per-router-registries.md`).

Shutdown sequence (`server.go:gracefulShutdown`):
1. User-registered `OnShutdown` hooks run in order (panic-recovered, logged).
2. `defaultSSERegistry.closeAll(...)` — final comment frame.
3. `defaultRegistry.closeAll(CloseGoingAway, ...)` — close code 1001.
4. `srv.Shutdown(ctx)` — stop accepting + drain in-flight requests.

`Brew()` blocks on SIGINT/SIGTERM/SIGQUIT; `BrewContext(ctx, opts...)` is the programmatic variant for tests/embedding.

### Sub-packages at a glance

| Package | Purpose |
|---------|---------|
| `extractor/` | Built-in `FromRequest` implementations. |
| `middleware/http/` | HTTP-level middleware + auth (JWT, BasicAuth, APIKey). |
| `middleware/service/` | Service layers (Timeout, Retry, CircuitBreaker, Validation, Logging, Metrics). |
| `pool/` | `sync.Pool`-backed `BufferPool`, `ByteSlicePool`, `StringSlicePool`. |
| `openapi/` | OpenAPI 3.0 generator with handler introspection + Scalar UI. |
| `validator/` | Struct-tag validator (`required`, `min`, `max`, `email`, `url`, `regex`, `oneof`). Returns `espresso.FieldErrors`. |

### Error pipeline

`*espresso.Error` (`error.go`) is the framework's structured error. Constructors: `ErrBadRequest`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrConflict`, `ErrUnprocessableEntity`, `ErrTooManyRequests`, `ErrInternal`, `ErrServiceUnavailable`. Extractor failures, service-call failures, SSE setup failures, and WebSocket upgrade failures all produce the same JSON shape (`{"error":{"code":...,"message":...,"details":...,"request_id":...}}`) — there is a regression-locking test (`TestWithLayers_ExtractorErrorReturnsStructuredJSON`) for this; do not regress it back to `http.Error()` text/plain.

## Conventions

- **No breaking changes on v1.x.** The v2.0 roadmap (`roadmaps/v2.0/`) is where breakages are scoped — do not preempt them. `roadmaps/v2.0/AGENT_GUIDELINES.md` governs v2 work specifically.
- **Pointer receivers for `Extract`.** Always. Value receivers silently fail.
- **Atomic over mutex on hot paths.** Existing examples: `WS.closed` is `atomic.Bool` because two handler-end guards previously read a plain bool without the mutex (data race caught by `-race`).
- **CHANGELOG.md follows Keep-a-Changelog.** Per-version entries describe Added/Changed/Fixed.

## Files most likely to matter when extending

- New extractor → `extractor/extractor.go` + tests + register-style doc note in `core.go` if it's central.
- New HTTP middleware → `middleware/http/middleware.go` (or `auth.go` for auth flavors) + tests.
- New service layer → `middleware/service/layer.go`, then a `LayerConfig` constructor in `layerconfig.go` so it composes via `WithLayers`.
- New response type → implement `IntoResponse` in `response.go` (or co-locate with the feature, e.g. SSE in `sse.go`).
- New handler shape → add a typed `Handler*[Req,Res]` entry in `handler.go` AND wire the reflection path in `withlayers.go`'s `inferTypes`/`createTypedHandler`.

## Roadmap conventions

Per-version roadmaps live under `roadmaps/<version>/` and follow a fixed shape established in `roadmaps/v1.3/`. When asked to scaffold a new roadmap or back-fill one for an already-shipped version, mirror the existing layout — do not invent a new one.

### Directory layout

```
roadmaps/<version>/
├── README.md           # why this release, principles, task index, project info
├── AGENT_GUIDELINES.md # rules for AI agents on this release
├── EXECUTION_ORDER.md  # week-by-week schedule, contingency planning
└── tasks/
    ├── task-01-<slug>.md
    ├── task-02-<slug>.md
    └── …
```

### README structure (in this order)

1. `# Espresso vX.Y.Z — <tagline>` heading.
2. **Why vX.Y?** — concrete debt or capability the release closes; bullets cite v(prior) gaps.
3. **Design Principles** — carry forward from prior version unchanged unless a principle flips (e.g. v2.0 lifts strict backward-compat). Call out the flip explicitly.
4. **Task Index** — grouped tables by priority tier, columns: `# | Task | File | Estimated Effort`. Tiers and emoji are fixed:
   - 🔴 **P0** — must ship.
   - 🟡 **P1** — should ship.
   - 🔵 **Verification** — tests/coverage tasks, no new features.
   - ⚪ **Cleanup** — dead-code removal, doc-only fixes (used in v1.4; not always present).
   - 📦 **Meta** — CHANGELOG, release tagging, migration guide.
5. **Recommended Execution Order** — single-line pointer to `EXECUTION_ORDER.md`.
6. **For AI Agents** — single-line pointer to `AGENT_GUIDELINES.md`.
7. **Project Information** — repo, prior version, target version, downstream project (Barista when relevant).
8. **Existing Package Layout** — tree snapshot at the start of the release, marking new directories with `← new in vX.Y` comments.
9. **Global Conventions** — bullets: test coverage minimum, `cmd/example/` updates, godoc requirements, `-race` and lint gates.
10. (Major versions only) **Out of Scope** — items deliberately deferred, with reasoning.

### AGENT_GUIDELINES structure

- **First version** (v1.3 baseline) is full text covering: read-before-write, coffee metaphor, type-safety, context-mandatory, testing requirements, commit conventions, small commits, backward compat, performance, error philosophy, workflow, PR template, files-not-to-touch, common mistakes.
- **Subsequent versions** (v1.4, v2.0, …) are **delta-only** — explicitly say "Read this **in addition to** `roadmaps/v(prior)/AGENT_GUIDELINES.md`. This file only lists what is **different** for vX.Y." Then list 3-5 deltas. Do not restate inherited rules.

### EXECUTION_ORDER structure

- **Overview** — fenced text block sketching the week-by-week shape.
- **Week N — `<theme>`** sections — daily breakdown with task references (`Task X, Step X.Y: …`). "Weekend" entries are flexible overflow.
- **Contingency Planning** — four buckets:
  - Must Not Slip (hard requirements for the release).
  - Can Slip to vX.Y.Z+1 (next patch).
  - Can Slip to vX.Y+1 (next minor) / vX+1.0 (next major).
  - Must Not Compromise (quality gates: `-race`, lint, coverage, backward compat).
- **Parallel Work Opportunities** — batches of tasks with no cross-file conflicts.
- **Notes** — buffer-time guidance, review-cycle assumptions.

### Task file structure (every task)

```markdown
# Task N: <Title>

**Priority:** <emoji> <tier — feature|hardening|verification|cleanup|meta>
**Estimated Effort:** <X days>
**Dependencies:** <prior task numbers, or "None">

## Context
<Why this task exists. Cite the TODO number, prior-version test that exposed it, or
upstream constraint. 2-4 paragraphs max.>

## Acceptance Criteria
- [ ] <Concrete, checkable item.>
- [ ] <…>

## Technical Approach
### Step N.1: <name>
<Code snippets, file paths, decisions to make.>

### Step N.2: <name>
<…>

## Tests Required
- <Specific test cases, not "good coverage".>

## Definition of Done
- [ ] <Quality gate.>
- [ ] <…>
```

### Forward-looking vs retrospective

- **Forward-looking** roadmap (release not yet shipped, e.g. `v2.0/` today): leave acceptance and DoD checkboxes empty (`- [ ]`).
- **Retrospective** roadmap (release already shipped, e.g. `v1.4/` documenting v1.4.0): pre-check the boxes (`- [x]`), state the shipped date in the README and Task 10, and write `EXECUTION_ORDER.md` in past tense ("This document records the actual execution order").

### Mapping CHANGELOG to tasks (retrospective)

When back-filling a roadmap from a shipped CHANGELOG entry, group entries into 8-10 tasks. Bundling rules:

- One feature → one task. (validator/, bench/ each got their own task.)
- Multiple `Changed`/`Fixed` items targeting the same subsystem → one hardening task. (v1.4 Task 3 bundled four WS/SSE concurrency fixes.)
- All `Removed` items → one cleanup task unless they're independent enough to split.
- The CHANGELOG/release/tag work is always the final task.
- The migration guide (major versions only) is its own meta task before the release task.
