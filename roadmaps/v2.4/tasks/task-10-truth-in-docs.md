# Task 10: Truth-in-Docs Sweep

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 2 days
**Dependencies:** None (but coordinate with Tasks 1-9 on any doc comments those tasks change)

> **Status: ✅ Shipped 2026-07-05 (v2.4.0).** Delivered via #63 + #72 + #73 — Middleware order (4 places), docs/api Solo/Doppio/GetState signatures, phantom WithServer/StateKey deleted, core.go pointer-arg examples, zero-alloc reword, false pooling comments removed, WSConfig.ReadTimeout no-op annotated.

## Context

The audit surfaced ~20 confirmed doc-drift findings across `README.md`, `docs/api/`, `docs/guide/`, `docs/examples/`, `core.go`, `extractor/doc.go`, and `CLAUDE.md`. Some are compile-breaking (copy-paste from docs won't build); some are behaviorally misleading (middleware order documented exactly backwards in four places); some are false marketing claims (the "zero-allocation handlers" line contradicted by measurement on every path).

This task is P1 — not P0 — but material: five of the six P0 code findings shipped in part because a doc comment covered for the code. `Generator` godoc claimed "-race verified"; `WSConfig.ReadTimeout` godoc claimed a timeout it never enforced; `TimeoutLayer` godoc did not mention retention hazards; `middleware/index.md` documented execution order backwards. Truth-in-docs is what keeps the audit from being re-run in 12 months to the same result.

The full inventory is in the audit report; the substantive drifts grouped by file are below.

## Acceptance Criteria

Each of the following changes lands, verified by grep or by compile:

**Marketing / performance claims**
- [x] `README.md:18`, `README.md:1147-1151`, `docs/index.md:126-128,144-148`, `docs/guide/index.md:11`, `core.go:3` all reworded from "zero-allocation handlers" to a defensible measurable claim about request-object pooling (`sync.Pool`-backed, zero request-struct allocations per request).
- [x] README's per-handler-shape performance table replaces "Zero per request" cells with the measured per-request numbers (e.g. `Doppio JSON: 8 allocs/op framework-side; Ristretto: 2 allocs/op`).
- [x] `docs/performance.md:133-134` refreshed to the current v2.1 refresh numbers or reduced to just the still-true ordering claim (beats Gin on JSON, trails Echo).

**Middleware execution order (four places, all inverted)**
- [x] `docs/guide/middleware/index.md:83-91` rewritten: first-registered = outermost = executes first.
- [x] `docs/examples/middleware-stack.md:69-73` diagram and prose rewritten.
- [x] `service.go:49-63` godoc rewritten.
- [x] `withlayers.go:391` comment rewritten.

**docs/api core signatures**
- [x] `docs/api/espresso.md`:
  - `Solo` signature changed from ctx-only to `func Solo[Req FromRequest, Res IntoResponse](fn func(Req) (Res, error))`.
  - `Doppio` signature includes the mandatory `(Res, error)` return.
  - `Ristretto` example returns `Text`, not bare `string`.
  - `WithServer` deleted (phantom); replaced with the six real `ServerOption` constructors (`WithAddr`, `WithReadTimeout`, `WithReadHeaderTimeout`, `WithWriteTimeout`, `WithIdleTimeout`, `WithShutdownTimeout`).
  - New "Server Lifecycle" section covers `BrewContext`, `OnShutdown`, `ShutdownHook`.
- [x] `docs/api/state.md`:
  - `GetState` return type corrected to `(T, bool)`.
  - Handler examples use `ok` pattern.
  - Phantom exported `StateKey` deleted.
  - 3-arg handler example rewritten to `espresso.Lungo` with `*State[T]`.
  - `FromState` and `FromMustState` documented (currently absent).
- [x] `docs/api/index.md` cross-references match (signatures for `Solo`/`Doppio`/`GetState`).

**Absent v2.3 features surfaced**
- [x] `docs/api/openapi.md` gets a Security section covering `AddSecurityScheme`, `SecurityScheme`, `BearerScheme`, `APIKeyHeaderScheme`, `openapi.Security`, `UnresolvedSecurityRefs`.
- [x] `docs/api/openapi.md` documents `openapi.Status` operation option and the `OpenAPIStatusCode` interface for non-200 status codes.
- [x] `docs/api/openapi.md` documents `ScalarVersion` pin.
- [x] `docs/api/validator.md` documents `AsDefaultValidator`.
- [x] `docs/api/middleware-service.md` includes `ValidatorFunc`, `ErrCircuitBreakerOpen`, `CircuitBreakerState`.

**core.go value-typed extractor examples panic at registration**
- [x] `core.go:32-57`: all doc examples show pointer-typed extractor args (`req *JSON[CreateUserReq]`, `req *extractor.Query[SearchReq]`, etc.).
- [x] `extractor/doc.go:13`: same fix.
- [x] `response.go:92`: same fix in the `JSON[T].Extract` doc example.

**Guide examples that fail to compile**
- [x] `docs/guide/middleware/service.md:144, 198-204`: rewrite Validation examples to use `WithLayersTyped` (Validation layer requires typed request; the inference path panics — audit `docs-guides#2`).
- [x] `docs/guide/middleware/service.md:219, 239, 256`: remove `servicemiddleware.ServiceFunc` (does not exist); define a local adapter or explicitly show the `Layer[Req, Res]` interface implementation.
- [x] `docs/guide/handlers.md:82` and `docs/guide/core-concepts.md:90`: replace `espresso.GetRequestID` with `httpmiddleware.GetRequestID` (real location).
- [x] `docs/guide/handlers.md:22`: "Since v1.6" → "Since v2.0" (v1.6 never existed).
- [x] `docs/guide/extractors.md:157, 391-397`: fix `espresso.RawBody` → `extractor.RawBody`; fix custom-extractor usage to pointer args and Doppio registration.
- [x] `docs/guide/routing.md:63-68`: delete the fake `*` wildcard claim; show only `{path...}` syntax with a working handler.
- [x] `docs/guide/state.md:103`: state extractor example unregistrable — rewrite via `Lungo`.
- [x] `docs/guide/testing.md:144`: fix the cross-package `_test.go` import pattern (currently references a symbol only visible to the test binary — audit `docs-guides#10`).
- [x] `docs/streaming.md:149`: `Send(name, data)` → `Send(Event{Name, Data})`.
- [x] `docs/guide/response.md:307-310`: fix redirect and error-handler statements per audit `docs-guides#12`.
- [x] `docs/guide/installation.md:6`: delete false "CGO required" claim (empirically not required; cross-compiles fine).

**Examples that break user code**
- [x] `docs/examples/basic-api.md:227` (and `production.md:353-399`): rewrite the custom-`APIError` with `WriteResponse` pattern (silently converts every error to 500) — use framework error constructors `ErrNotFound`, `ErrBadRequest`, etc.
- [x] `docs/examples/authentication.md:64, 157, 234, 423, 433`: replace `JSON[struct{}]` on GET routes with `func(ctx) (Res, error)` (Ristretto/reflection path); documented curl tests then actually succeed.
- [x] `docs/examples/authentication.md:357-388, 310-321`: rewrite dead sub-router examples (`protected := espresso.Portafilter()` unmounted → 404). Use single-router + per-route middleware, or the Skipper pattern.
- [x] `docs/examples/production.md:284-302`: rewrite `startServer` — `Brew` has no error return; do not run in a goroutine racing signal handling. Use plain `router.Brew(...)` or documented `BrewContext(ctx, ...)`.
- [x] `docs/examples/production.md:777-780`: remove stale claim that OpenAPI security is unsupported; show `AddSecurityScheme` + `openapi.Security` usage.
- [x] `docs/examples/state-management.md:40, 314, 67`: fix `GetState` return, delete phantom `StateKey`, rewrite 3-arg example via `Lungo`.
- [x] `docs/examples/basic-api.md:73, 339, 199`: fix default port (":8080", not ":3000"), fix unsynchronized-map data race in `UserHandler`.

**CLAUDE.md and README front-door**
- [x] `CLAUDE.md` alias table adds `LungoNoErr`.
- [x] `CLAUDE.md` Err* list adds `ErrPreconditionFailed`.
- [x] `README.md` mentions `Lungo` and `LungoNoErr` as the two-extractor entry points.

**False doc comments in code**
- [x] `response.go:71-74`: delete or rewrite the "Use pooled buffer for encoding" comment — pooled encoding does not happen there (only real if Task 6 wires it; coordinate).
- [x] `extractor/extractor.go:293, 299`: delete or rewrite the false `RawBody` pooling claims.
- [x] `websocket.go:83-86`: `WithWSReadTimeout` godoc marked as no-op with a `// TODO(v2.5): implement or delete` — the option is dead config (audit `streaming#4`); v2.4 documents it, v2.5 implements or deletes.
- [x] `sse.go:148`: (already fixed in #58; verify.)

## Technical Approach

The work is mechanical string-substitution and prose rewrites, not code refactoring. Order for review:

### Step 10.1 — Verify every drift on HEAD

Grep each cited line/text. Some may have been fixed since the audit (e.g. #58 landed `sse.go:148` and the ST1021 comment).

### Step 10.2 — Rewrite in file-group order

Do docs first (isolated per file), then godoc last (couples to code changes in Tasks 1-9). Recommended pass order:

1. `README.md` + `docs/index.md` + `docs/performance.md` — front-door claims.
2. `docs/api/*.md` — reference drift.
3. `docs/guide/**` — conceptual/behavioral claims.
4. `docs/examples/**` — user-facing examples.
5. `docs/streaming.md`, `docs/error-handling.md`, `docs/README.md` — root pages.
6. `core.go`, `extractor/doc.go`, `response.go`, `service.go`, `withlayers.go`, `websocket.go` — godoc.
7. `CLAUDE.md` — alias table + Err* list.

### Step 10.3 — Extend the snippet-compile guard

Task 11 owns hardening the snippet-compile test. Task 10's job is to make sure that after the rewrites, the self-contained fences the guard already compiles are green. If the rewrite of `docs/examples/basic-api.md` moves it into "compilable" territory, the guard will exercise it — verify no new failures.

### Step 10.4 — Coordinate with code tasks

If Task 6 lands after Task 10, its regression tests may catch a `response.go:71-74` comment mismatch — coordinate the merge order so the false comment is either removed by Task 10 first, or rewritten to describe the new (wired) pooled behavior by Task 6.

## Tests Required

Not a code task, but:

- Run the existing `docs_snippet_compile_test.go` after each pass; every previously-passing fence must still pass.
- Grep verifications: after the pass, `grep "espresso\.Path\[" docs/` should be zero; `grep "zero-allocation handlers" .` should be zero; `grep "Send(name" docs/streaming.md` should be zero; etc.
- Manual: build a small program from the corrected `docs/api/espresso.md` `Solo`/`Doppio`/`Ristretto` examples — must compile.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `go test -race ./... -count=2` clean (no code changes; only godoc).
- [x] `docs_snippet_compile_test.go` clean.
- [x] `golangci-lint run ./...` clean (misspell, godot etc. don't regress).
- [x] CI green on the PR.
- [x] CHANGELOG `[Unreleased]` entry under `Changed` (docs): comprehensive truth-in-docs sweep — corrected middleware execution order (4 places), corrected core-page API signatures (`Solo`/`Doppio`/`GetState`), surfaced v2.3 OpenAPI security/status/`ScalarVersion` APIs, fixed value-typed extractor examples in `core.go` and `extractor/doc.go`, deleted false pooling comments, reworded "zero-allocation handlers" to the defensible pooled-request claim.
- [x] No code behavior change.
