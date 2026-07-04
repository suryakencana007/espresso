# Espresso v2.4.0 — Ground Truth

This directory contains the roadmap and task specifications for Espresso v2.4.0, a **minor** release that is a correctness/quality cleanup with **no new feature surface** other than a JSON-body limit hook. Ground Truth is the release where the framework's behavior catches up to what its docs, comments, and marketing claim — the audit run of 2026-07-02 through 2026-07-04 surfaced ~30 confirmed defects between shipped behavior and shipped contracts, and v2.4 closes the P0/P1 ones.

## Why v2.4?

A **2026-07-02/03/04** 10-dimension alignment audit (114 findings, 221 agents, adversarially verified where budget allowed) took the framework as it shipped in v2.3.0 and compared it against its own docs, its own error contract, its own zero-allocation claims, and its own graceful-shutdown promises. Six correctness defects were reproduced under `-race` or live servers and are the P0 spine of this release. Four more high-severity defects that break behavior on well-formed user code make the P1 tail. Everything below is confirmed, not suspected.

The headline is **behavior matches contract on the paths users actually take**. Today the framework:

- **Races its own pool under Timeout.** `TimeoutLayer` (`middleware/service/layer.go:50`) spawns a goroutine and returns on `ctx.Done()`, but `applyLayersAndConvert` (`withlayers.go:421`) unconditionally resets and re-pools the request struct on the deferred exit. When a timeout fires, the abandoned goroutine keeps reading `*Req` while the next request `Extract`s into it — a `-race`-reproduced data race in the flagship service layer, on the exact scenario Timeout exists for.
- **Starves under sustained sub-second traffic.** `TokenBucketLimiter` (`middleware/http/middleware.go:306`) computes `refill := int(elapsed.Seconds()) * l.rate / int(time.Second.Seconds())` and unconditionally advances `lastRefill = now`. Under ≥1 req/s on a key (the normal case), `int(elapsed.Seconds())` truncates to 0 while `lastRefill` advances, so tokens never refill. Reproduced: rate=100/s admitted **5 of 30** requests; the existing test sleeps 1.1s once and hides it.
- **Breaks its own SSE and WebSocket.** `LoggingMiddleware.statusRecorder` (`middleware/http/middleware.go:472`) wraps `http.ResponseWriter` and overrides only `WriteHeader` — it forwards neither `http.Flusher`, nor `http.Hijacker`, nor `Unwrap()` for `http.ResponseController`. With `LoggingMiddleware` installed, `StreamSimple`/SSE routes return 500 "streaming not supported" (Flusher assertion at `sse.go:346`), and WebSocket upgrades fail with 501. `gzipResponseWriter` in the same file forwards `Flush/Hijack/Push` correctly — the inconsistency is the tell.
- **Shuts down programmatically with zero drain.** `BrewContext` (`server.go:159-166`) waits on `<-ctx.Done()` and then calls `gracefulShutdown(ctx, ...)` with the **same canceled context** as parent. `context.WithTimeout(parentCtx, timeout)` yields an already-expired shutdown context, `OnShutdown` hooks receive `ctx.Err()==context.Canceled`, and `srv.Shutdown` returns instantly without draining in-flight requests — voiding the documented six-step sequence on the programmatic path. `Brew()` (which passes `context.Background()`) is unaffected; every existing shutdown test bypasses the bug by calling `gracefulShutdown` with `context.Background()` directly.
- **Races the OpenAPI generator despite claiming it is race-safe.** The `Generator` godoc (`openapi/openapi.go:141-147`) explicitly says register-while-serving is race-safe and "verified under `-race`". Every mutation method — `AddPath`, `AddSchema`, `AddSecurityScheme`, `Server` — writes `g.spec` **outside** `g.mu`; only the cache fields are guarded. A mutate-while-serve `-race` test yields `WARNING: DATA RACE` immediately. Plus a recursive request/response type stack-overflows `GenerateSchemaFromType` (`openapi.go:370-447`) at route registration — unrecoverable, no cycle detection, no `$ref` emission.
- **Accepts unbounded JSON bodies.** `JSON[T].Extract` (`response.go:96-102`) decodes `r.Body` with `sonic.ConfigDefault.NewDecoder(r.Body).Decode(...)` — no size limit. Meanwhile `http.go:14-45` documents `MaxPayloadSize=1MB` and a pooled buffer for "safe JSON decoding — prevent memory exhaustion attacks", but the only caller of `DecodeSafeJSON` is `cmd/example/main.go:111`. The documented safety property does not hold on the path every JSON handler actually uses.

Plus four P1 defects that break user code on well-formed inputs, and a docs sweep that fixes several core-page signatures that would not compile if copied.

## Design Principles

Carried forward unchanged from v2.3:

1. **Coffee metaphor** for any public surface.
2. **Type-safety first** — generics over `any`.
3. **Zero-allocation hot paths** — performance-conscious; atomic over mutex on hot paths.
4. **State injection via `MustGetState[T]`** stays the canonical pattern.
5. **Context-first** on I/O.
6. **Structured errors** — `*espresso.Error` and the canonical JSON envelope are the contract.
7. **Trustworthy generated artifacts** (from v2.3) — spec-inspection tests lock generator output.

The v2.0 charter still applies: the backward-compat flip holds — v2.x may make breaking or behavior changes, each justified and documented.

New emphasis for v2.4:

8. **Truth-in-docs.** When a doc comment or a README bullet describes behavior the code does not perform, fix one of the two — but never both by paraphrase. "Zero-allocation handlers" (README.md:18), "Uses pooled byte slices" (`extractor/extractor.go:293`), "verified under `-race`" (`openapi/openapi.go:145`), "Middleware runs in reverse order" (`docs/guide/middleware/index.md:83`), "handler's context is cancelled on shutdown" (`docs/streaming.md:216-232` — closed by v2.3 SSE fix in #58 for Close()) — every one of these was contradicted by measurement or by a repro test. Truth-in-docs is the release's second axis after correctness.

9. **Regression tests written from the repro, not paraphrased from the fix.** Every P0 fix is locked by the same test shape that reproduced the bug (a `-race` run against the exact goroutine pattern for the pool race; a request-rate loop for the rate limiter; a mutate-while-serve for the OpenAPI generator). A green unit test that only exercises the fixed function in isolation is not sufficient — the lock is what would have caught the bug before the audit did.

## Task Index

### 🔴 P0 — Must Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 1 | TimeoutLayer + pool data race | [task-01-timeoutlayer-pool-race.md](./tasks/task-01-timeoutlayer-pool-race.md) | 1.5 days |
| 2 | TokenBucketLimiter refill starvation | [task-02-tokenbucket-refill.md](./tasks/task-02-tokenbucket-refill.md) | 0.5 day |
| 3 | LoggingMiddleware statusRecorder — Flusher/Hijacker/Unwrap | [task-03-logging-statusrecorder.md](./tasks/task-03-logging-statusrecorder.md) | 0.5 day |
| 4 | BrewContext dead-context shutdown + SSE-vs-WriteTimeout | [task-04-brew-shutdown-and-writetimeout.md](./tasks/task-04-brew-shutdown-and-writetimeout.md) | 1 day |
| 5 | OpenAPI Generator mutate-while-serve race + recursive-type overflow | [task-05-openapi-generator-races.md](./tasks/task-05-openapi-generator-races.md) | 1.5 days |
| 6 | Bound JSON[T] body extraction | [task-06-json-body-cap.md](./tasks/task-06-json-body-cap.md) | 1 day |

### 🟡 P1 — Should Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 7 | Circuit breaker — four state-machine defects | [task-07-circuit-breaker-defects.md](./tasks/task-07-circuit-breaker-defects.md) | 1 day |
| 8 | Rate-limit — XFF trust + per-key eviction | [task-08-ratelimit-xff-eviction.md](./tasks/task-08-ratelimit-xff-eviction.md) | 0.5 day |
| 9 | Validator — pointer-field dereference | [task-09-validator-pointer-fields.md](./tasks/task-09-validator-pointer-fields.md) | 0.5 day |
| 10 | Truth-in-docs sweep (README, docs/api, guides, core.go, examples) | [task-10-truth-in-docs.md](./tasks/task-10-truth-in-docs.md) | 2 days |

### 🔵 Verification

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 11 | Regression matrix + snippet-compile hardening | [task-11-verification.md](./tasks/task-11-verification.md) | 1 day |

### 📦 Meta

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 12 | CHANGELOG + v2.4.0 release + migration guide | [task-12-changelog-release.md](./tasks/task-12-changelog-release.md) | 0.5 day |

## Recommended Execution Order

See [EXECUTION_ORDER.md](./EXECUTION_ORDER.md). Tasks 1, 2, 3, 4, 5, 6, and 8 touch disjoint files and dispatch in parallel; the circuit-breaker fix (Task 7) lands after Task 1 because both edit `middleware/service/layer.go`; the docs pass (Task 10) can run in parallel with the code work; verification and the release follow.

## For AI Agents

See [AGENT_GUIDELINES.md](./AGENT_GUIDELINES.md). v1.3, v1.4, v1.5, v2.0, v2.1, v2.2, v2.3 rules carry over; this file lists only what changed for v2.4.

## Project Information

- **Repository:** `github.com/suryakencana007/espresso/v2`
- **Previous Version:** v2.3.0 (2026-06-29, "Backflush")
- **Target Version:** v2.4.0
- **Module path:** unchanged (`/v2`; v2.4 is a minor bump)
- **Downstream Project:** Barista
- **Audit source:** 10-dimension alignment audit run 2026-07-02/03/04 (workflow runs `wf_946afaeb-ca4`, 221 agents, 114 findings). Full digest was ephemeral (temp files); the durable summary is the chat report and the memory entry `full-audit-2026-07.md`. The audit report contains ~20 additional low/medium findings that are documented but explicitly not v2.4 scope — captured as follow-ups.

## Existing Package Layout (post-v2.3)

v2.4 adds **no new packages or directories** — it edits existing files only. PR #58 (2026-07-04) already landed `LICENSE`, `.github/workflows/ci.yml` (test-race, lint, govulncheck), the docs.yml setup-go cleanup, and the `SSEStream.Close()` ctx-cancellation fix (audit finding `streaming#3`).

```
espresso/
├── context.go
├── core.go                     ← Task 10 fixes value-typed extractor doc examples
├── error.go
├── handler.go
├── handler_cache.go
├── http.go                     ← Task 6 wires DecodeSafeJSON / MaxPayloadSize into JSON[T].Extract
├── layerconfig.go
├── layerstack.go
├── response.go                 ← Task 6 edits JSON[T].Extract; Task 10 fixes WriteResponse comment
├── router.go
├── router_layers.go
├── router_openapi.go
├── server.go                   ← Task 4 fixes BrewContext + default WriteTimeout
├── service.go
├── sse.go                      ← Task 4 wires SetWriteDeadline for SSE (if defaults keep WriteTimeout>0)
├── state.go
├── validate.go
├── websocket.go
├── withlayers.go               ← Task 1 skips pool.Put on abandonment; Task 10 docs fixes
├── bench/
├── cmd/example/                ← Task 4 fixes cmd/example/sse+websocket goroutine-Brew antipattern
├── docs/                       ← Task 10 sweeps middleware-order + docs/api signatures + guides
│   ├── api/
│   ├── examples/
│   ├── guide/
│   ├── migration-v1-to-v2.md
│   ├── migration-v2-to-v2.1.md
│   └── migration-v2.3-to-v2.4.md   ← new in v2.4 (Task 12)
├── extractor/                  ← Task 10 fixes extractor/doc.go value-typed example
├── internal/
│   ├── errorenvelope/
│   └── validatehook/
├── middleware/
│   ├── http/                   ← Tasks 3 (statusRecorder), 8 (RateLimit) edit middleware.go
│   └── service/                ← Tasks 1 (Timeout), 7 (CircuitBreaker) edit layer.go
├── openapi/                    ← Task 5 fixes generator race + cycle detection
├── pool/
├── tests/integration/
└── validator/                  ← Task 9 fixes pointer-field deref in walkStruct
```

## Related, Deferred

Items the audit surfaced that v2.4 deliberately does **not** pick up, so the scope boundary is explicit:

- **Real JWT cryptographic validation.** `JWTMiddleware` (`middleware/http/auth.go:28-93`) declares `Secret` and `SigningMethod` fields that are dead config; all validation is delegated to `ClaimsExtractor`. Fixing this requires adding a JWT dependency (`golang-jwt/jwt` or equivalent), an alg-allowlist, and exp/nbf/leeway handling — a feature addition, not a correctness fix. v2.4 stays hands-off; a follow-up either implements it as a real feature or renames/deletes the misleading fields.
- **File/multipart content access.** `FileInfo` (`extractor/extractor.go:109-113`) drops `*multipart.FileHeader`, so typed handlers cannot read uploaded content. Fixing this changes the `FileInfo` surface and needs a documented request-in-context escape hatch or an `Open()` method — a design decision, not a correctness fix. Deferred.
- **Reflection-path per-request cost.** The reflection dispatch path (`WithLayers` without types) allocates ~13 allocs/op vs the typed path's 8, from per-request `reflect.Call` and no request pooling. Documented in the audit as `core-perf#4`, but WithLayers-vs-WithLayersTyped is a design tradeoff, not a defect. Deferred; if hot-path users hit this, the answer is `WithLayersTyped`, which Task 10 will surface more prominently.
- **The 2-4 alloc registry-injection tax in `Router.ServeHTTP`.** Every request pays for WS/SSE registry context injection. A prebox optimization is small; a "no-injection when no streams registered" opt-in is bigger. Deferred pending a concrete performance complaint.
- **Additional low/medium audit findings** (repeated query-param binding, XML value-receiver footgun, mid-body write→500-envelope, `pool/` package cleanup or removal, README front-door surfacing of `LungoNoErr`) — documented in the audit report and left to a future minor. Task 10's truth-in-docs pass touches the doc-visible subset of these.

## Global Conventions

Same as v2.3:

- Unit tests with minimum 80% coverage on touched lines.
- `cmd/example/` updated when user-facing behavior changes (Task 4 explicitly fixes cmd/example/sse and cmd/example/websocket).
- All public APIs have godoc comments with examples; when this release changes behavior, godoc is corrected in the same PR.
- `go test ./... -race` clean before a task is considered done.
- `golangci-lint run ./...` clean before a task is considered done.
- `govulncheck ./...` clean (regressed-guarded by CI as of #58).
- Behavior changes are locked with a regression test **shaped like the audit's repro**, not a paraphrase of the fix, and called out in the CHANGELOG `Changed`/`Fixed` section with before/after.
