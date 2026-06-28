# Espresso v2.3.0 — Backflush

This directory contains the roadmap and task specifications for Espresso v2.3.0, a **minor** release that is a correctness/quality cleanup with **one tiny additive API** (security-scheme registration) and otherwise **no new feature surface**. Backflushing an espresso machine runs clean water back through the group head to clear built-up coffee residue — v2.3 is that cycle for Espresso: flush out the accumulated correctness debt the v2.2 analysis surfaced and deferred, so the generated artifacts and docs can be trusted again.

## Why v2.3?

A **2026-06-28** verify-and-scope pass took the items the v2.1/v2.2 analyses had flagged-and-deferred and empirically confirmed them against real code — specs were generated and inspected, the WebSocket failure was reproduced and root-caused. Everything below is confirmed, not suspected.

The headline is making the **OpenAPI generator trustworthy**. Today it emits specs that are quietly wrong: ten distinct defects were confirmed by generating a real spec and inspecting it.

- **The spec misrepresents the routes it documents.** Custom-`FromRequest` introspection is dead code (the interface declares `Extract(r any) error` but every real extractor uses `Extract(*http.Request) error`), so custom extractors are never introspected. `extractStatusCode` always returns `0`, so a `201` POST documents as `200`. Extractor classification is by type-name **prefix** string-match, so a user type named `Files...` or `Format...` silently mis-classifies with zero compile signal. `registerPath` and `RegisterHandler` are copy-paste twins that drifted — only one attaches the response-body schema, and `registerPath` silently swallows introspection errors so the route just vanishes from the spec.
- **`components.securitySchemes` is allocated empty and never populated**, while `Security("bearerAuth")` references a scheme by name — producing a dangling reference that fails strict OpenAPI validation and breaks the Scalar/Swagger auth button. v2.3 adds the one small additive API of the release to register security schemes.
- **The serving path is hardened.** The spec-generation failure path emits `text/plain` via `http.Error` instead of the canonical JSON envelope; the spec is re-marshaled on every request despite being immutable after registration; the Scalar UI loads `@scalar/api-reference` with no version pin from an external host; and `AutoRegister` is an empty no-op stub whose godoc promises it registers all routes. The no-op `AutoRegister` is **deleted** so the API stops lying.

Plus two small cleanups, both confirmed:

- **A docs type-reference sweep.** `espresso.Path/Query/Form/Header/XML[T]` are written wrong throughout `docs/` — those types live in the `extractor` package and are not re-exported by root. A mechanical `espresso.X` → `extractor.X` fix, plus a docs-snippet-compile check so self-contained example fences are actually built.
- **A one-line WebSocket integration-test fix.** `TestLongLived_WS_StableConnection` pings in a loop but never reads, and `coder/websocket` only processes pong frames inside the read path — so `Ping` times out. This is a test-harness issue, **not a framework bug** (the server-side read loop is healthy); the fix is `conn.CloseRead(ctx)` after the dial.

No new feature surface beyond the security-scheme registration API. Every finding was confirmed against real code on 2026-06-28.

## Design Principles

Carried forward unchanged from v2.2:

1. **Coffee metaphor** for any public surface.
2. **Type-safety first** — generics over `any`.
3. **Zero-allocation hot paths** — performance-conscious; atomic over mutex on hot paths.
4. **State injection via `MustGetState[T]`** stays the canonical pattern.
5. **Context-first** on I/O.
6. **Structured errors** — `*espresso.Error` and the canonical JSON envelope are the contract.

The v2.0 charter still applies: the backward-compat flip holds — v2.x may make breaking or behavior changes, each justified and documented.

New emphasis for v2.3:

7. **Trustworthy generated artifacts.** The OpenAPI spec must accurately reflect the routes. When a generator's output disagrees with reality, fix the generator and lock it with a spec-inspection test — never paper over it in docs.

## Task Index

### 🔴 P0 — Must Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 1 | OpenAPI generation correctness | [task-01-openapi-generation-correctness.md](./tasks/task-01-openapi-generation-correctness.md) | 3 days |
| 2 | OpenAPI security schemes | [task-02-openapi-security-schemes.md](./tasks/task-02-openapi-security-schemes.md) | 1.5 days |

### 🟡 P1 — Should Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 3 | OpenAPI serving hardening + remove AutoRegister stub | [task-03-openapi-serving-and-autoregister.md](./tasks/task-03-openapi-serving-and-autoregister.md) | 1.5 days |
| 4 | Docs type-reference sweep + snippet-compile check | [task-04-docs-extractor-sweep.md](./tasks/task-04-docs-extractor-sweep.md) | 1 day |
| 5 | Fix WebSocket long-lived integration test | [task-05-ws-integration-test-fix.md](./tasks/task-05-ws-integration-test-fix.md) | 0.5 day |

### 🔵 Verification

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 6 | OpenAPI spec-correctness matrix + suites green | [task-06-verification.md](./tasks/task-06-verification.md) | 1 day |

### 📦 Meta

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 7 | CHANGELOG + v2.3.0 release | [task-07-changelog-release.md](./tasks/task-07-changelog-release.md) | 0.5 day |

## Recommended Execution Order

See [EXECUTION_ORDER.md](./EXECUTION_ORDER.md). Tasks 1, 2, 4, and 5 touch disjoint files and run in parallel; the serving hardening (Task 3) lands after Tasks 1 and 2 because it shares `openapi.go` and `router_openapi.go`; verification and the release follow.

## For AI Agents

See [AGENT_GUIDELINES.md](./AGENT_GUIDELINES.md). v1.3, v1.4, v1.5, v2.0, v2.1, v2.2 rules carry over; this file lists only what changed for v2.3.

## Project Information

- **Repository:** `github.com/suryakencana007/espresso/v2`
- **Previous Version:** v2.2.0 (2026-06-28, "Dial It In")
- **Target Version:** v2.3.0
- **Module path:** unchanged (`/v2`; v2.3 is a minor bump)
- **Downstream Project:** Barista

## Existing Package Layout (post-v2.2)

v2.3 adds **no new packages or directories** — it edits existing files only.

```
espresso/
├── context.go
├── core.go
├── error.go
├── handler.go
├── handler_cache.go
├── http.go
├── layerconfig.go
├── layerstack.go
├── response.go
├── router.go
├── router_layers.go
├── router_openapi.go          ← Tasks 1, 3 edit registerPath / RegisterHandler / delete AutoRegister
├── server.go
├── service.go
├── sse.go
├── state.go
├── validate.go
├── websocket.go
├── withlayers.go
├── bench/
├── cmd/example/
│   ├── sse/
│   ├── validate/
│   └── websocket/
├── docs/                      ← Task 4 sweeps espresso.X → extractor.X + snippet-compile check
│   ├── migration-v1-to-v2.md
│   └── migration-v2-to-v2.1.md
├── extractor/
├── internal/
│   ├── errorenvelope/         ← reused by Task 3 for the OpenAPI failure path (stdlib-only leaf, no import cycle)
│   └── validatehook/
├── middleware/
│   ├── http/
│   └── service/
├── openapi/                   ← Tasks 1, 2, 3 edit introspect.go / openapi.go / options.go / scalar.go
├── pool/
├── tests/integration/         ← Task 5 fixes the WS long-lived test (CloseRead after dial)
└── validator/
```

## Related, Deferred

Items the 2026-06-28 verify-scope pass touched on that v2.3 deliberately does **not** pick up, so the scope boundary is explicit:

- **Real OpenAPI auto-registration.** `AutoRegister` is deleted in v2.3 as a no-op stub whose godoc lied; a genuine route-walking auto-registration that introspects the live router is a **possible future feature**, not v2.3 scope.
- **The WebSocket finding is not a framework bug.** `TestLongLived_WS_StableConnection` failed because the test client never read (so `coder/websocket` never processed pongs); the espresso WS server is healthy. v2.3 fixes the test only — no framework code change.

## Global Conventions

Same as v2.2:

- Unit tests with minimum 80% coverage on touched lines.
- `cmd/example/` updated when user-facing behavior changes.
- All public APIs have godoc comments with examples; when this release changes behavior, godoc is corrected in the same PR.
- `go test ./... -race` clean before a task is considered done; the integration suite (`go test -tags=integration ./tests/integration/...`) must pass on this machine after Task 5.
- `golangci-lint run ./...` clean before a task is considered done.
- Behavior changes are locked with a regression test (for the OpenAPI work, a spec-inspection test) and called out in the CHANGELOG `Changed`/`Fixed` section with before/after.
