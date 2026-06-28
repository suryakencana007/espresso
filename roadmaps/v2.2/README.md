# Espresso v2.2.0 — Dial It In

> **Status: shipped 2026-06-28.** Tag [`v2.2.0`](https://github.com/suryakencana007/espresso/releases/tag/v2.2.0).
> A correctness pass — no new feature surface; upgrade notes are in the CHANGELOG `[2.2.0]` section.
> This directory is retained for historical reference; future work has its own roadmap.

This directory contains the roadmap and task specifications for Espresso v2.2.0, a **minor** release that ships **no new feature surface**. Dialing in an espresso shot means tuning grind, dose, and time until the pull is actually correct — v2.2 is that pass over Espresso itself: make behavior match contract, lock it with tests, ship nothing new.

## Why v2.2?

A post-v2.1 codebase analysis on **2026-06-27** empirically confirmed several places where Espresso's documented or expected behavior does not actually hold. v2.2 closes the gap between contract and code — correctness only:

- **Reflection-path two-extractor handlers panic at request time.** `CLAUDE.md` and the `Handler` godoc claimed the reflection dispatch path (`Handler(any)` via `router.Get/Post/Handle`) supports `func(ctx, *Req1, *Req2) (T, error)`, but `handlerInfo` carries a single request slot — the registration-time arg loop overwrites it on the second `FromRequest` arg, so registration succeeds silently and then `createHandlerFromInfo` hits `panic("espresso: invalid handler argument - this is a bug")` per request (500 under `RecoverMiddleware`). The typed `HandlerCtxReq1Req2Err` / `Lungo` path works correctly. (PR #39 already removed the false doc claim; this finishes the job in code.)
- **Service-layer errors collapse to HTTP 500.** `writeHandlerError` only special-cases `*espresso.Error`; everything else is wrapped as `ErrInternal`. Errors surfaced through service layers are not `*espresso.Error`, so `ValidationLayer` (`ErrValidation{}`), an open `CircuitBreaker` (`*CircuitBreakerError`), and `TimeoutLayer` (`context.DeadlineExceeded`) all return 500 — proven with a runnable reproduction. The mapping infrastructure already exists but is unwired.
- **Two error paths bypass the structured-JSON envelope.** `RecoverMiddleware` emits a hand-rolled anonymous-struct JSON that omits `details` (code `PANIC`), and auth middleware (JWT/BasicAuth/APIKey) plus the rate limiter emit `http.Error` `text/plain` (401/429) — diverging from the canonical `{"error":{"code","message","details","request_id"}}` shape that `TestWithLayers_ExtractorErrorReturnsStructuredJSON` locks for the extractor/handler/SSE/WS paths. This finding was surfaced by the analysis and must be confirmed-then-fixed; it is cycle-sensitive (root → `httpmiddleware`, never the reverse), which is precisely why `RecoverMiddleware` hand-rolls its JSON today.

No new features. Two findings were proven with runnable reproductions; the third (error-envelope consistency) is confirmed-then-fixed.

## Design Principles

Carried forward unchanged from v2.1:

1. **Coffee metaphor** for any public surface.
2. **Type-safety first** — generics over `any`.
3. **Zero-allocation hot paths** — performance-conscious; atomic over mutex on hot paths.
4. **State injection via `MustGetState[T]`** stays the canonical pattern.
5. **Context-first** on I/O.
6. **Structured errors** — `*espresso.Error` and the canonical JSON envelope are the contract.

The v2.0 charter still applies: the backward-compat flip holds — v2.x may make breaking or behavior changes, each justified and documented.

New emphasis for v2.2:

7. **Behavior matches contract.** When docs/godoc and code disagree, fix whichever is wrong and lock it with a test. Correctness over new surface.

## Task Index

### 🔴 P0 — Must Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 1 | Reflection-path two-extractor handlers | [task-01-reflection-two-extractor.md](./tasks/task-01-reflection-two-extractor.md) | 2 days |
| 2 | Service-layer error → HTTP status mapping | [task-02-service-layer-error-status.md](./tasks/task-02-service-layer-error-status.md) | 2 days |

### 🟡 P1 — Should Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 3 | Structured-JSON envelope on every error path | [task-03-error-envelope-consistency.md](./tasks/task-03-error-envelope-consistency.md) | 1.5 days |

### 🔵 Verification

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 4 | Status-code matrix + signature + doc/code consistency tests | [task-04-verification.md](./tasks/task-04-verification.md) | 1 day |

### 📦 Meta

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 5 | CHANGELOG + v2.2.0 release | [task-05-changelog-release.md](./tasks/task-05-changelog-release.md) | 0.5 day |

## Recommended Execution Order

See [EXECUTION_ORDER.md](./EXECUTION_ORDER.md). The two P0 correctness fixes are independent and run in parallel; the envelope unification (P1) and verification follow.

## For AI Agents

See [AGENT_GUIDELINES.md](./AGENT_GUIDELINES.md). v1.3, v1.4, v1.5, v2.0, v2.1 rules carry over; this file lists only what changed for v2.2.

## Project Information

- **Repository:** `github.com/suryakencana007/espresso/v2`
- **Previous Version:** v2.1.0 (2026-05-12, "Finish the Pull")
- **Target Version:** v2.2.0
- **Module path:** unchanged (`/v2`; v2.2 is a minor bump)
- **Downstream Project:** Barista

## Existing Package Layout (post-v2.1)

v2.2 adds **no new packages or directories** — it edits existing files only.

```
espresso/
├── context.go
├── core.go
├── error.go                  ← Tasks 2, 3 edit writeHandlerError / envelope writer
├── handler.go                ← Task 1 edits the reflection dispatch path
├── handler_cache.go
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
├── validate.go
├── websocket.go
├── withlayers.go             ← Task 2 updates TestWithLayersTyped_WithTimeout assertion
├── bench/
├── cmd/example/
│   ├── sse/
│   ├── validate/
│   └── websocket/
├── docs/
│   ├── migration-v1-to-v2.md
│   └── migration-v2-to-v2.1.md
├── extractor/
├── internal/validatehook/    ← cycle-break precedent for Task 3's shared envelope leaf
├── middleware/
│   ├── http/                 ← Task 3 edits RecoverMiddleware + auth + rate limiter
│   └── service/              ← Task 2 maps Validation/CircuitBreaker/Timeout errors
├── openapi/
├── pool/
├── tests/integration/
└── validator/
```

## Related, Deferred

Other items the 2026-06-27 analysis surfaced that v2.2 deliberately does **not** pick up, so the scope boundary is explicit:

- **OpenAPI generator defects** — introspection gaps and schema-emission bugs in `openapi/`. Their own correctness pass; not bundled into v2.2's error/handler scope.
- **Per-Router handler-cache config** — the cache is still package-global (carried over from the v2.1 "Out of Scope" list). Defer until a concrete user surfaces the need.
- **Encoder `sonic`/stdlib split** — making the JSON encoder pluggable/selectable. Additive surface, not a correctness gap; future minor.

## Global Conventions

Same as v2.1:

- Unit tests with minimum 80% coverage on touched lines.
- `cmd/example/` updated when user-facing behavior changes.
- All public APIs have godoc comments with examples; when this release changes behavior, godoc is corrected in the same PR.
- `go test ./... -race` clean before a task is considered done.
- `golangci-lint run ./...` clean before a task is considered done.
- Behavior changes are locked with a regression test and called out in the CHANGELOG `Changed` section with before/after.
