# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.4.0] - 2026-07-05

A correctness release — **Ground Truth** — that closes the ten P0/P1
findings the 2026-07-02 alignment audit surfaced against v2.3.0. Nine of
those were real bugs (a `-race`-reproducible data race in the flagship
`TimeoutLayer`, a token-bucket that starved under sustained sub-second
traffic, `LoggingMiddleware` breaking the framework's own SSE and
WebSocket, `BrewContext` skipping in-flight-request drain, an OpenAPI
generator that raced its own cache and stack-overflowed on recursive
types, unbounded JSON body reads, four circuit-breaker state-machine
defects, `X-Forwarded-For` blindly trusted for rate-limiting, and
validator rules that failed every non-nil pointer field), and one was a
truth-in-docs sweep that reworded the "zero-allocation handlers"
marketing claim against measurement. No new runtime feature; every
change is either a correctness fix or an additive safety option.

**Upgrade from v2.3:** see
[`docs/migration-v2.3-to-v2.4.md`](docs/migration-v2.3-to-v2.4.md). Two
items need action for correctness:

- **`RateLimitMiddleware` no longer trusts `X-Forwarded-For` by default.**
  Add `httpmiddleware.WithTrustedProxies("your-proxy-cidr/prefix", ...)`
  if you run behind a reverse proxy — otherwise all requests key on the
  proxy's IP.
- **JSON body reads are capped at 1 MB by default.** If any of your
  routes legitimately accepts larger bodies, set
  `router.WithJSONBodyLimit(N)`. Otherwise clients hit `413 Payload
  Too Large`.

Every other v2.4 change is either an internal correctness fix or an
additive option. Existing apps compile against v2.4 unchanged.

### Added

- **`Router.WithJSONBodyLimit(int64)`** — configure the per-router
  request-body size cap. Applies to `JSON[T]`, `RawBody`,
  `RawBodyWithHeaders`, and `XML[T]` extractors. Default is
  `MaxPayloadSize` (1 MB); non-positive values fall back to the default.
  Bodies larger than the cap return `413 Payload Too Large` with the
  canonical error envelope before the handler is invoked (v2.4 task-06,
  #70).
- **`espresso.ErrRequestEntityTooLarge(msg string) *Error`** — 413
  Payload Too Large constructor. Framework extractors return this
  automatically when the body-size cap is exceeded; handlers can
  return it directly when their own I/O paths need the same shape.
- **`httpmiddleware.WithTrustedProxies(cidrs ...string) RateLimitOption`** —
  opt-in trust for `X-Forwarded-For` when behind a reverse proxy.
  Accepts CIDR notation or bare IPs (auto-wrapped `/32` for IPv4,
  `/128` for IPv6). When RemoteAddr's host is in a trusted CIDR and
  XFF is present, walks the XFF list right-to-left, skipping trusted
  hops, and returns the first non-trusted address as the client key
  (RFC 7239) (v2.4 task-08, #69).
- **`httpmiddleware.WithBucketTTL(d time.Duration) TokenBucketLimiterOption`** —
  configure the idle-bucket eviction TTL on `TokenBucketLimiterPerKey`
  (default 10 minutes) (v2.4 task-08, #69).
- **`(*TokenBucketLimiter).Close() error`** — stop the background
  sweeper goroutine on per-key limiters. No-op on global limiters, safe
  to call unconditionally (v2.4 task-08, #69).
- **`CircuitBreakerConfig.HalfOpenMaxProbes int`** — bounds concurrent
  probes admitted in the `HalfOpen` state (default 1). Extra requests
  short-circuit with a `CircuitBreakerError` instead of stampeding the
  recovering upstream. Zero or negative values are normalized to 1 at
  layer construction (v2.4 task-07, #68).
- **`servicemiddleware.IsAbandonedByTimeout(err error) bool`** —
  detects the sentinel error returned by `TimeoutLayer` when it
  abandoned a still-running handler goroutine. Framework internal use
  primarily (v2.4 task-01, #64).
- **`docs/migration-v2.3-to-v2.4.md`** — release-cycle migration guide
  covering the two required actions above plus the semantic changes to
  circuit breaker, validator, and shutdown.

### Changed

- **`RateLimitMiddleware` no longer trusts `X-Forwarded-For` by
  default; keys on host, not host:port** (v2.4 task-08, #69). Every
  request keys on the host portion of `r.RemoteAddr` via
  `net.SplitHostPort` — the pre-v2.4 default of full `host:port` let a
  client bypass a per-key limiter by reconnecting from a fresh
  ephemeral port. `X-Forwarded-For` is ignored unless a request's
  RemoteAddr host is in a `WithTrustedProxies` CIDR. Callers behind a
  reverse proxy must add the option; direct-Internet-facing callers
  need no change. **Breaking behavior change.**
- **`JSON[T].Extract`, `extractor.RawBody`, `extractor.RawBodyWithHeaders`,
  and `extractor.XML[T]` now respect a configurable body cap** (default
  1 MB via `MaxPayloadSize`) instead of reading unbounded (v2.4
  task-06, #70). Existing callers with bodies under 1 MB see no change;
  callers with larger bodies must set `router.WithJSONBodyLimit(N)`.
  **Breaking behavior change on request paths using >1 MB bodies.**
- **`CircuitBreakerLayer` — failures reset on Closed-state success**
  (v2.4 task-07, #68). In v2.3, `FailureThreshold=5` meant "5 failures
  ever" (accumulated over process lifetime). v2.4 resets the counter
  on any successful call while `Closed`, matching Netflix Hystrix's
  behavior and user intuition.
- **`BrewContext` drains in-flight requests on ctx cancel** (v2.4
  task-04a, #62). The programmatic shutdown path previously passed the
  canceled ctx as the parent of `context.WithTimeout`, yielding an
  already-expired shutdown ctx — `http.Server.Shutdown` returned in
  ~0 ms without waiting for in-flight requests, and `OnShutdown` hooks
  saw a canceled ctx. `BrewContext` now uses `context.WithoutCancel`
  to derive the shutdown timeout, honoring the documented drain
  contract.
- **SSE streams survive the default server `WriteTimeout` (10 s)**
  (v2.4 task-04b, #67). `serveStream` clears the per-connection write
  deadline via `http.NewResponseController(w).SetWriteDeadline(time.Time{})`
  at stream open, so SSE handlers can run indefinitely without the
  workaround `WithWriteTimeout(0)` — which had disabled DoS protection
  for all routes. The 10 s default remains in place for non-stream
  routes.
- **`context.Canceled` from a service layer maps to `503 Service
  Unavailable`** (v2.4 task-01, #64). Previously it fell through to
  the internal-error fallback (500). Now it joins
  `context.DeadlineExceeded` in the "service unavailable" bucket via
  `translateLayerError`.
- **`espresso.CircuitBreakerError` is a type alias for
  `servicemiddleware.CircuitBreakerError`** (v2.4 task-07, #68). Both
  packages resolve to the same underlying type, so `errors.As` from
  either package matches the same instance. Existing users see no
  surface change.
- **Truth-in-docs sweep** (v2.4 task-10 across #63 and #72). Corrects
  audit-flagged doc drifts:
  - Middleware execution order in `docs/guide/middleware/index.md`
    (first-registered = outermost = executes first — was documented
    backwards).
  - `docs/api/espresso.md` — corrected `Solo`/`Doppio`/`Ristretto`/`Lungo`
    signatures with actual `FromRequest`/`IntoResponse` constraints
    and mandatory error returns.
  - Deleted phantom `WithServer` `ServerOption`; documented the six
    real server options plus `BrewContext`, `OnShutdown`, `ShutdownHook`.
  - `docs/api/state.md` — corrected `GetState` return type to
    `(T, bool)`; added `FromState`, `FromMustState`; deleted phantom
    exported `StateKey`.
  - `core.go` + `extractor/doc.go` — value-typed extractor examples
    rewritten to pointer-typed (the value form panics at registration).
  - Reworded "zero-allocation handlers" across README, docs/index,
    docs/guide/index, and `core.go`'s package doc to the defensible
    pooled-request claim, with measured allocation numbers in the
    README's Handler Performance table.
  - Removed false pooling comments from `response.go` and
    `extractor/extractor.go`.
  - `WSConfig.ReadTimeout` godoc annotates the field as stored but
    not-yet-enforced (tracked for a future release).
- **`docs/api/index.md` overview** — rewrote the State Functions and
  Handler Functions blocks with correct signatures matching the API
  reference pages (v2.4 task-11, #73).

### Fixed

- **`TimeoutLayer` no longer races the request-struct pool** (v2.4
  task-01, #64). The layer spawned a goroutine that could outlive its
  handler; the outer handler unconditionally reset and re-pooled the
  request struct, and the next request `Extract`'d into memory the
  abandoned goroutine still read. `-race` reproduced the race on every
  test run of the flagship pairing (`WithLayersTyped` + `Timeout`).
  Fixed by returning a sentinel error that the handler detects to
  skip the pool return (`servicemiddleware.IsAbandonedByTimeout`); the
  abandoned struct is leaked to GC rather than reused while a
  goroutine still holds it.
- **`TokenBucketLimiter` refill starvation under sustained sub-second
  traffic** (v2.4 task-02, #60). Both `allowGlobal` and `allowPerKey`
  truncated `int(elapsed.Seconds())` to zero for any call arriving
  <1 s after the previous one, while unconditionally advancing
  `lastRefill = now`. Under ≥1 req/s traffic, tokens were consumed to
  zero and never replenished. Repro at `rate=100/s, cap=5, 30 requests
  every 100ms`: pre-fix admitted 5 of 30; post-fix admits ~30 of 30.
  Fixed by computing refill in nanoseconds via explicit `int64` math.
- **`LoggingMiddleware`'s `statusRecorder` forwards Flusher/Hijacker/
  Push/Unwrap** (v2.4 task-03, #61). Pre-fix `statusRecorder` embedded
  `http.ResponseWriter` and overrode only `WriteHeader`. Any route
  wrapped by `LoggingMiddleware` failed for SSE (500 "streaming not
  supported") and WebSocket (non-101 upgrade response). `Unwrap()`
  additionally lets `http.ResponseController` walk the wrapper chain
  for future callers using controllers.
- **`BrewContext` shutdown-drain fix** (v2.4 task-04a, #62 — see
  Changed for details).
- **SSE default-WriteTimeout survival** (v2.4 task-04b, #67 — see
  Changed for details).
- **`cmd/example/sse` and `cmd/example/websocket` use `BrewContext`
  correctly** (v2.4 task-04c, #66). Both previously ran `router.Brew`
  in a goroutine while blocking `main` on their own `signal.NotifyContext`
  for the same signals `Brew` traps — racing the framework's graceful
  shutdown against the process exit. Refactored to `main() → run() error`
  with `BrewContext(ctx, ...)` blocking on `ctx.Done()`.
- **OpenAPI generator mutate-while-serve race** (v2.4 task-05, #71).
  Every mutation method (`AddPath`, `AddSchema`, `AddSecurityScheme`,
  `Server`, `Description`, `SetDescription`) now holds `g.mu` across
  the spec mutation and the cache invalidation in a single critical
  section. Pre-fix a concurrent `AddPath` + `Handler.ServeHTTP` under
  `-race` reliably printed `WARNING: DATA RACE`. The `Generator` godoc
  claimed "verified under -race" but the existing test only served
  concurrently after registration completed; the new
  `TestGenerator_MutateWhileServeRaceFree` locks the real property.
- **OpenAPI generator no longer stack-overflows on recursive types**
  (v2.4 task-05, #71). `GenerateSchemaFromType` on any self-referential
  struct (a tree node, a comment thread, a linked category) previously
  died with `fatal error: stack overflow`. Now uses a per-call visited-
  types set: first visit inlines the struct schema, revisits emit
  `$ref` to `components/schemas/<sanitized-name>`. The exported
  signature is unchanged.
- **JSON/RawBody/XML body caps** (v2.4 task-06, #70 — see Changed for
  details).
- **Circuit breaker four state-machine defects** (v2.4 task-07, #68):
  (1) failures reset on Closed-state success; (2) the transitioning
  probe's success is counted (was previously skipped because
  `currentState` was captured on goroutine entry, before the
  `Open→HalfOpen` transition); (3) success and failure paths re-read
  state under the write lock at the mutation point; (4) half-open
  admits at most `HalfOpenMaxProbes` (default 1) concurrent probes.
- **`RateLimitMiddleware` XFF trust + host keying + per-key eviction**
  (v2.4 task-08, #69 — see Changed for details).
- **Validator dereferences pointer fields for non-required rules**
  (v2.4 task-09, #65). Pre-fix, every rule except `required` failed
  on non-nil `*string`/`*int`/`*float`/`*bool` fields — the canonical
  Go pattern for optional JSON fields. `email` on a valid `*string`
  yielded "email rule requires string field"; `min=18` on a valid
  `*int` yielded "min/max not supported for kind ptr". Now nil
  pointers skip all rules except `required`, and non-nil pointers are
  dereferenced so rules see the element value. `required` continues
  to operate on the pointer itself.

### Testing

- **`testDocsCorrectedClaims` drift guard** (v2.4 task-11, #73). New
  test in `docs_consistency_test.go` scans each corrected file for
  the specific pre-fix phrase, failing if a future edit reintroduces
  it. Covers eleven audit-flagged phrases spanning `README.md`,
  `docs/index.md`, `docs/guide/index.md`, `docs/guide/middleware/index.md`,
  `docs/examples/middleware-stack.md`, `docs/api/espresso.md`,
  `docs/api/state.md`, `docs/api/index.md`, `core.go`, `response.go`,
  `extractor/extractor.go`, and `openapi/openapi.go`. The guard
  immediately caught one regression PR #63 missed (`docs/api/index.md:89`
  still carried the pre-fix `GetState[T any](ctx) (T, error)` signature).

### Under the hood

- New `internal/bodylimit/` package — stdlib-only leaf carrying the
  request-body-size limit ctx key, middleware, sentinel error, and
  `ReadAllLimited` helper. Follows the `internal/errorenvelope` and
  `internal/validatehook` pattern of sharing cross-package plumbing
  without creating a new dependency direction (extractor stays a leaf
  sibling of root).
- `openapi/openapi.go` — `invalidateCache` renamed to
  `invalidateCacheLocked` to make the lock-holder contract explicit.
  Callers hold `g.mu` before invoking.

## [2.3.0] - 2026-06-29

A correctness/quality release — **Backflush** — that makes the OpenAPI
generator trustworthy and clears the deferred debt the post-v2.1 analysis
surfaced. The only new runtime surface is the additive
`openapi.AddSecurityScheme` API; everything else corrects behavior or tests.
The generated spec now reflects the registered routes accurately — custom
extractors are introspected (#52), status codes are real, security references
resolve (#49), and response schemas appear on both registration paths — the
spec endpoint is served correctly (JSON error envelope on failure, cached,
version-pinned Scalar UI; #53), the docs compile (#51), and the long-lived
WebSocket integration suite is green again (#50). The `AutoRegister` no-op
stub was removed. A consolidated spec-correctness matrix locks it all (#54).

**Upgrade from v2.2:**

- **`AutoRegister` is removed.** It was an exported no-op that did nothing;
  delete any call to it. Use `RegisterHandler` or the `OpenAPIRouter` fluent
  API. (Real route auto-registration is possible future work, not committed.)
- **Secured specs must register their schemes.** A route using
  `openapi.Security("bearerAuth")` now needs a matching
  `gen.AddSecurityScheme("bearerAuth", openapi.BearerScheme("JWT"))` (or
  `APIKeyHeaderScheme`) so the reference resolves; `UnresolvedSecurityRefs()`
  surfaces any that don't. Previously such references were emitted dangling.
- **The Scalar docs UI is pinned** to `@scalar/api-reference@1.25.122` via
  `openapi.ScalarVersion`. It still loads from an external CDN — offline /
  air-gapped / strict-CSP deployments should self-host the bundle and bump the
  pin deliberately.

### Fixed

- **OpenAPI generation correctness — five generation-path defects** (v2.3 task-01).
  The spec generator emitted specs that silently disagreed with the routes they
  described. All five fixes are in `openapi/introspect.go` and `router_openapi.go`:
  - **Custom extractors are now introspected (D2).** The custom-`FromRequest`
    probe interface was `interface{ Extract(r any) error }`, which no real
    extractor satisfies (every extractor implements `Extract(*http.Request) error`),
    so the custom-extractor branch in `Introspect` was dead and custom extractors
    contributed nothing to the spec. The probe now matches the real contract.
  - **Extractor classification is no longer prefix-fragile (D8).** `getExtractorKind`
    classified by `strings.HasPrefix` on the type name, which mis-classified any
    `Files…`-named type as a single `file` (since `"File"` is a prefix of `"Files"`)
    and changed classification silently on a rename. Classification now keys off the
    actual extractor base types (matched by package path + base name, ignoring the
    generic instantiation), referencing `extractor.PathExtractor`/`QueryExtractor`/etc.
    concretely so a rename is a compile error. `FileExtractor` and `FilesExtractor`
    are distinct base types — no more prefix ambiguity.
  - **Response status codes are documented accurately (D3).** `extractStatusCode`
    previously always returned `0` (its `//nolint:unparam` admitted as much), so every
    operation documented `200`. It now returns the documented default (`200`) and
    honors a response type that declares its status via the new optional
    `OpenAPIStatusCode() int` interface. The `//nolint:unparam` is removed.
  - **`RegisterHandler` now attaches the response-body schema (D9).** `registerPath`
    and `RegisterHandler` were drifted copy-paste twins; only the former wired the
    response schema, so `RegisterHandler` emitted a bare `200:{description:"Success"}`
    for handlers with a known response type. Both paths are now unified on a single
    `openapi.BuildPathOperation` helper that seeds the response under the handler's
    documented status code (honoring an explicit `openapi.Status("201", …)` over the
    default) and attaches the response-body schema.
  - **Introspection failures are surfaced, not silently dropped (D6).** `registerPath`
    did `if err != nil { return }`, so a route whose handler failed introspection
    vanished from the spec with no diagnostic. It now logs the failure and records it
    on the router (`(*OpenAPIRouter).Errors()`); `RegisterHandler` still returns the
    error directly.

- **Docs: corrected extractor generics to their `extractor.X[T]` package and
  added a snippet-compile guard** (v2.3 task-04). The documentation site taught
  `espresso.Path[T]`, `espresso.Query[T]`, `espresso.Form[T]`,
  `espresso.Header[T]`, and `espresso.XML[T]` as if those generics were exported
  from the root package — they are not; they live in the `extractor` package
  (type aliases in `extractor/extractor.go`). A reader copying a snippet verbatim
  hit `undefined: espresso.Path`. All such references across `docs/` (9 files) are
  now rewritten to `extractor.X[T]`, the self-contained `package main` programs
  that use them gained the `github.com/suryakencana007/espresso/v2/extractor`
  import, and two `package main` upload examples in `docs/examples/file-upload.md`
  dropped an unused `net/http` import so they compile. Root-package symbols
  (`JSON`, `Text`, `Status`, the `Err*` constructors, `WS`, `SSEStream`, the
  coffee aliases, state helpers, …) were left untouched. Two new guards lock this
  in: `TestDocsConsistency/extractor_generics_qualified_with_extractor_pkg`
  asserts no `espresso.{Path,Query,Form,Header,XML}[` reference remains in
  `docs/`, and `TestDocsSnippetsCompile` extracts every self-contained,
  extractor-importing `package main` fence from `docs/` and `go build`s it in a
  hermetic temp dir (no network) so a non-compiling published program fails CI.

- **WebSocket long-lived integration tests no longer hang on client-side
  ping** (v2.3 task-05). `TestLongLived_WS_StableConnection` and
  `TestLongLived_WS_100Concurrent` (`tests/integration/longlived_test.go`)
  dialed a `coder/websocket` connection but never read from it. Because
  `coder/websocket` has no background read pump, incoming control frames —
  including the **pong** that `conn.Ping` blocks on — are only processed from
  the read path, so the stable-connection test failed at its 3s ping deadline.
  Both tests now call `conn.CloseRead(ctx)` immediately after the dial (the
  library-documented idiom for read-less clients), starting a background reader
  that drains control frames while discarding data frames. This is a
  **test-harness fix only** — no framework behavior changed; the server-side
  `readLoop` (`websocket.go`) already drains the connection and auto-replies
  pong correctly.

- **OpenAPI serving path hardened — three serving/surface defects** (v2.3
  task-03). The generated spec was correct after tasks 1-2; these fixes make the
  *serving* of it trustworthy. All three are in `openapi/`:
  - **The spec-generation failure path emits the canonical JSON envelope, not
    `text/plain` (D1).** `Generator.Handler()` (`openapi/openapi.go`) previously
    called `http.Error(...)` on a marshal failure, producing a `text/plain` body
    with no `code`, no `request_id`, and no `{"error":{...}}` wrapper — the
    opposite of every other framework failure path. It now writes the canonical
    `{"error":{"code":"INTERNAL","message":...}}` envelope with `Content-Type:
    application/json` and a 500 status, via the stdlib-only
    `internal/errorenvelope` leaf. The `openapi` package still does **not** import
    the root `espresso` package, so no import cycle is introduced (the leaf is
    the cycle-safe path, mirroring how `middleware/http` reuses it).
  - **The marshaled spec is cached, not re-marshaled on every request (D7).**
    `Handler()` called `ToJSON()` → `json.MarshalIndent` on each hit even though
    the spec is immutable once route registration completes. The `Generator` now
    marshals lazily on the first request and serves the cached bytes thereafter,
    guarded by a mutex so concurrent serving is `-race` clean. Every mutation
    method (`Server`/`AddServer`, `AddPath`, `AddSchema`, `Schema`,
    `AddSecurityScheme`, `Description`/`SetDescription`) invalidates the cache, so
    a spec built incrementally is never served stale.

### Changed

- **The Scalar docs-UI CDN bundle is now version-pinned (D10, v2.3 task-03).**
  `openapi/scalar.go` embedded `https://cdn.jsdelivr.net/npm/@scalar/api-reference`
  with no `@version`, resolving to `latest` — so a breaking Scalar release could
  silently break the docs page. It is now pinned to a concrete published version
  via the exported `openapi.ScalarVersion` constant
  (`@scalar/api-reference@1.25.122`), so the pin is bumped deliberately in a
  dedicated release rather than left to float. A godoc note on `ScalarVersion`
  and `ScalarUIHandler` documents that the bundle still loads from an external
  CDN, so offline / air-gapped / strict-CSP deployments should self-host the
  bundle and point their docs HTML at the local copy (the spec is already
  referenced via a relative data-url, so only the bundle URL needs repointing).

### Removed

- **`AutoRegister` no-op stub removed** (v2.3 task-03, D5). The exported
  `espresso.AutoRegister(gen, router, optsMap)` in `router_openapi.go` was an
  **empty no-op** whose godoc described it in detail as registering every route
  on a `Router` into the spec — the API promised behavior it never had, so
  callers wired it up and got nothing. The symbol and its misleading godoc are
  deleted so the surface stops lying. This is an API removal, but no behavior is
  lost (it did nothing): callers of the no-op simply delete the call. Genuine
  route auto-registration — walking a live `*Router` and introspecting each route
  into the spec — is **possible future work, not a v2.3 commitment**; use
  `RegisterHandler` or the `OpenAPIRouter` fluent API in the meantime.

### Added

- **`openapi.Generator.AddSecurityScheme(name, scheme)` registers security
  schemes so operation-level `Security(...)` references resolve** (v2.3
  task-02). Previously `components.securitySchemes` was allocated empty and
  never populated, so a route decorated with `openapi.Security("bearerAuth")`
  emitted a dangling reference: the operation pointed at `bearerAuth`, but no
  such scheme was defined anywhere in `components`. The spec failed strict
  OpenAPI 3.0 validation and the Scalar/Swagger "Authorize" button had nothing
  to render. `AddSecurityScheme` populates `components.securitySchemes[name]`
  with a `SecurityScheme` (a minimal subset of the OpenAPI 3.0 Security Scheme
  Object: `type`, `scheme`, `bearerFormat`, `in`, `name`, `description`). Two
  ergonomic constructors cover the common cases:
  - `openapi.BearerScheme("JWT")` → `{type:"http", scheme:"bearer", bearerFormat:"JWT"}`
  - `openapi.APIKeyHeaderScheme("X-API-Key")` → `{type:"apiKey", in:"header", name:"X-API-Key"}`

  Example:

  ```go
  gen.AddSecurityScheme("bearerAuth", openapi.BearerScheme("JWT"))
  gen.AddSecurityScheme("apiKeyAuth", openapi.APIKeyHeaderScheme("X-API-Key"))
  ```

  `Generator.UnresolvedSecurityRefs()` surfaces (sorted, de-duplicated) any
  scheme name referenced by an operation but never registered, so a
  `Security("name")` typo is flagged rather than emitted as a silent dangling
  reference. OAuth2 flows and OpenID Connect are intentionally out of scope.

### Internal

- Added the consolidated OpenAPI spec-correctness matrix
  (`TestOpenAPISpecCorrectnessMatrix`, tests only) that builds one router across
  both registration paths and locks the combined v2.3 generation/serving behavior
  (extractors, real status codes, resolvable security refs, response schemas, no
  dropped routes, the JSON failure envelope, cache-stable serving), plus an
  `openapi`-does-not-import-root import-direction guard (v2.3 task-06).

## [2.2.0] - 2026-06-28

A correctness release — **Dial It In** — that makes Espresso's behavior
match its documented and expected contract; no new feature surface.
Service-layer errors now map to the right HTTP status (#42), every
framework error path emits the one canonical JSON envelope (#43), and the
reflection dispatch path rejects an unsupported two-extractor signature at
registration instead of panicking per-request (#41) — all backed by new
status-code / handler-signature / doc-consistency verification matrices
(#44). See the per-change **Upgrade from v2.1** notes below before bumping.

### Changed

- **Service-layer errors now map to their contract HTTP status instead of
  collapsing to 500** (v2.2 task-02). `writeHandlerError` previously
  special-cased only `*espresso.Error`; every other error surfaced through
  `WithLayers` / `WithLayersTyped` was wrapped as `ErrInternal` → 500. A new
  `translateLayerError` step (in `error.go`, run after the `*espresso.Error`
  fast path and before the 500 fallback) recognizes the three built-in layer
  failure modes:
  - **ValidationLayer** (`servicemiddleware.ErrValidation`): `500 INTERNAL`
    → `400 VALIDATION_ERROR`. When the validator returns
    `espresso.FieldErrors`, the field detail is preserved under
    `details.errors`; otherwise the validator's message is preserved.
  - **CircuitBreaker open** (`*servicemiddleware.CircuitBreakerError`):
    `500 INTERNAL` → `503 SERVICE_UNAVAILABLE`.
  - **TimeoutLayer deadline** (`context.DeadlineExceeded`): `500 INTERNAL`
    → `503 SERVICE_UNAVAILABLE`.

  Handler-returned `*espresso.Error` values (e.g. `ErrConflict`) keep their
  explicit status — the fast path runs first and is not shadowed. Unknown /
  unrecognized errors still map to `500 INTERNAL` (fallback unchanged). The
  auto-validate-on-extract path (which already returns 400 via `ErrBadRequest`)
  was left untouched. Every mapped error continues to emit the canonical
  `{"error":{"code","message","details","request_id"}}` envelope.

  **Upgrade from v2.1:** clients that key off HTTP 500 to detect a failed
  *service-layer* validation, open circuit breaker, or layer timeout must
  switch to 400 (validation) / 503 (circuit-breaker, timeout). Handler-returned
  `*espresso.Error` responses and genuinely unknown errors are unaffected.

- **Every framework-produced error now emits the canonical JSON envelope**
  (v2.2 task-03). Three error paths previously bypassed the
  `{"error":{"code","message","details","request_id"}}` shape that
  `writeErrorResponse` produces for extractor / handler / SSE / WebSocket
  failures:
  - **Auth rejection (401)** — `JWTMiddleware` / `BasicAuthMiddleware` /
    `APIKeyMiddleware` (plus the generic `AuthMiddleware`) emitted
    `text/plain` `"Unauthorized…"` via `http.Error`. They now emit the JSON
    envelope with code `UNAUTHORIZED` and `request_id`, preserving the
    previous message text inside `message` (e.g.
    `{"error":{"code":"UNAUTHORIZED","message":"Unauthorized: invalid token","request_id":"…"}}`).
    The `WWW-Authenticate` header on Basic-auth challenges is unchanged.
  - **Rate-limit rejection (429)** — `RateLimitMiddleware` emitted
    `text/plain` `"Too Many Requests"`. It now emits the envelope with code
    `TOO_MANY_REQUESTS` and `request_id`.
  - **Panic recovery (500)** — `RecoverMiddleware` hand-rolled an
    anonymous-struct JSON (code `PANIC`) that omitted the `details` key. It
    now routes through the shared writer, so the body is byte-identical to a
    `writeHandlerError` 500: `details` is omitted (nil → `omitempty`),
    matching the canonical 500 exactly.

  These three paths and the root package's `writeErrorResponse` now share a
  single stdlib-only leaf package, `internal/errorenvelope`, so the wire
  format cannot drift between them. The root → `middleware/http` import
  direction is preserved (the leaf is importable by both without a cycle);
  `middleware/http` still does not import the root package. No API signatures
  changed.

  **Upgrade from v2.1:** callers that parsed the old `text/plain` 401 / 429
  bodies (e.g. matching the literal strings `"Unauthorized"` /
  `"Too Many Requests"`, or branching on a `text/plain` content type) must
  switch to parsing the JSON envelope and reading `error.code`
  (`UNAUTHORIZED` / `TOO_MANY_REQUESTS`). The 500 panic body is unchanged in
  practice (it already lacked `details`).

### Fixed

- **Two-extractor reflection handlers now fail fast at registration**
  (v2.2 task-01): registering `func(ctx, *Req1, *Req2) (T, error)` via the
  reflection path (`Handler` / `router.Get/Post/Handle`) previously
  succeeded silently — `handlerInfo` carries a single request slot, so the
  second `FromRequest` arg clobbered the first and every request hit the
  defensive `panic("espresso: invalid handler argument - this is a bug")`
  in `createHandlerFromInfo`, surfacing as an HTTP 500 under
  `RecoverMiddleware`. The registration-time argument loop in `handlerFunc`
  now counts `FromRequest` arguments and panics immediately (at startup,
  not per-request) with an actionable message pointing to the typed
  `HandlerCtxReq1Req2Err` constructor (or its `Lungo` alias). This aligns
  with the framework's "panic at registration, not request" philosophy and
  makes the "this is a bug" panic unreachable for any registrable
  signature. The typed `HandlerCtxReq1Req2Err` / `HandlerCtxReq1Req2` /
  `Lungo` / `LungoNoErr` two-extractor path is unchanged.

### Documentation

- **`docs/guide/testing.md`** — new page documenting the recommended
  test-seam pattern for services with unexported function-field
  dispatch. Recommends private functional options keyed through
  `_test.go`-only setters so `*ForTest` setters don't leak into the
  production API surface or appear on `pkg.go.dev`. Includes a "when
  to promote to an interface seam instead" decision rule (4+ stubs
  across test files). Wired into the docs sidebar under Advanced.
  Closes the last open friction item from Barista's feedback log
  (F-08, see [`roadmaps/USAGE_ESPRESSO.md`](roadmaps/USAGE_ESPRESSO.md#f-08))
  — application-layer pattern, framework ships guidance not API.

### Internal

- **Verification matrices** (v2.2 task-04, #44): added table-driven
  `TestErrorStatusMatrix` (asserts HTTP status + the canonical envelope for
  every error origin — extractor, handler, the three service-layer errors,
  panic, auth, rate-limit), `TestHandlerSignatureMatrix` (every supported
  reflection / typed / coffee-alias handler shape, plus the two-extractor
  registration panic and a guard that the request-time "this is a bug" panic
  is unreachable), and `TestDocsConsistency` (guards against re-introducing
  removed SSE symbols or the false two-extractor godoc claim). Tests only; no
  production change.

## [2.1.0] - 2026-05-12

A maintenance release that pays off v2.0's deferred SSE-removal debt
(#30), closes Barista F-02 with a new `WithPreFlight` Stream option
(#33), and ships one validator ergonomics helper (#31). Plus a bench
refresh against current hardware/Go-version (#32) and a v2.0 → v2.1
migration guide (#34). See
[`docs/migration-v2-to-v2.1.md`](docs/migration-v2-to-v2.1.md) for
upgrade recipes.

### Added

- **`validator.AsDefaultValidator()` helper** (v2.1 task-03): returns
  the canonical `func(any) error` adapter for
  `espresso.SetDefaultValidator(...)`. Wraps `validator.Struct` and
  converts the resulting `espresso.FieldErrors` into
  `espresso.ValidationErrors` so failures surface as the framework's
  standard 400 JSON shape. Drops the auto-validate wiring in user
  code from ~10 lines to 1:
  ```go
  func init() {
      espresso.SetDefaultValidator(validator.AsDefaultValidator())
  }
  ```
  Users who need a different error code, extra detail keys, or other
  customization keep writing the inline closure — the helper is the
  most-common-case shortcut, not a configuration surface.

- **`WithPreFlight(fn)` Stream option** (v2.1 task-02, closes Barista
  F-02): SSE handlers can reject a request with a real HTTP 4xx
  *before* the response headers commit. Pass any
  `func(ctx context.Context) error` to `Stream[T]` or `StreamSimple`;
  a non-nil return routes through `writeHandlerError`, so an
  `*espresso.Error` (e.g. `ErrNotFound`, `ErrForbidden`) surfaces with
  its declared status code and the framework's structured JSON
  envelope — not as an `event: error` frame on a 200-OK stream. The
  closure receives the request context and can call
  `MustGetState[T]` / `GetState[T]`. Closes
  [USAGE_ESPRESSO.md F-02](roadmaps/USAGE_ESPRESSO.md#f-02) — the
  Barista per-route `RequireAppAccess` / `RequireDeploymentAccess`
  preflight middleware collapses into a single
  `WithPreFlight(...)` call. Additive: existing `Stream[T]` /
  `StreamSimple` callers see no behavioural change and pay zero
  overhead on the happy path. See
  [`roadmaps/v2.1/tasks/task-02-stream-preflight.md`](roadmaps/v2.1/tasks/task-02-stream-preflight.md).

### Removed (BREAKING)

- **Deprecated SSE types removed from `response.go`** (v2.1 task-01):
  `SSE`, `SSEEvent`, `SSEWriter`, `NewSSEWriter`, and their methods.
  Tagged `// Deprecated:` since v1.3; carry-over from v2.0 task-02
  deferral. Replace with `Stream[T]` / `StreamSimple` and `*SSEStream`:
  ```go
  // Before (v2.0)
  sse := espresso.NewSSEWriter(w)
  sse.Event("update", "hello")

  // After (v2.1)
  router.Get("/stream", espresso.StreamSimple(func(ctx context.Context, s *espresso.SSEStream) error {
      return s.SendText("update", "hello")
  }))
  ```
  Net diff: -185 lines from `response.go`, -178 lines from
  `response_test.go` (ten SSE/SSEWriter tests deleted). The `fmt`
  import is now unused in `response.go` and dropped. Docs API
  reference (`docs/api/espresso.md`) updated — the deprecated
  sections that v2.0 PR #26 left in with warning banners are now
  removed entirely; a one-paragraph forward pointer to the streaming
  guide replaces them.

### Documentation

- **v2.0 → v2.1 migration guide** (v2.1 task-05): new
  [`docs/migration-v2-to-v2.1.md`](docs/migration-v2-to-v2.1.md)
  covering the four user-facing v2.1 deltas — the removed deprecated
  SSE types (PR #30), the new `WithPreFlight` Stream option (PR #33,
  closes Barista F-02), the `validator.AsDefaultValidator()` helper
  (PR #31), and the framework-comparison bench refresh (PR #32). Five-
  minute upgrade checklist at the top in the same style as the v1 → v2
  guide; Before/After recipes for each change. Docs nav
  (`docs/.vitepress/config.ts`) gains a "Migrations" dropdown at the
  top level and a second entry under the "Upgrading" sidebar group so
  readers on either v1.x or v2.0 land in the right guide. README
  `Upgrading` section gains a parallel paragraph for the v2.0 → v2.1
  jump. Cross-references added from `roadmaps/v2.0/README.md` and
  `roadmaps/v2.1/README.md` to the new guide. Pure documentation; no
  behavioural change.

- **Framework comparison benchmarks refreshed against v2.1.x**
  (v2.1 task-04). Re-ran `bench/` (Gin / Echo / Espresso / Fiber across
  static-text, JSON round-trip, and path-parameter scenarios) on
  Go 1.25.6 / AMD Ryzen 7 4800H with
  `go test -bench . -benchmem -benchtime=3s -count=3 -cpu=1`, then
  substituted the mean values into the README's three "Framework
  Comparison" tables. Caption updated to record run conditions
  (hardware, Go version, commit, command line). The new numbers run
  ~2-3x higher than the v1.4 publication across **all four**
  frameworks — this is a benchmark-runner hardware shift (the v1.4
  baseline was an Intel Core Ultra 7 155H), not an Espresso dispatch
  regression. Allocations and B/op are essentially unchanged for the
  competitor frameworks and only marginally up for Espresso, which
  rules out an allocation regression. Added a paragraph to
  `bench/README.md` capturing the hardware-shift explanation so
  future readers don't mistake the absolute numbers for a v2.x
  slowdown. Purely informational; no behavioral change to Espresso.

## [2.0.0] - 2026-05-10

The first major release since v1.0. Bundles five breaking changes and
two additive features that v1.x deferred under its strict
backward-compatibility promise. Every breaking change has a mechanical
migration recipe in
[`docs/migration-v1-to-v2.md`](docs/migration-v1-to-v2.md); the full
upgrade for a typical app fits in one sitting.

### Changed (BREAKING)

- **Module path changed** to
  `github.com/suryakencana007/espresso/v2` per Go's major-version
  module convention. Update imports:

  ```bash
  gofmt -r '"github.com/suryakencana007/espresso" -> "github.com/suryakencana007/espresso/v2"' -w .
  # plus the same rewrite for each subpackage (extractor, middleware/http, etc.) — see migration guide.
  ```

- **`espresso.Validation` is now generic** (v2.0 task-04). Signature
  changed from `Validation(validator any) LayerConfig` to
  `Validation[Req any](validator servicemiddleware.Validator[Req]) LayerConfig`.
  Closes the v1.x footgun where a mismatched validator failed silently
  (the type assertion in `buildLayer` returned a panic only when the
  pipeline actually built; before that, the assertion result was
  invisible). Now: a `Validator[Req1]` applied to a handler with `Req2`
  panics at registration with a descriptive message naming both types.

  Migration: most callers don't need a syntactic change because Go
  infers the type parameter from the argument:

  ```go
  validator := servicemiddleware.ValidatorFunc[*JSON[CreateUserReq]](...)
  Validation(validator)            // Req inferred as *JSON[CreateUserReq]
  Validation[*JSON[CreateUserReq]](validator)  // explicit form, optional
  ```

  The explicit form is recommended for documentation purposes when the
  validator is constructed elsewhere and the call site benefits from a
  visible type at the binding point. New regression-locking test:
  `TestValidation_TypeMismatch_PanicsAtBuild`. Internal `validationConfig`
  is now `validationConfig[Req]` (unexported, no external migration).

### Added

- **Auto-validate on extract** (v2.0 task-05). New package-level hook
  `espresso.SetDefaultValidator(fn func(any) error)`. When set, every
  built-in extractor — `JSON[T]`, `Query[T]`, `Path[T]`, `Form[T]`,
  `Header[T]`, `Cookie[T]`, `XML[T]`, `Multipart[T]`,
  `RawBodyWithHeaders[H]` — calls the hook with a pointer to the decoded
  value at the end of `Extract`. A non-nil error is propagated as an
  extraction failure, which the existing structured-JSON 400 path handles;
  the handler does not run.
  Pair with `validator.Struct` for one-line wiring of struct-tag
  validation:
  ```go
  func init() {
      espresso.SetDefaultValidator(func(v any) error {
          if err := validator.Struct(v); err != nil {
              if fe, ok := err.(espresso.FieldErrors); ok {
                  return espresso.ValidationErrors(fe.ToValidationErrors())
              }
              return err
          }
          return nil
      })
  }
  ```
  Default is nil — v1.x behavior is preserved exactly when the hook is
  unset. Hot-path overhead measured at **2.24 ns/op, 0 allocs** for the
  nil-fast path (`BenchmarkRunDefaultValidator_NilHook`) and 2.62 ns/op
  with a hook installed. `RunDefaultValidator(v any) error` is also
  exported so custom `Extract` methods can opt into the same hook.
  New example at `cmd/example/validate/main.go`. Composable with the
  existing `Validation[Req]` service layer (which runs **after**
  extraction); the two solve different problems and can be used together.

  Implementation note: the hook lives in `internal/validatehook` so both
  the root `espresso` package and the `extractor` subpackage can depend
  on it without forming an import cycle.

- **Handler-reflection cache is now bounded with LRU eviction**
  (v2.0 task-03). Default upper bound is `DefaultHandlerCacheSize`
  (1024). Tuning surface added:
  ```go
  espresso.SetHandlerCacheSize(2048)
  espresso.OnHandlerCacheEvict(func(t reflect.Type) {
      metrics.Inc("handler_cache.evict", "type", t.String())
  })
  ```
  Static apps stay well under the default and never evict — the LRU
  bookkeeping adds ~24 ns/registration (measured) and does not touch
  the per-request hot path. Dynamic-registration scenarios (plugin
  hosts, per-tenant codegen, `reflect.MakeFunc`) now have an upper
  bound on cache memory regardless of churn rate. In-flight requests
  are unaffected by eviction: `*handlerInfo` values are immutable, and
  request-side handlers hold the pointer captured at registration time.
  Replaces the previous unbounded `sync.Map`. New benchmarks:
  `BenchmarkHandlerCache_SteadyState` (24 ns/op, 0 allocs),
  `BenchmarkHandlerCache_Overflow` (224 ns/op for insert+evict+hook).
  This is **purely additive** — apps that don't call the new setters
  see only the bound (1024) and identical hot-path behavior.

### Changed (BREAKING)

- **Stream registries are now per-Router instead of process-global**
  (v2.0 task-01). The package-level `defaultRegistry` (WebSocket) and
  `defaultSSERegistry` (SSE) are removed; each `*Router` owns a
  `wsReg` and `sseReg` initialized in `Portafilter()`. `gracefulShutdown`
  drains the owning Router's registries, so two `Portafilter()` instances
  in the same process now shut down independently — closing router A
  does not touch router B's streams.
  Internally, `*Router.ServeHTTP` injects the registries into the
  request context; `WebSocketSimple` / `WebSocket[T]` / `StreamSimple` /
  `Stream[T]` look them up via `routerRegistriesFrom(ctx)` and register
  the connections they create. If the wrappers are invoked outside a
  Router context (e.g., wired into a non-Espresso mux), registration is
  a silent no-op — the connection still works, but graceful-shutdown
  won't drain it.
  Migration: external callers of the package-level `defaultRegistry` /
  `defaultSSERegistry` must reach into `router.wsReg` / `router.sseReg`
  on a specific `*Router` instance instead. Internal users (handlers
  created via `router.Get(...)` etc.) need no change. Adds
  `TestShutdown_MultiRouter_WebSocketIsolation` and
  `TestShutdown_MultiRouter_SSEIsolation` regression locks.

- **`Ristretto[Res]` signature changed** from `func() Res` to
  `func(context.Context) Res`. Closes Barista F-01 — `Ristretto` was
  pitched as the lightweight health-check primitive but couldn't reach
  state via `MustGetState[T]` since it had no `context.Context`. The new
  signature delegates to `HandlerCtxNoErr` (instead of `HandlerNoReqNoErr`)
  and preserves the no-error character that distinguishes `Ristretto`
  from `Doppio` / `HandlerCtx`.
  Migration: add a `context.Context` parameter to every Ristretto handler.
  If you don't need it, name it `_`:
  ```go
  // Before
  func ping() Text { return Text{Body: "pong"} }
  router.Get("/ping", espresso.Ristretto(ping))

  // After
  func ping(_ context.Context) Text { return Text{Body: "pong"} }
  router.Get("/ping", espresso.Ristretto(ping))
  ```
  If your handler needs to return an error, use `Doppio` or `HandlerCtx`
  instead — `Ristretto` keeps its no-error stance.
  Mechanical search-and-replace isn't safe for this one (function literal
  parameter signature varies); a one-time `go build` flags the call sites.
  If you want a 0-arg handler with no error and no context, use
  `HandlerNoReqNoErr` (still available, unchanged).

### Removed (BREAKING)

- **Legacy error constructors** removed from `error.go`: `BadRequest`,
  `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `InternalError`,
  `ServiceUnavailable`. These were marked "prefer `Err*` for new code"
  in their godoc since v1.0 but lacked the machine-readable
  `// Deprecated:` tag, so v1.x callers got no migration runway. They
  were originally scoped for v2.0 (`roadmaps/v2.0/tasks/task-02`); folded
  forward into pre-v2.0 cleanup. Migration: rename per the table —
  `BadRequest` → `ErrBadRequest`, etc. The replacement constructors
  have shipped since v1.0; mechanical rename via
  `gofmt -r 'espresso.BadRequest(x) -> espresso.ErrBadRequest(x)' -w .`
  (and similarly for the other six).
- **`ErrorResponse` type alias** for `Error` removed. Migration:
  use `*espresso.Error` directly (the alias was a thin pass-through).
- **Dead handlers in `cmd/example/main.go`** (`createUserWithError`,
  `circuitBreakerExample`) removed. They were `//nolint:unused`-flagged
  and used the legacy constructors above.
- **Backward Compatibility section in `docs/error-handling.md`**
  removed — the constructors it described are gone.

### Changed

- **Test coverage table in `README.md` refreshed** with current numbers
  (Root: 80.8%, extractor: 85.5%, middleware/http: 86.9%, validator:
  80.6%, openapi: 77.3%; the latter two were missing from the table).

### Still present, deprecated

The following types remain in v2.0 with `// Deprecated:` markers and are
targeted for removal in v2.1. Existing callers continue to work
unchanged; `staticcheck SA1019` will flag them when you're ready to
migrate.

- `SSE`, `SSEEvent` — use `Stream[T]` / `StreamSimple` and `*SSEStream`.
- `SSEWriter`, `NewSSEWriter` — use `*SSEStream`'s `SendText` / `SendJSON`
  / `SendData` / `Comment`.

## [1.5.0] - 2026-05-10

### Added

- **`JSON[T].Cookies` field** — set HTTP cookies alongside JSON
  responses. Cookies are written via `http.SetCookie` before the
  status header, ensuring `Set-Cookie` lands in the response head.
  Zero-value (`nil`) `Cookies` is byte-identical to v1.4. `Reset()`
  clears the slice for `sync.Pool` reuse. Closes Barista F-05.

- **`extractor.RawBodyWithHeaders[H]`** — new extractor that reads the
  raw request body alongside structured headers in a single pass.
  Designed for webhook receivers that verify HMAC against the
  unparsed payload (GitHub `X-Hub-Signature-256`, GitLab `X-Gitlab-Token`,
  Stripe, Slack, etc.). `H` uses the existing `header:"Name,required"`
  tag convention. `Reset()` releases buffers larger than 64KB to
  prevent pool memory bloat, mirroring `RawBodyExtractor`.
  Closes Barista F-06.

- **`ErrPreconditionFailed(message string)`** — 412 Precondition Failed
  constructor, completing the symmetry with the other status-keyed
  helpers. Use when a request precondition is not met (missing
  prerequisite infrastructure, If-Match mismatch, required feature
  flag disabled). Closes Barista F-07.

- **`/api/login` and `/api/webhook/github` examples** in
  `cmd/example/main.go` demonstrate the refresh-token cookie pattern
  and HMAC-SHA256 webhook verification respectively.

### Migration Notes

v1.5 is **strictly additive**. No v1.4 caller is affected — every
public type, function, and method retains its v1.4 signature and
behavior.

If you wrote a chart-internal `JSONWithCookies[T]` wrapper (the
pattern Barista shipped as `httpx.JSONWithCookies[T]`), you can now
retire it:

```go
// Before
return httpx.JSONWithCookies[Token]{
    Data:    Token{Access: t},
    Cookies: []*http.Cookie{refreshCookie},
}

// After
return espresso.JSON[Token]{
    Data:    Token{Access: t},
    Cookies: []*http.Cookie{refreshCookie},
}
```

If you wrote a custom `webhookRequest`-style extractor for "raw body +
provider header", switch to `extractor.RawBodyWithHeaders[H]`. Define
a struct with `header:"X-Foo,required"` tags and use the generic.

If you used `NewError(http.StatusPreconditionFailed, msg).WithCode(...)`,
swap for `ErrPreconditionFailed(msg)`.

## [1.4.0] - 2026-04-20

### Added

- **`validator/` subpackage** — struct-tag-driven validation via
  `validator.Struct(v any) error`. Returns `espresso.FieldErrors` that flow
  into the existing structured-error pipeline. Built-in rules: `required`,
  `min`, `max` (numeric value or string/slice/map length), `email`, `url`,
  `regex`, `oneof`. Recurses into nested structs, pointers to structs, and
  slices of structs with path tracking. See `docs/guide/validation.md` and
  `docs/api/validator.md`. 16 unit tests.

- **`bench/` framework-comparison module** — separate Go module (replace
  directive back to the parent) so comparison deps don't leak into the main
  module. Head-to-head benchmarks against Gin, Echo, and Fiber on three
  scenarios (static text, JSON round-trip, path parameter). Results tabled
  in `README.md` under "Framework Comparison".

- **`TestWebSocket_GracefulShutdown`** — end-to-end test verifying
  connected WebSocket clients receive close code 1001 via
  `router.gracefulShutdown`.

- **`TestShutdown_WebSocketsClosed`** — mirrors the existing SSE shutdown
  test; covers registry close-all on shutdown.

- **`TestSSE_Stream_StateInjection`** — covers state injection through the
  `Stream[Req]` variant (previously only `StreamSimple` had a state test).

- **`TestWithLayers_ExtractorErrorReturnsStructuredJSON`** — locks the JSON
  error shape so regressing back to `http.Error` text/plain would fail CI.

- **v2.0 roadmap** — `roadmaps/v2.0/` mirroring the v1.3 layout. Scopes
  per-Router registries, deprecated-API removal, handler-cache eviction,
  typed `Validation[Req]`, opt-in auto-validate, migration guide, release.

- **Handler-cache growth documentation** — `handler.go` now explains the
  cache's growth semantics; `docs/performance.md` carries the same under
  "Known Limitations".

### Changed

- **Extractor failures now produce structured JSON error responses**
  instead of `text/plain` via `http.Error()`. Paths affected: `withlayers.go`
  extractor and service-call failures, `sse.go` extract/flusher failures,
  `websocket.go` extract and upgrade failures. Response now uses the same
  shape as handler errors: `{"error":{"code":"BAD_REQUEST",...}}`. Clients
  parsing 4xx bodies as text will need to update — most treat 4xx as error
  regardless of body shape.
- **Unified `serveStream` / `serveStreamSimple`** — the ~90% duplicated
  transport code in `sse.go` is now a single helper that both `Stream[Req]`
  and `StreamSimple` delegate to via a closure.
- **`WS.closed` is now `atomic.Bool`** — previously a plain `bool` read
  without the mutex in two call sites (handler wrapper end-of-func guards).
  `Close` is idempotent via CAS.
- **`WS.Close` is idempotent and always removes from the registry** —
  previously a leaked registry entry when the client disconnected before
  the handler explicitly closed.
- **`WS.readLoop` channel sends guarded by `ctx.Done()`** — previously
  could block indefinitely if the handler already returned and nothing was
  reading `msgCh`.
- **`go.mod` go directive lowered from 1.25.6 to 1.23** — verified
  codebase + full test suite + lint pass under 1.23. Widens supported
  toolchain range.

### Fixed

- **Data race on `WS.closed`** caught by `go test -race`. The two
  plain-bool reads in the handler wrappers now go through the atomic API.
- **Gosec G115** — two test-only `int → rune` overflow conversions in
  `handler_test.go` and `sse_test.go` replaced with `strconv.FormatInt`
  and a slice lookup.

### Removed

- **`(*Router).Routes() []Route` and the `Route` type** — the method
  always returned `nil` (documented as "ServeMux doesn't expose routes").
  No caller inside the repo; any external caller was getting no data.
  Migration: delete the call. If you need route introspection, track it
  yourself at registration time.
- **`closeErr` field on `*WS`** — was set in `readLoop` but never read.
  Unexported, no migration needed.

### Migration Notes

v1.4 is **mostly** backward compatible with v1.3:

- The one wire-format change is extractor failures returning JSON instead
  of text/plain. Handler-returned errors already produced JSON in v1.3;
  this change just brings the extractor path into line. Most clients treat
  4xx as an error regardless of body shape, so no action is usually needed.

- `Routes()` is gone. It returned `nil` in v1.x so no working code
  depended on its output — just remove the call.

To adopt the new validator:

```go
import "github.com/suryakencana007/espresso/validator"

type CreateUserReq struct {
    Name  string `json:"name"  validate:"required,min=3,max=50"`
    Email string `json:"email" validate:"required,email"`
}

// In your handler:
if err := validator.Struct(req.Data); err != nil {
    fe := err.(espresso.FieldErrors)
    return zero, espresso.ValidationErrors(fe.ToValidationErrors())
}
```

See `docs/guide/validation.md` for the full pattern, including service-layer
integration and custom-rule composition.

## [1.3.0] - 2026-04-19

### Added

- **WebSocket handler support** with the new `espresso.WS` type and
  `espresso.WebSocket[T]()` / `espresso.WebSocketSimple()` wrappers. Supports
  text and binary frames, ping/pong keepalive, state injection, context
  cancellation on disconnect, and graceful shutdown integration.
  Uses `github.com/coder/websocket` as the underlying library.
  Includes `cmd/example/websocket/` example.

- **Typed SSE streaming** with the new `espresso.SSEStream` type and
  `espresso.Stream[T]()` / `espresso.StreamSimple()` handlers. Supports
  typed events, JSON streaming, keepalive pings, Last-Event-ID resumption,
  state injection, concurrent-safe writes, and graceful shutdown integration.
  Includes `cmd/example/sse/` example.

- **Structured error responses** with the new `espresso.Error` type and
  fluent builder API (`WithCode`, `WithDetail`, `WithDetails`, `Wrap`).
  Handler errors are now automatically serialized as `{"error": {"code": ..., "message": ..., ...}}`.
  Includes `ErrBadRequest`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`,
  `ErrConflict`, `ErrUnprocessableEntity`, `ErrTooManyRequests`, `ErrInternal`,
  `ErrServiceUnavailable` constructors. Backward-compatible `BadRequest()`,
  `NotFound()`, etc. constructors now return `*Error`.
  `ErrorResponse` is now a type alias for `Error`.
  `RecoverMiddleware` now returns structured JSON with `"PANIC"` code
  and stack trace logging.

- **Graceful shutdown hooks** with `router.OnShutdown(hook)` for registering
  cleanup functions that run before the server stops. Hooks run in registration
  order, each receiving a context with the shutdown timeout. Hook panics and
  errors are logged but don't block subsequent hooks. The shutdown sequence is:
  1) OnShutdown hooks, 2) SSE streams close, 3) WebSockets close (code 1001),
  4) HTTP server stops accepting connections, 5) in-flight requests drain.
  Added `BrewContext(ctx, opts)` for programmatic server lifecycle control.

- **CircuitBreakerError** type for circuit breaker integration with
  `IsCircuitBreakerError()` helper.

- **FieldError/FieldErrors** types for structured validation errors with
  convenience constructors: `RequiredFieldError`, `InvalidTypeError`,
  `RangeError`, `LengthError`, `PatternError`, `CustomValidationError`.

- Context cancellation tests verifying propagation to SSE and WebSocket
  handlers, and goroutine leak detection across 50 connect/disconnect cycles.

- Integration tests (build tag: `integration`) for long-lived SSE and
  WebSocket connection stability under load.

### Changed

- Handler error responses now produce structured JSON
  (`{"error": {"code": ..., "message": ...}}`) instead of plain text via
  `http.Error()`. Handlers returning `*espresso.Error` use the error's
  status code and fields; plain `error` returns produce a generic 500 response.
- `RecoverMiddleware` now returns structured JSON error responses with stack
  trace logging and request ID inclusion, instead of plain text.
- SSE keepalive goroutine now properly synchronizes with the handler goroutine
  to avoid data races.

### Deprecated

- `SSEWriter` low-level API is deprecated in favor of `SSEStream`.
  Will be removed in v2.0. No removal in v1.x.
- `SSE` and `SSEEvent` types in `response.go` are deprecated in favor of
  `SSEStream` and `Event` types in `sse.go`.

### Migration Notes

v1.3 is backward compatible with v1.2. No code changes required to upgrade.

To adopt new features:

- Replace manual SSE handling with `espresso.Stream[T]()` or `espresso.StreamSimple()`
  for better integration, concurrency safety, and graceful shutdown support.
- Use `*espresso.Error` in handlers for consistent JSON error responses.
- Register cleanup hooks with `router.OnShutdown(fn)` for graceful resource cleanup.
- Use `router.BrewContext(ctx, opts)` for programmatic server lifecycle control
  (useful in tests or embedding).

[Unreleased]: https://github.com/suryakencana007/espresso/compare/v1.4.0...HEAD
[1.4.0]: https://github.com/suryakencana007/espresso/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/suryakencana007/espresso/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/suryakencana007/espresso/compare/v1.1.0...v1.2.0

## [1.2.0] - 2025-04-09

### Added

- **Fluent API for OpenAPI** - Modern chainable interface for OpenAPI generation
  - `openapi.New()` as primary entry point (replaces `NewGenerator()`)
  - `Description()` method replaces `SetDescription()`
  - `Server()` method replaces `AddServer()`
  - `Schema()` method for automatic type generation and registration
  - `JSON()` convenience method for spec export
  - Full backward compatibility with deprecated old methods

- **Handler Introspection** - Automatic extraction of handler type information
  - `Introspect()` analyzes handler signatures to detect parameter types
  - `BuildOperation()` creates OpenAPI operations from introspected handlers
  - `GeneratePathParams()`, `GenerateQueryParams()`, `GenerateRequestBody()` helpers
  - Automatic detection of `Path[T]`, `Query[T]`, `JSON[T]`, etc. extractors
  - Support for response type introspection

- **Operation Options** - Fine-grained control over OpenAPI operations
  - `Tags()` for grouping endpoints
  - `Summary()` and `Description()` for documentation
  - `Security()` for authentication requirements
  - `Status()` for response codes
  - `Deprecated()` for marking deprecated endpoints
  - `AddParam()` and `AddResponse()` for custom parameters/responses

- **OpenAPIRouter** - Seamless integration of routing and documentation
  - `espresso.OpenAPI(gen)` wraps router with automatic OpenAPI generation
  - `Get()`, `Post()`, `Put()`, `Delete()`, etc. register both route and OpenAPI docs
  - Automatic introspection of handler parameters and return types
  - Works with Espresso's fluent API and middleware chains

- **ServeOpenAPI Methods** - Clean integration for serving documentation
  - `ServeOpenAPI()` serves OpenAPI spec as JSON
  - `ServeDocs()` serves Scalar UI documentation interface
  - `ServeOpenAPIWithDocs()` serves both in one call
  - All integrated into router middleware chain

- **Comprehensive Tests** - 104 new tests for OpenAPI functionality
  - Tests for introspection and type extraction
  - Tests for operation options and building
  - Tests for OpenAPIRouter integration
  - Tests for ServeOpenAPI methods

### Changed

- **OpenAPI API Improvements** - Better developer experience
  - Old methods (`NewGenerator`, `SetDescription`, `AddServer`) marked as deprecated
  - Backward compatible: deprecated methods still work
  - Chainable methods enable fluent programming style

### Documentation

- Updated `docs/api/openapi.md` with fluent API examples
- Updated `docs/examples/production.md` with new patterns
- Updated `README.md` OpenAPI section
- Updated `cmd/example/main.go` to demonstrate OpenAPIRouter best practices

### Migration Guide

From v1.1.0 to v1.2.0:

```go
// v1.1.0 (still works, but deprecated)
gen := openapi.NewGenerator("My API", "1.0.0")
gen.SetDescription("REST API")
gen.AddServer("http://localhost:8080", "Dev")

// v1.2.0 (recommended)
gen := openapi.New("My API", "1.0.0").
    Description("REST API").
    Server("http://localhost:8080", "Dev")

// Old way: manual registration
router := espresso.Portafilter()
router.Get("/users", getUsers)
http.Handle("/openapi.json", gen.Handler())

// New way: automatic integration
router := espresso.OpenAPI(gen)
router.Get("/users", getUsers, openapi.Tags("users")).
    ServeOpenAPI("/openapi.json").
    ServeDocs("/docs", "/openapi.json").
    Brew()
```

## [1.1.0] - 2025-04-08

### Added
- **Cookie extractor** - `extractor.Cookie[T]` for HTTP cookies
- **File upload extractors** - `extractor.File`, `extractor.Files`, `extractor.Multipart` for file uploads
- **SSE streaming** - `response.SSE` for Server-Sent Events support
- **Authentication middleware** - JWT, BasicAuth, APIKey middlewares in `middleware/http`
  - `JWTMiddleware` with RS256/HS256 support
  - `BasicAuthMiddleware` with user validation
  - `APIKeyMiddleware` with header/query param support
- **OpenAPI generator** - Package `openapi` for OpenAPI 3.0 specification generation
  - `NewGenerator()`, `AddPath()`, `AddSchema()`, `Handler()`
  - `GenerateSchemaFromType()` for automatic schema generation from Go types
- **Scalar UI** - Modern API documentation UI (`ScalarUIHandler`, `ScalarUI`)

### Changed
- Replaced `interface{}` with `any` throughout codebase (Go 1.18+ idiom)

### Documentation
- Added comprehensive examples for file upload, SSE streaming, and authentication
- Added authentication middleware documentation

## [1.0.2] - 2025-01-XX

### Added
- **Lungo handler** - New handler for 3-parameter functions (context + 2 extractors)
  - `HandlerCtxReq1Req2Err[Req1, Req2, Res]` - Typed handler for dual extractors
  - `HandlerCtxReq1Req2[Req1, Req2, Res]` - Variant without error return
  - `Lungo[Req1, Req2, Res]` - Coffee-themed alias (named after "long" espresso)
  - `LungoNoErr[Req1, Req2, Res]` - No-error variant
  - Use case: handlers needing both path parameters AND request body

### Fixed
- Escaped angle brackets in Go comments for Vue parsing
- Fixed documentation sidebar link (handler → espresso)
- Fixed ignore dead links in VitePress config
- Updated documentation for bun instead of npm

### Documentation
- Complete VitePress documentation site with guides, examples, and API reference
- Added Mermaid diagram support
- Added comprehensive examples (basic-api, middleware-stack, state-management, production)

## [1.0.1] - 2024-12-XX

### Added
- Initial VitePress documentation site
- Code Hike integration for syntax highlighting
- Mermaid diagram support

## [1.0.0] - 2024-12-XX

### Added
- Initial release
- Core router with fluent API (`Portafilter()`, `Get()`, `Post()`, `Put()`, `Delete()`, etc.)
- Handler aliases: `Ristretto()`, `Solo()`, `Doppio()`
- Built-in response types: `JSON[T]`, `Text`, `Status`
- State/dependency injection with `WithState()` and `GetState[T]()`
- Extractors: `JSON[T]`, `Query[T]`, `Path[T]`, `Form[T]`, `Header[T]`, `XML[T]`
- HTTP middleware: `RequestIDMiddleware`, `RecoverMiddleware`, `CORSMiddleware`, `CompressMiddleware`, `RateLimitMiddleware`, `AuthMiddleware`
- Service layers: `LoggingLayer`, `TimeoutLayer`, `RetryLayer`, `CircuitBreakerLayer`, `ConcurrencyLimitLayer`, `MetricsLayer`, `ValidationLayer`
- Object pooling: `BufferPool`, `ByteSlicePool`, `StringSlicePool`
- Comprehensive test coverage (78%+)