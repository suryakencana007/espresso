# Espresso v2.0.0 — Sharpen the Edges

This directory contains the roadmap and task specifications for Espresso v2.0.0, a **major** release that bundles the breaking changes we deliberately deferred through the v1.x line.

## Why v2.0?

v1.x shipped production-ready REST + streaming + OpenAPI on a strict backward-compatibility promise. That promise accumulated some baggage:

- **Global streaming registries** (`defaultRegistry`, `defaultSSERegistry`) that two `Portafilter()` instances in the same process silently share.
- **Multiple names for the same thing** (`NewBadRequest` vs `ErrBadRequest`, `ErrorResponse` alias, deprecated `SSEWriter` / `SSE` response).
- **Untyped extension points** (`espresso.Validation(validator any)`) that should be generic now that all callers are.
- **Unbounded per-process cache** in `handler.go` (`handlerCache sync.Map`) that works for static route tables but grows forever under dynamic handler registration.

v2.0 fixes these in one deliberate break so the v2.x line can carry clean internals forward without repeatedly nursing deprecations.

## Design Principles

Most v1.3 principles carry over unchanged. The one that changes for v2.0:

**Breaking changes are allowed, but each must be justified in its task spec and come with a migration recipe.** "Clean up for its own sake" is not a v2.0 scope; fixing a concrete footgun, removing a confirmed-unused API, or typing an untyped interface are.

All tasks must still:

1. Follow the coffee metaphor — `Portafilter`, `Ristretto`, `Doppio`, `Brew` — for any new surface.
2. Prefer Go generics over `any`.
3. Zero-allocation on hot paths; follow the `sync.Pool` pattern.
4. Support `espresso.MustGetState[T]` injection.
5. Accept `context.Context` on I/O-bound APIs.
6. Compose naturally with the middleware chain.

The v1.3 promise "code written for v1.2 must continue to work" is **lifted** for v2.0. Each breaking change is gated by an explicit task and paired with a migration-guide section.

## Task Index

### 🔴 P0 — Must Have in v2.0

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 1 | Per-Router stream registries | [task-01-per-router-registries.md](./tasks/task-01-per-router-registries.md) | 3-4 days |
| 2 | Remove deprecated APIs | [task-02-remove-deprecated-apis.md](./tasks/task-02-remove-deprecated-apis.md) | 1-2 days |

### 🟡 P1 — Should Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 3 | Handler-cache eviction + metrics hook | [task-03-handler-cache-eviction.md](./tasks/task-03-handler-cache-eviction.md) | 2 days |
| 4 | Typed `Validation[T]` layer | [task-04-typed-validation-layer.md](./tasks/task-04-typed-validation-layer.md) | 2 days |
| 5 | Optional auto-validate on extract | [task-05-auto-validate-on-extract.md](./tasks/task-05-auto-validate-on-extract.md) | 2-3 days |

### 📦 Meta

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 6 | v1→v2 migration guide | [task-06-migration-guide.md](./tasks/task-06-migration-guide.md) | 2 days |
| 7 | CHANGELOG, version bump, release | [task-07-changelog-release.md](./tasks/task-07-changelog-release.md) | 1 day |

## Out of Scope for v2.0

The following came up during v1.3 work but are intentionally **not** v2.0 material:

- **GraphQL / gRPC adapters** (TODOS #8, #9) — additive, ship in v2.x minor releases.
- **WebSocket connection pool/manager** (TODOS #5 carryover) — additive; a v2.x minor.
- **Multi-tier cache partitioning across handler types** — speculative, no user pain reported.
- **Dropping `net/http` for fasthttp** — too disruptive, does not match the framework's design posture.

## Recommended Execution Order

See [EXECUTION_ORDER.md](./EXECUTION_ORDER.md) for the recommended 3-week schedule.

## For AI Agents

See [AGENT_GUIDELINES.md](./AGENT_GUIDELINES.md) for rules specific to a major-version bump.

## Project Information

- **Repository:** `github.com/suryakencana007/espresso`
- **Current Version:** v1.3.0
- **Target Version:** v2.0.0
- **Module path change:** **Yes** — the module path becomes `github.com/suryakencana007/espresso/v2` per Go major-version conventions. Tooling and imports must update.

## Global Conventions

- All tasks require unit tests with minimum 80% coverage on touched lines.
- All tasks that change user-facing APIs must update or add a `cmd/example/` entry.
- All public APIs must have godoc comments with examples.
- Run `go test ./... -race` before considering a task complete.
- Run `golangci-lint run ./...` before considering a task complete.
- Every breaking change needs a matching "Migration" subsection in `docs/migration-v1-to-v2.md` (produced by Task 6).
