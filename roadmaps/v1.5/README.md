# Espresso v1.5.0 — Close the Barista Boilerplate

This directory contains the roadmap and task specifications for Espresso v1.5.0, a **minor** release that closes three concrete friction items surfaced by Barista (the flagship downstream app) across the v0.3-v0.6 milestones.

## Why v1.5?

`roadmaps/USAGE_ESPRESSO.md` records what Barista has stress-tested and where the framework forced workarounds. As of v0.6 (2026-05-09), four friction items remain open:

- **F-05** — `JSON[T]` can't set cookies on the response side. Refresh-token rollout had to write a 16-line `httpx.JSONWithCookies[T]` `IntoResponse` wrapper.
- **F-06** — No `RawBodyWithHeaders[H]` extractor. Webhook receivers re-derive a 35-line custom extractor for "raw body + provider header" (two such handlers in v0.4).
- **F-07** — No `ErrPreconditionFailed` (412) helper. Canary handlers fall back to `NewError(http.StatusPreconditionFailed, ...)`.
- **F-01, F-02** — deeper API-shape questions (Ristretto-cannot-reach-state, Stream-commits-headers-early). These require breaking changes or non-trivial restructuring; both are deferred to v2.0 (see `roadmaps/v2.0/`).

v1.5 closes F-05, F-06, F-07. All three are **additive** — no v1.4 caller is affected, no wire-format changes, no removed symbols.

## Design Principles

Carried forward unchanged from v1.4 (which carried forward unchanged from v1.3):

1. **Consistency with existing patterns** — coffee metaphor, generics over `any`, `sync.Pool` for hot paths.
2. **Type-safety first** — `RawBodyWithHeaders[H]` follows the same shape as `Path[T]`, `Header[T]`.
3. **Zero-allocation where possible.**
4. **State injection compatibility** — new extractors must work alongside `MustGetState[T]`.
5. **Context-first** on I/O paths.
6. **Middleware composability.**
7. **Backward compatible** — v1.5 is a minor bump. Every v1.4 caller must continue to compile and behave identically.

## Task Index

### 🔴 P0 — Features

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 1 | Cookies on `JSON[T]` response | [task-01-json-response-cookies.md](./tasks/task-01-json-response-cookies.md) | 0.5 day |
| 2 | `extractor.RawBodyWithHeaders[H]` | [task-02-rawbody-with-headers-extractor.md](./tasks/task-02-rawbody-with-headers-extractor.md) | 1 day |
| 3 | `ErrPreconditionFailed` (412) | [task-03-precondition-failed-error.md](./tasks/task-03-precondition-failed-error.md) | 0.25 day |

### 📦 Meta Tasks

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 4 | CHANGELOG & v1.5.0 release | [task-04-changelog-release.md](./tasks/task-04-changelog-release.md) | 0.25 day |

## Recommended Execution Order

See [EXECUTION_ORDER.md](./EXECUTION_ORDER.md) for the recommended 1-week schedule.

## For AI Agents

See [AGENT_GUIDELINES.md](./AGENT_GUIDELINES.md). v1.3 and v1.4 rules carry over; this file lists what changed for v1.5.

## Project Information

- **Repository:** `github.com/suryakencana007/espresso`
- **Previous Version:** v1.4.0 (2026-04-20)
- **Target Version:** v1.5.0
- **Downstream Project:** Barista (self-hosted PaaS — see `roadmaps/USAGE_ESPRESSO.md`)
- **Friction items closed:** F-05, F-06, F-07
- **Friction items deferred to v2.0:** F-01, F-02

## Existing Package Layout (post-v1.4)

```
espresso/
├── core.go
├── error.go              ← Task 3 touches this
├── handler.go
├── response.go           ← Task 1 touches this
├── router.go
├── server.go
├── sse.go
├── state.go
├── websocket.go
├── withlayers.go
├── bench/
├── extractor/            ← Task 2 adds RawBodyWithHeaders here
├── middleware/
│   ├── http/
│   └── service/
├── openapi/
├── pool/
├── tests/integration/
└── validator/
```

## Out of Scope for v1.5

Explicitly **not** in this release:

- **F-01** (`Ristretto` can't reach state) — fix requires either a deprecation or a signature variant; both belong in v2.0 alongside the other deprecated-API removals.
- **F-02** (`Stream` commits headers early) — fix requires restructuring `serveStream`'s pre-flight phase; v2.0 Task 1 (per-Router registries) is already restructuring that path, so the fix should land there.
- **F-03** (handler-signature sprawl) — flagged not-urgent in USAGE_ESPRESSO.md; the W-03 interface-seam pattern contains the churn.
- **F-08** (test-seam pattern guidance) — application-layer concern, not a framework change.

## Global Conventions

- Unit tests with minimum 80% coverage on touched lines.
- `cmd/example/` updated when user-facing API changes.
- All public APIs have godoc comments with examples.
- `go test ./... -race` clean before a task is considered done.
- `golangci-lint run ./...` clean before a task is considered done.
- Docs site (`docs/api/`) updated alongside each feature task.
