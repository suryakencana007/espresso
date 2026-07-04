# Execution Order for v2.4.0

This document provides the recommended execution order for the v2.4 roadmap tasks. The schedule is sized for approximately two focused weeks. v2.4 is a correctness/quality cleanup pass — the headline is closing the P0/P1 defects the 2026-07-02/03/04 alignment audit reproduced against v2.3.0. The work is broader than v2.3's OpenAPI-focused cycle but each individual task is narrow, and seven of the ten code tasks touch disjoint files, so heavy parallelism is available.

## Overview (planned)

```
Day 1-3:  Task 1 (TimeoutLayer + pool race)                 ─┐
          Task 2 (TokenBucketLimiter refill)                 │ PARALLEL — disjoint files
          Task 3 (LoggingMiddleware statusRecorder)          │ (middleware/service/layer.go ‖
          Task 4 (BrewContext + WriteTimeout + cmd/example)  │  middleware/http/middleware.go ‖
          Task 5 (OpenAPI generator races + cycles)          │  middleware/http/middleware.go ‖
          Task 6 (JSON body cap)                             │  server.go + cmd/example ‖
          Task 8 (RateLimit XFF + eviction)                  │  openapi/openapi.go ‖ response.go
          Task 9 (Validator pointer fields)                  │  middleware/http/middleware.go ‖
          Task 10 (Truth-in-docs)                           ─┘  validator/validator.go ‖ docs/)
Day 4:    Task 7 (Circuit breaker) — starts AFTER Task 1 lands
                                     (shares middleware/service/layer.go with Task 1)
Day 5-6:  Task 11 (Verification) — once Tasks 1-10 are in
Day 7:    Task 12 (CHANGELOG & v2.4.0 release)
```

The shape is deliberately front-loaded: all six P0 tasks, two of the four P1 tasks, and the docs sweep touch disjoint files and can be dispatched together, leaving only Task 7 (circuit breaker, which shares `middleware/service/layer.go` with Task 1) serialized, and Tasks 11 and 12 downstream by definition.

## Week 1 — Ground Truth

The theme is the release tagline: the framework's behavior catches up to what its docs, comments, and marketing claim. Every day below ends at a PR; every PR rebases against the running `[Unreleased]` CHANGELOG section.

### Day 1-3 — Parallel batch (Tasks 1, 2, 3, 4, 5, 6, 8, 9, 10)

These nine run in **parallel** — their files are disjoint. Task 1 lives in `middleware/service/layer.go` (Timeout function only); Task 2 in `middleware/http/middleware.go` (`TokenBucketLimiter` methods only); Task 3 in the same file but on `statusRecorder`/`LoggingMiddleware`; Task 4 in `server.go` + `sse.go` + `cmd/example/{sse,websocket}/main.go`; Task 5 in `openapi/openapi.go`; Task 6 in `response.go` + `http.go` + a router option; Task 8 in `middleware/http/middleware.go` on `RateLimitMiddleware`/`SlidingWindowLimiter`; Task 9 in `validator/validator.go`; Task 10 across `docs/`, `core.go` godoc, and `README.md`. Tasks 2, 3, and 8 all touch `middleware/http/middleware.go` on **disjoint methods** — coordinate on the `[Unreleased]` CHANGELOG merge order, but the code diffs do not conflict.

**Task 1 — TimeoutLayer + pool data race (`middleware/service/layer.go`, `withlayers.go`)**

- Step 1.1: Reproduce the race with the audit's exact shape — `WithLayersTyped` + `Timeout(10ms)` + an 80ms handler that reads `req.Field` in a loop, plus a second request that gets the recycled pool struct and `Extract`s into it. `go test -race` must print `WARNING: DATA RACE` before the fix (this is the regression lock).
- Step 1.2: Decide: (a) do not recycle the request struct when the wrapped call returned a `context.DeadlineExceeded` (or a sentinel from `TimeoutLayer`) — skip `pool.Put`, let GC reclaim; or (b) make `TimeoutLayer` synchronous — no goroutine, only deadline propagation via `ctx`. (a) preserves the abandon-on-timeout semantics `Timeout` exists for; (b) removes the hazard by construction but changes user-visible latency behavior. Pick (a); (b) is documented in the task file as the alternative for future review.
- Step 1.3: Wire the skip into `applyLayersAndConvert` (`withlayers.go:405-424`). Also document in `core.go` (`Resettable` godoc) that pooled request structs must not be retained past handler return.
- Step 1.4: Add the `-race` regression test. Open the Task 1 PR.

**Task 2 — TokenBucketLimiter refill starvation (`middleware/http/middleware.go`)**

- Step 2.1: Reproduce the starvation with the audit's shape — rate=100/s, cap=5, one request every 100ms for 3s. Assert exactly 5 of 30 admissions on the pre-fix code (regression lock).
- Step 2.2: Fix the math in both `allowGlobal` (`middleware.go:306-320`) and `allowPerKey` (`middleware.go:337-358`): compute `refill` in nanoseconds/float, or advance `lastRefill` only by the credited time via `lastRefill.Add(time.Duration(whole)*time.Second)`.
- Step 2.3: Assert post-fix admission ~= `rate * duration` within tolerance (`~300 of 300` for the 3s run).
- Step 2.4: Open the Task 2 PR.

**Task 3 — LoggingMiddleware statusRecorder — Flusher/Hijacker/Unwrap (`middleware/http/middleware.go`)**

- Step 3.1: Reproduce the SSE break — install `LoggingMiddleware`, register a `StreamSimple` SSE route, hit it, assert the pre-fix response is `500 {"error":{"code":"INTERNAL","message":"streaming not supported"}}` (regression lock). Same for WebSocket upgrade (should return non-101).
- Step 3.2: Extend `statusRecorder` to forward `http.Flusher`, `http.Hijacker`, `http.Pusher`, and provide `Unwrap() http.ResponseWriter` for `http.ResponseController`. Mirror `gzipResponseWriter` (`middleware.go:210-229`) which already does this correctly.
- Step 3.3: Assert post-fix SSE returns `200 text/event-stream` and WebSocket upgrades to `101`.
- Step 3.4: Open the Task 3 PR.

**Task 4 — BrewContext dead-context shutdown + SSE-vs-WriteTimeout (`server.go`, `sse.go`, `cmd/example/{sse,websocket}/main.go`)**

- Step 4.1: Reproduce the drain skip — `signal.NotifyContext` + `router.BrewContext(ctx, ...)` + a 500ms in-flight request + `WithShutdownTimeout(5s)`; cancel the ctx and assert the request is *not* drained (BrewContext returns in ~0s, request goroutine still running) on the pre-fix code.
- Step 4.2: Fix `BrewContext` (`server.go:159-166`) to derive the shutdown context via `context.WithoutCancel(ctx)` before calling `gracefulShutdown`, so `srv.Shutdown` gets a fresh `WithTimeout` from an uncancelled parent and `OnShutdown` hooks see a live ctx with a future deadline.
- Step 4.3: Reproduce the WriteTimeout SSE kill — an SSE stream through `router.Brew(WithAddr(":0"))` with default `WriteTimeout=10s` dies at ~10s despite `WithKeepAlive`. Fix in `sse.go` by using `http.NewResponseController(w).SetWriteDeadline(...)` per write (Go 1.20+), extending the per-write deadline on every `Send`/`Comment`. Do not change the default `WriteTimeout` — the 10s protection is correct for non-stream routes.
- Step 4.4: Fix `cmd/example/sse/main.go:51-57` and `cmd/example/websocket/main.go:53-59` to stop running `Brew` in a goroutine while blocking main on `signal.NotifyContext` for the same signals — either use plain `router.Brew(...)` (blocks on signals internally) or the `BrewContext(ctx, ...)` pattern with `signal.NotifyContext` and the ctx propagated in.
- Step 4.5: Open the Task 4 PR. Regression tests: `TestBrewContext_DrainsInFlight`, `TestSSE_SurvivesDefaultWriteTimeout`.

**Task 5 — OpenAPI Generator mutate-while-serve race + recursive-type overflow (`openapi/openapi.go`)**

- Step 5.1: Reproduce the mutate-while-serve race — `AddPath` called concurrently with `Handler().ServeHTTP` under `-race`; the pre-fix code prints `WARNING: DATA RACE`. This directly contradicts the `Generator` godoc claim at `openapi.go:141-147` ("verified under `-race`") — that godoc is corrected in the same PR.
- Step 5.2: Fix by holding `g.mu` across the spec mutation in every mutation method (`AddPath`, `AddSchema`, `AddSecurityScheme`, `Server`, `Description`, `SetDescription`), folding `invalidateCache()` into the same critical section. Do not introduce a second lock; use the existing one correctly.
- Step 5.3: Reproduce the recursive-type overflow — `GenerateSchemaFromType(reflect.TypeOf(node{}))` where `type node struct{ Children []*node }` dies with `fatal error: stack overflow`. Fix by threading a visited-types set through schema generation; on revisit, register under `components/schemas` and return `{$ref: "#/components/schemas/<Name>"}`. The `Schema` type already has a `Ref` field (`openapi.go:106`); the infrastructure is present.
- Step 5.4: Assert post-fix: the concurrent test's `-race` is clean, and the recursive type produces a spec with a `$ref` to itself instead of dying.
- Step 5.5: Update the `Generator` godoc to reflect real behavior. Open the Task 5 PR.

**Task 6 — Bound JSON[T] body extraction (`response.go`, `http.go`, router option)**

- Step 6.1: Confirm the exposure — `JSON[T].Extract` (`response.go:96-102`) is unbounded; `DecodeSafeJSON` (`http.go:62-76`) with `MaxPayloadSize=1MB` is only used by `cmd/example`.
- Step 6.2: Wire `JSON[T].Extract` through the safe path. Two viable shapes: (a) route `JSON[T].Extract` through `DecodeSafeJSON` directly, using the existing pooled buffer and 1MB cap; (b) wrap the body with `http.MaxBytesReader(w, r.Body, cap)` — but `Extract` has no `w` in scope, so (a) is the fit. Add a router-level option `WithJSONBodyLimit(n int64)` that sets the cap, defaulting to `MaxPayloadSize`.
- Step 6.3: Return an `*espresso.Error` on cap overflow: `ErrRequestEntityTooLarge(413, "request body exceeds N bytes")` (add the constructor if missing; `error.go` has the pattern). Do not silently truncate.
- Step 6.4: Regression test: `TestJSON_ExtractRejectsOverLimit` — a body of `cap+1` bytes returns 413 with the canonical envelope; a body at `cap` succeeds.
- Step 6.5: Also cap `extractor.RawBody`/`RawBodyWithHeaders` on the same knob (currently uncapped `io.ReadAll`, audit `extract-resp-pool#5`). Open the Task 6 PR.

**Task 8 — Rate-limit XFF + eviction (`middleware/http/middleware.go`)**

- Step 8.1: Reproduce the spoof — `RateLimitMiddleware` with per-key limiter, client sends `X-Forwarded-For: attacker-controlled`; the key becomes the attacker's chosen value. And: the ephemeral port in `r.RemoteAddr` means each new TCP connection gets a fresh bucket (regression lock).
- Step 8.2: Key on `net.SplitHostPort(r.RemoteAddr).Host` by default (no port). Make `X-Forwarded-For` opt-in behind a `WithTrustedProxies(...)` option — take the *rightmost trusted hop* only, not the raw header value.
- Step 8.3: Add TTL-based eviction to `TokenBucketLimiterPerKey`'s buckets map — a background sweep or per-Allow lazy expiry that removes keys idle for > TTL. Match `SlidingWindowLimiter`'s cleanup shape at `middleware.go:391-411`.
- Step 8.4: Regression tests: `TestRateLimit_XFFNotTrustedByDefault`, `TestRateLimit_TrustedProxyTakesRightmostHop`, `TestRateLimitPerKey_EvictsIdleBuckets`.
- Step 8.5: Open the Task 8 PR.

**Task 9 — Validator pointer-field dereference (`validator/validator.go`)**

- Step 9.1: Reproduce the failure — `type Req struct { Email *string \`validate:"email"\` }` with `Email = strPtr("a@b.com")` returns `email rule requires string field` on the pre-fix code. Same for `*int` with `min=18` and a valid value (regression lock).
- Step 9.2: Fix `walkStruct` (`validator/validator.go:104-119`): before applying rules, skip non-`required` rules when the pointer is nil (matching the documented email/url empty-pass semantics), and dereference non-nil pointers so rules see the element value. Keep `required` operating on the pointer itself.
- Step 9.3: Regression tests over each rule × `*string`/`*int` × nil/valid/invalid.
- Step 9.4: Open the Task 9 PR.

**Task 10 — Truth-in-docs sweep (`README.md`, `docs/api/`, `docs/guide/`, `docs/examples/`, `core.go`, `extractor/doc.go`, `docs/streaming.md`)**

- Step 10.1: Rewrite the "zero-allocation handlers" claim across `README.md:18/1147-1151`, `docs/index.md:126-128,144-148`, `docs/guide/index.md:11`, and `core.go:3` to the defensible measurable claim: "request objects are pooled via `sync.Pool` — zero request-struct allocations per request". Replace the "Zero per request" table cells with the measured per-request numbers.
- Step 10.2: Fix the middleware-order docs in all four confirmed locations — `docs/guide/middleware/index.md:83-91`, `docs/examples/middleware-stack.md:69-73`, `service.go:49-63` godoc, and `withlayers.go:391`. The rule is: first-registered = outermost = executes first.
- Step 10.3: Fix `docs/api/espresso.md` for `Solo` (currently ctx-only, should be `func Solo[Req FromRequest, Res IntoResponse](fn func(Req) (Res, error))`), `Doppio` (add mandatory error return), `Ristretto` example (return `Text`, not bare string). Delete phantom `WithServer`; document the six real `ServerOption` constructors plus `BrewContext`, `OnShutdown`, `ShutdownHook`.
- Step 10.4: Fix `docs/api/state.md`: correct `GetState` return to `(T, bool)`, delete phantom exported `StateKey`, rewrite the 3-arg handler example to `espresso.Lungo(...)` with `*State[T]`. Add `FromState` and `FromMustState`.
- Step 10.5: Add v2.3 features to `docs/api/openapi.md`: `AddSecurityScheme`, `BearerScheme`, `APIKeyHeaderScheme`, `UnresolvedSecurityRefs`, `openapi.Security`, `openapi.Status`/`OpenAPIStatusCode`, `ScalarVersion`.
- Step 10.6: Fix `core.go:32-57` and `extractor/doc.go:13` to show pointer-typed extractor args (`req *JSON[CreateUserReq]`) — the current value-typed examples panic at registration.
- Step 10.7: Fix `docs/api/validator.md` to include `AsDefaultValidator`. Fix `docs/api/middleware-service.md` to include `ValidatorFunc`, `ErrCircuitBreakerOpen`, `CircuitBreakerState`.
- Step 10.8: Update `README.md` to mention `Lungo` and `LungoNoErr` as the two-extractor entry points; update `CLAUDE.md`'s alias table and Err* list to include `LungoNoErr` and `ErrPreconditionFailed`.
- Step 10.9: Fix false `JSON.WriteResponse` comment at `response.go:71-74` (claims pooled buffer that does not exist — either wire the pool per Task 6, or delete the comment; coordinate with Task 6). Same for `RawBody` docs at `extractor.go:293/299`.
- Step 10.10: Fix `docs/streaming.md:149` `Send(name, data)` → `Send(Event{Name, Data})`.
- Step 10.11: Fix `installation.md:6` "CGO required" claim (empirically false — cross-compiles fine without CGO).
- Step 10.12: Open the Task 10 PR.

### Day 4 — `Circuit breaker four defects` (Task 7)

Starts **after Task 1 lands**. Task 7 edits `middleware/service/layer.go` (shared with Task 1); serializing avoids conflicts on the CircuitBreaker methods.

- Step 7.1: Reproduce the four defects with distinct tests: (a) 5 failures over 102 calls at 99% success opens the circuit despite the failures being non-consecutive; (b) `SuccessThreshold=1` half-open probe succeeds but the circuit stays open due to stale-state check; (c) concurrent Success + Failure produces reopen-then-close where the failure count is lost; (d) 10 concurrent requests observing HalfOpen all pass through as probes.
- Step 7.2: Reset `failures` on success while `StateClosed` (or use a rolling window); re-check state under the write lock before mutating on the success path; count the transitioning probe's result by capturing the observed state at the mutation site, not the entry site; cap half-open concurrency to 1 probe (or configurable N).
- Step 7.3: Open the Task 7 PR.

### Day 5-6 — `Verification` (Task 11)

Depends on Tasks 1-10 being in. Lands once the substantive fixes have merged so it tests reality, not intent.

- Step 11.1: Verify every P0 regression test reliably fails on the pre-fix commit (git-bisect-style spot-check).
- Step 11.2: Harden the docs-snippet-compile guard — extend it to compile fences that reference third-party imports via a whitelist (e.g. `golang-jwt/jwt` for the auth Complete Example) rather than skipping them. Alternatively, extract the Complete Example fences into `docs/examples/testdata/` and compile them there.
- Step 11.3: Confirm `go test ./... -race -shuffle=on` passes on both local Windows and CI Linux.
- Step 11.4: Confirm `golangci-lint run ./...` clean; `govulncheck ./...` clean; `go test -tags=integration ./tests/integration/...` passes.
- Step 11.5: Open the Task 11 PR.

### Day 7 — `CHANGELOG & v2.4.0 release` (Task 12)

Last task. Depends on Tasks 1-11.

- Step 12.1: Promote `[Unreleased]` → `[2.4.0]` in `CHANGELOG.md` in a single atomic commit. The correctness fixes (Tasks 1-9) go under **Fixed**; the truth-in-docs sweep (Task 10) under **Changed**; the body-cap knob (Task 6) additive under **Added**.
- Step 12.2: Bump version chips — `package.json` to `2.4.0` and `docs/.vitepress/config.ts` to `v2.4.0` — in the same atomic commit.
- Step 12.3: Write `docs/migration-v2.3-to-v2.4.md` covering: the SSE default-WriteTimeout behavior change (SSE now survives on default settings), the `RateLimit` default-XFF-not-trusted breaking change, the JSON body cap (413 on oversized bodies), the CircuitBreaker semantics (failures reset on Closed-state successes).
- Step 12.4: Final quality gates: `go test -race ./...`, `golangci-lint run ./...`, `govulncheck ./...`, `go test -tags=integration ./tests/integration/...`, bench module spot-check.
- Step 12.5: Tag `v2.4.0` from the merge commit, push, `gh release create v2.4.0` with the `[2.4.0]` body.

## Contingency Planning

### Must Not Slip (hard requirements for the v2.4 release)

- Task 1 (TimeoutLayer race) — `-race`-reproduced, cross-request data corruption on the flagship service layer.
- Task 2 (TokenBucket refill) — starves the rate limiter under normal traffic; the audit reproduced 5-of-30 admission.
- Task 3 (LoggingMiddleware SSE/WS break) — two first-party features (LoggingMiddleware + StreamSimple/WebSocketSimple) are mutually exclusive today.
- Task 4 (BrewContext drain + SSE WriteTimeout) — the programmatic-shutdown path skips drain entirely; SSE dies at ~10s on the default config.
- Task 5 (OpenAPI race) — the godoc explicitly claims race-safety it does not have.
- Task 6 (JSON body cap) — advertised safety property is dead code on the framework path.
- Task 10 (Truth-in-docs) — the "zero-allocation" claim, middleware-order docs, and the wrong `Solo`/`Doppio`/`GetState` signatures ship user-facing lies; the docs are part of the release contract.
- Task 12 (release).

### Can Slip to v2.4.1 (next patch)

- Task 11 (verification) — hardening; if the cycle runs hot, a thinner version can land with v2.4.0 and the full docs-snippet guard extension follow in the patch.
- Task 8 (RateLimit XFF/eviction) — real defect but exploitable only in specific deployment topologies; a patch release is acceptable if the P0 spine has ordering pressure.

### Can Slip to v2.5 (next minor)

- Task 7 (Circuit breaker four defects) — semantic correctness of a service layer that fewer users touch than TimeoutLayer; can wait one minor if needed.
- Task 9 (Validator pointer fields) — real defect on `*string`/`*int` optional fields with rules, but the workaround (rules on the value field with `required` on the pointer) is documentable.
- Additional Task 10 sub-items (e.g. the JWT-no-crypto doc note; the file-content-inaccessible docs note) if the docs pass proves oversized.

### Must Not Compromise (quality gates)

- `go test -race ./...` clean.
- `golangci-lint run ./...` clean.
- `govulncheck ./...` clean.
- `go test -tags=integration ./tests/integration/...` passes.
- Every P0 regression test reliably fails on pre-fix code (Task 11 spot-check).

## Parallel Work Opportunities

**Parallel batch 1** (no cross-file conflicts):
- Tasks 1, 2/3/8 (disjoint methods in `middleware/http/middleware.go`), 4 (`server.go`+`sse.go`+`cmd/example`), 5 (`openapi/openapi.go`), 6 (`response.go`+`http.go`), 9 (`validator/`), 10 (`docs/`+`core.go`) — dispatch all together on Day 1 (isolated worktrees, as in v2.1's tasks 02/03/04 round and v2.3's four-way parallel dispatch).

**Strictly serialized** (shared file):
- Task 7 shares `middleware/service/layer.go` with Task 1 — must land after Task 1.

**Downstream** (after the fixes are in):
- Task 11 (verification) is downstream of Tasks 1-10 by definition; Task 12 (release) is downstream of everything.

## Notes

- All v2.4 work happens on the `/v2` module path; no module-bump considerations.
- v2.4 adds **no** new packages. It edits existing files only, plus adds one new doc: `docs/migration-v2.3-to-v2.4.md`.
- Resist scope creep. v2.4 is a correctness cleanup: fix the six confirmed P0 defects, the four confirmed P1s, sweep the docs, lock everything with tests, ship. If the audit findings deferred to v2.5+ (JWT crypto validation, file-content extraction, per-Router handler-cache config, README front-door surfacing of `LungoNoErr` beyond Task 10.8's mention) come up as adjacent while a task is landing, capture as a follow-up issue rather than bundling.
- The CHANGELOG `[Unreleased]` section will take conflicts on each PR after the first, as in v2.3 — these are one-file, one-section rebases and resolve in seconds.
- Every change lands via a feature branch + PR. CLI merge is blocked by branch protection; the maintainer merges via the GitHub UI. Use Conventional Commits throughout.
- Tasks 4, 6, and 8 are user-visible behavior changes that the migration guide (Task 12.3) must call out — SSE now survives default `WriteTimeout`, `RateLimitMiddleware` no longer trusts `X-Forwarded-For` by default, and `JSON[T].Extract` now returns 413 on oversized bodies.
