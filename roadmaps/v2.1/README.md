# Espresso v2.1.0 — Finish the Pull

This directory contains the roadmap and task specifications for Espresso v2.1.0, a **minor** release that finishes the work v2.0 deliberately deferred and ships one bounded ergonomics improvement.

## Why v2.1?

v2.0 ([shipped 2026-05-10](https://github.com/suryakencana007/espresso/releases/tag/v2.0.0)) was scoped tight on purpose — get the breaking changes out, ship clean, don't bundle additive cleanup that would have padded the release. Two items that legitimately belonged in v2.0 slipped to v2.1:

- **Deprecated SSE types** (`SSE`, `SSEEvent`, `SSEWriter`, `NewSSEWriter`) — task-02 was scoped to remove them, but the removal had no functional payoff in v2.0 itself, just a search-and-replace tax on downstream apps. Skipping kept v2.0 release timing tight.
- **`Stream` pre-flight phase (Barista F-02)** — let SSE handlers return `*espresso.Error` before headers commit, instead of forcing per-route preflight middleware. v2.0's per-Router-registries work (task-01) restructured `serveStream`; F-02 lands cleanly on top of that.

Plus one ergonomics nick that turned up while writing the v2.0 migration guide:

- The `cmd/example/validate/` opt-in wiring requires users to wrap `validator.Struct` in a closure that converts `FieldErrors` → `ValidationErrors`. That's the same closure every user will write. Ship a helper.

## Design Principles

Carried forward unchanged from v2.0:

1. **Coffee metaphor** for any new public surface.
2. **Type-safety first** — generics over `any`.
3. **Zero-allocation hot paths** — the auto-validate nil-fast load is the bar.
4. **State injection via `MustGetState[T]`** stays the canonical pattern.
5. **Context-first** on I/O.
6. **Migration recipes accompany breaking changes** — the v1→v2 guide pattern carries forward.

The v2.0 charter ("breaking changes are allowed, each must be justified") still applies. v2.1 has two breaking entries; each one is debt v2.0 explicitly deferred, not new scope.

## Task Index

### 🔴 P0 — Must Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 1 | Remove deprecated SSE types | [task-01-remove-deprecated-sse.md](./tasks/task-01-remove-deprecated-sse.md) | 1 day |
| 2 | `Stream` pre-flight phase (closes Barista F-02) | [task-02-stream-preflight.md](./tasks/task-02-stream-preflight.md) | 2-3 days |

### 🟡 P1 — Should Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 3 | `validator.AsDefaultValidator()` adapter | [task-03-validator-default-adapter.md](./tasks/task-03-validator-default-adapter.md) | 0.5 day |

### 🔵 Verification

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 4 | Bench refresh vs Gin/Echo/Fiber on v2.x | [task-04-bench-refresh.md](./tasks/task-04-bench-refresh.md) | 0.5 day |

### 📦 Meta

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 5 | Migration-guide v2.0 → v2.1 update | [task-05-migration-update.md](./tasks/task-05-migration-update.md) | 0.5 day |
| 6 | CHANGELOG & v2.1.0 release | [task-06-changelog-release.md](./tasks/task-06-changelog-release.md) | 0.5 day |

## Recommended Execution Order

See [EXECUTION_ORDER.md](./EXECUTION_ORDER.md). Schedule fits in one focused week.

## For AI Agents

See [AGENT_GUIDELINES.md](./AGENT_GUIDELINES.md). v1.3, v1.4, v1.5, v2.0 rules carry over; this file lists what changed for v2.1.

## Project Information

- **Repository:** `github.com/suryakencana007/espresso/v2`
- **Previous Version:** v2.0.0 (2026-05-10)
- **Target Version:** v2.1.0
- **Module path:** unchanged (`/v2` from v2.0; v2.1 is a minor bump)
- **Downstream Project:** Barista (closes F-02)

## Existing Package Layout (post-v2.0)

```
espresso/
├── core.go
├── error.go
├── handler.go
├── handler_cache.go
├── http.go
├── layerconfig.go
├── layerstack.go
├── response.go              ← Task 1 removes deprecated SSE/SSEWriter/SSEEvent
├── router.go
├── server.go
├── service.go
├── sse.go                   ← Task 2 modifies Stream pre-flight phase
├── state.go
├── validate.go
├── websocket.go
├── withlayers.go
├── bench/                   ← Task 4 re-runs against v2.x
├── cmd/example/
│   └── validate/            ← Task 3 simplifies the opt-in wiring
├── docs/
│   ├── migration-v1-to-v2.md
│   └── migration-v2-to-v2.1.md   ← Task 5 creates this
├── extractor/
├── internal/validatehook/
├── middleware/{http,service}/
├── openapi/
├── pool/
├── tests/integration/
└── validator/               ← Task 3 adds AsDefaultValidator()
```

## Out of Scope for v2.1

Items raised during v2.0 retrospective that v2.1 deliberately does NOT pick up:

- **Per-Router handler-cache config** — the cache is still package-global. A per-Router move was floated in `roadmaps/v2.0/tasks/task-03` but explicitly scoped out. Defer to v2.2 unless a concrete user surfaces the need.
- **`SetDefaultValidator` becoming `Router.WithDefaultValidator`** — same shape; same defer reason. The package-level setter is fine until someone wants per-Router differentiation.
- **GraphQL / gRPC adapters** (TODOS #8, #9 from v1.x) — additive minor features; their own future minor-version slot.
- **WebSocket connection pool / manager** (v1.3 carryover) — additive; defer.
- **Auth-gate pattern docs** — `USAGE_ESPRESSO.md` W-10/W-12 noted three deployments of `RequireXxx` middleware in Barista. Could extract a `docs/guide/auth-gates.md`. Defer to a docs-only PR; not blocking v2.1.

## Global Conventions

Same as v2.0:

- Unit tests with minimum 80% coverage on touched lines.
- `cmd/example/` updated when user-facing API changes.
- All public APIs have godoc comments with examples.
- `go test ./... -race` clean before a task is considered done.
- `golangci-lint run ./...` clean before a task is considered done.
- Migration recipes accompany every breaking change in the same PR.
