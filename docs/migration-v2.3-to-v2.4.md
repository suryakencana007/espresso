# Migrating from Espresso v2.3 to v2.4

v2.4 is a **correctness release** — **Ground Truth**. The module path is
unchanged, the dispatch and extractor surfaces are unchanged, and most
apps will not need any code changes at all. What changed is the *behavior*
of several first-party components on paths that were previously either
buggy or unsafe: rate-limiting no longer trusts `X-Forwarded-For` by
default, JSON body reads are capped at 1 MB, the circuit breaker's
failure counter now resets on success while `Closed`, `BrewContext`
actually drains in-flight requests on cancel, SSE streams survive the
default `WriteTimeout`, and the `TimeoutLayer` no longer races the
request-struct pool. Nine of the ten P0/P1 audit findings from
2026-07-02 are closed in this release.

For the v2.2 → v2.3 jump (OpenAPI security-scheme API, spec-serving
hardening, `AutoRegister` removal), see the v2.3 CHANGELOG entry — that
content is not re-covered here.

If a section says **"Required"**, your build or behavior will break in
v2.4 until you apply it. **"Recommended"** means the v2.3 pattern still
compiles and works but the v2.4 form is safer or clearer.
**"Informational"** means there is no action to take — listed so the
migration story is complete.

---

## Five-Minute Upgrade Checklist

For an app already on v2.3 through the public API only:

1. **Bump the dependency** —
   `go get github.com/suryakencana007/espresso/v2@v2.4.0`.
2. **If you run behind a reverse proxy and use `RateLimitMiddleware`** —
   add `httpmiddleware.WithTrustedProxies("your-proxy-cidr/prefix", ...)`
   (recipe below). Without it, all requests key on the proxy's IP
   instead of the client's — you effectively rate-limit the whole world
   as one bucket. **Required** for correct rate limiting.
3. **If any of your handlers legitimately POST bodies larger than 1 MB** —
   set `router.WithJSONBodyLimit(N)` with the size you actually accept
   (recipe below). Otherwise clients hit `413 Payload Too Large`.
   **Required** for those routes.
4. **If your circuit breaker was reopening surprisingly often over the
   process lifetime** — that was a defect (D1: failures accumulated
   forever), now fixed by resetting the counter on any success while
   `Closed`. No action needed; the new behavior matches what you
   probably expected. **Informational.**
5. **If any of your handlers is a long-running goroutine that reads a
   `*Req` after returning** — stop; the framework now leaks the
   `*Req` to GC when a `TimeoutLayer` abandons the goroutine, but any
   *user-code* goroutine that captures the pooled struct is your
   responsibility (this is not new, just now explicitly documented).
   **Informational.**
6. **`go build ./...`** — no step above produces a compile error.
   Every v2.4 change is a behavior fix or an additive API.

---

## Rate limiting no longer trusts X-Forwarded-For

**Required if you run behind a reverse proxy.**

Before, `httpmiddleware.RateLimitMiddleware` treated any request with an
`X-Forwarded-For` header as if the header contained the client's IP.
Any client that connected directly to the server could set XFF to
anything they wanted — spoofing another client's identity or minting
unlimited fresh keys by rotating XFF values.

v2.4 flips the default: XFF is **ignored** unless you opt in. When you
opt in with `WithTrustedProxies(...)`, the middleware only looks at XFF
if the request's `RemoteAddr` host is in a trusted CIDR, and even then
it walks the XFF list right-to-left, skipping trusted hops, and picks
the first non-trusted address as the client key (RFC 7239 semantics).

The key also switched from full `host:port` to the host portion via
`net.SplitHostPort`, so a client can no longer bypass a per-key limiter
by reconnecting from a fresh ephemeral port.

**Before (v2.3, unsafe by default):**

```go
limiter := httpmiddleware.NewTokenBucketLimiterPerKey(100, 100)
r.Use(httpmiddleware.RateLimitMiddleware(limiter))
// Any request with X-Forwarded-For was treated as if that header
// contained the client IP, regardless of who sent it.
```

**After (v2.4, opt in when you actually have a trusted proxy):**

```go
limiter := httpmiddleware.NewTokenBucketLimiterPerKey(100, 100)
r.Use(httpmiddleware.RateLimitMiddleware(
    limiter,
    // Only trust XFF when RemoteAddr is your reverse proxy.
    httpmiddleware.WithTrustedProxies("10.0.0.0/8", "192.168.0.0/16"),
))
```

If you're direct-Internet-facing without a proxy, do nothing — the safe
default keys on the client's actual IP.

`WithTrustedProxies` accepts CIDR notation (`"10.0.0.0/8"`) or bare IPs
(`"10.0.0.5"` — auto-wrapped as `/32` for IPv4 or `/128` for IPv6).

`TokenBucketLimiterPerKey` now also evicts idle buckets in the
background (default TTL 10 minutes; configurable via `WithBucketTTL`).
Call `limiter.Close()` when the limiter is no longer needed to stop the
sweeper goroutine — the global `TokenBucketLimiter` has an idempotent
no-op `Close`, so you can call it unconditionally.

---

## JSON body cap at 1 MB by default

**Required for routes that POST bodies larger than 1 MB.**

Before, `espresso.JSON[T].Extract`, `extractor.RawBody`,
`extractor.RawBodyWithHeaders`, and `extractor.XML[T]` all read the
request body **unbounded**. A client could stream any size body into
memory. The advertised 1 MB safety in `http.go` (`MaxPayloadSize`,
`DecodeSafeJSON`) was called only from `cmd/example` — the framework
extractors never used it.

v2.4 caps every one of those extractors at the router's configured
`WithJSONBodyLimit` (default: 1 MB, per `MaxPayloadSize`). Over-limit
bodies return `413 Payload Too Large` with the canonical error envelope
before the handler runs.

If your app accepts bodies up to `N` bytes on some routes:

```go
r := espresso.Portafilter().
    WithJSONBodyLimit(5 * 1024 * 1024). // 5 MB
    Post("/api/upload", handler)
```

Non-positive limits are silently coerced to `MaxPayloadSize` so
misconfiguration cannot uncap the router.

If you want to distinguish a 413 in your own handler code (e.g. to
retry with a smaller payload upstream), use the new constructor:

```go
if err := something(); err != nil {
    return espresso.ErrRequestEntityTooLarge("upstream limit exceeded"), nil
}
```

The extractor path returns the error automatically; you only need the
constructor if you're minting a 413 from your own handler.

---

## `BrewContext` now drains in-flight requests on cancel

**Informational — this fixes a data-loss bug on the programmatic
shutdown path.**

Before, `router.BrewContext(ctx, ...)` waited on `<-ctx.Done()` then
called `gracefulShutdown(ctx, ...)` passing the same canceled context
as the parent. Because `context.WithTimeout` on a canceled parent
returns an already-expired context, `http.Server.Shutdown` returned in
~0 ms without waiting for in-flight requests, and `OnShutdown` hooks
received `ctx.Err() == context.Canceled` — effectively skipping the
"drain in-flight requests up to `ShutdownTimeout`" contract.

v2.4 uses `context.WithoutCancel(ctx)` before deriving the shutdown
timeout, so the shutdown context is fresh, `OnShutdown` hooks see a
live ctx with a future deadline, and `http.Server.Shutdown` gets the
full `ShutdownTimeout` window to drain.

No API change. If you observed `BrewContext` returning ~immediately
after cancellation, that will now take up to `ShutdownTimeout`
(default 10 s) — which is the documented behavior.

---

## SSE streams survive the default `WriteTimeout`

**Informational — no action required, and cleaner defaults.**

Before, the default server `WriteTimeout` of 10 seconds
(`server.go:31`) killed every SSE stream at ~10 seconds, regardless of
`WithKeepAlive`. `cmd/example/sse` served an infinite ticker stream
via `router.Brew()` with defaults — it died at 10 s. The workaround
was `WithWriteTimeout(0)` on the router, which disabled the DoS
protection for all routes.

v2.4 fixes this in `serveStream` (in `sse.go`) by calling
`http.NewResponseController(w).SetWriteDeadline(time.Time{})` at stream
open, clearing the deadline for that specific connection while leaving
the 10 s default in place for non-stream routes.

If you had set `WithWriteTimeout(0)` as a workaround for SSE, you can
remove it — the default now supports SSE out of the box.

---

## Circuit breaker semantic fixes

**Informational — every fix moves the behavior toward what users
probably expected.**

Four correctness defects in `servicemiddleware.CircuitBreakerLayer`
were closed. Each moves the state machine toward its documented intent:

- **Failures no longer accumulate over process lifetime.** In v2.3,
  `FailureThreshold=5` meant "5 failures ever" — 5 transient errors
  across days of 99.99% success would open the circuit. v2.4 resets
  the failure counter on any successful call while the circuit is
  `Closed`.
- **The transitioning probe's success is counted.** In v2.3, the
  goroutine that flipped `Open→HalfOpen` captured its state on entry;
  by the time the success was recorded, the check
  `if currentState == StateHalfOpen` was false and the transitioning
  probe's success was skipped. v2.4 re-reads state under the write
  lock at the mutation point, so the transitioning probe's success
  counts toward `SuccessThreshold` and a single successful probe with
  `SuccessThreshold=1` genuinely closes the circuit.
- **Success and failure paths re-read state under the write lock**
  before mutating. Concurrent state changes since goroutine entry are
  now observed and handled.
- **Half-open bounds concurrent probes.** New
  `CircuitBreakerConfig.HalfOpenMaxProbes` field (default 1) limits
  the number of requests that pass through as probes at once — extra
  requests get an immediate `CircuitBreakerError` with
  `State=StateHalfOpen`. Before, every request in `HalfOpen` reached
  the wrapped service, stampeding the recovering upstream. Zero or
  negative values are normalized to 1 at layer construction.

The `CircuitBreakerConfig` struct gains one field (`HalfOpenMaxProbes`)
which is fully additive — existing users who don't set it get the
sensible default. Its `DefaultCircuitBreakerConfig` variable now
carries `HalfOpenMaxProbes: 1` too.

The duplicate `espresso.CircuitBreakerError` type is now a type alias
for `servicemiddleware.CircuitBreakerError` (both packages resolve to
the same underlying type), so `errors.As` from either package matches
the same instance. No API change; existing consumers work unchanged.

---

## Validator dereferences pointer fields

**Informational — this fixes optional-field validation on the canonical
Go pattern.**

Before, every validator rule except `required` failed on pointer
fields — the canonical Go idiom for optional JSON fields:

```go
type Req struct {
    Email *string `validate:"email"`  // v2.3: valid pointer → "email rule requires string field"
    Age   *int    `validate:"min=18"` // v2.3: valid pointer → "min/max not supported for kind ptr"
}
```

Both nil AND non-nil valid values failed — contradicting the docs'
"Nil pointer fields are skipped" claim and rejecting perfectly good
client requests.

v2.4 handles pointers correctly:

- Nil pointers **skip** all rules except `required`, matching the
  documented "optional pointer" semantics.
- Non-nil pointers are **dereferenced** so the rule sees the element
  value.
- `required` continues to operate on the pointer itself (checks
  `IsNil`), so a non-nil pointer to any value (including zero) passes
  `required` — matches Go's "*T is present iff non-nil" semantics.

If you had workaround code that validated on the value field with
`required` on the pointer, you can simplify to putting the rules
directly on the pointer field.

---

## Additive API surface (no migration needed)

Everything below is additive — call it or don't; existing code works
unchanged.

### Router options

- **`router.WithJSONBodyLimit(int64)`** — configure the per-router body
  size cap (default `MaxPayloadSize` = 1 MB). Non-positive values fall
  back to the default.

### Middleware options

- **`httpmiddleware.WithTrustedProxies(cidrs ...string)`** — opt into
  `X-Forwarded-For` when behind a reverse proxy (see the RateLimit
  section above).
- **`httpmiddleware.WithBucketTTL(d time.Duration)`** — configure the
  idle-bucket eviction TTL on `TokenBucketLimiterPerKey`.
- **`TokenBucketLimiter.Close() error`** — stop the background sweeper
  goroutine on per-key limiters. No-op on global limiters, safe to
  call unconditionally.

### Errors

- **`espresso.ErrRequestEntityTooLarge(msg string) *Error`** —
  413 Payload Too Large constructor. Body-cap-exceeded errors from
  the extractor path use this automatically; user handlers can return
  it directly when their own I/O paths need the same shape.

### Service-layer helpers

- **`CircuitBreakerConfig.HalfOpenMaxProbes int`** — bounds concurrent
  probes in the `HalfOpen` state (default 1).
- **`servicemiddleware.IsAbandonedByTimeout(err error) bool`** —
  detects the sentinel error that `TimeoutLayer` returns when it
  abandoned a still-running handler goroutine. The framework itself
  uses this in `applyLayersAndConvert` to avoid recycling the request
  struct while the abandoned goroutine still holds it; user code
  rarely needs to check this directly.

---

## What was fixed under the hood

These are behavior fixes with no user-facing API change; listed so the
release picture is complete:

- **`TimeoutLayer` no longer races the request-struct pool.** In v2.3
  the layer spawned a goroutine and returned on `ctx.Done()`, leaving
  the goroutine holding `*Req` while the outer handler reset and
  re-pooled it — a `-race`-detectable data race on the flagship
  service layer. v2.4 returns a sentinel error that the handler
  detects to skip the pool return; the abandoned struct is leaked to
  GC rather than reused.
- **`LoggingMiddleware` is compatible with SSE and WebSocket.** The
  `statusRecorder` wrapper now forwards `http.Flusher`, `http.Hijacker`,
  `http.Pusher`, and `Unwrap()`. In v2.3, installing `LoggingMiddleware`
  broke SSE (500 "streaming not supported") and WebSocket upgrades
  (non-101 response). v2.4: both work.
- **`TokenBucketLimiter` refill starvation fixed.** In v2.3 the refill
  math truncated to whole seconds while always advancing the refill
  clock, so under sustained ≥1 req/s traffic tokens were consumed to
  zero and never replenished. v2.4 uses nanosecond `int64` math so
  fractional seconds are credited.
- **OpenAPI generator mutate-while-serve race + recursive-type
  overflow.** Mutation methods now hold the generator's mutex across
  spec mutation and cache invalidation in one critical section, and
  `GenerateSchemaFromType` uses cycle-detection to emit `$ref` on
  recursive types instead of stack-overflowing. Documenting a tree
  node or comment thread no longer kills the process.
- **`context.Canceled` from a service layer maps to 503, not 500.**
  Previously it fell through to the internal-error fallback; now it
  joins `context.DeadlineExceeded` in the `Service Unavailable`
  bucket.

---

## Symbols removed or deprecated

None. v2.4 is additive-only at the API surface; every prior public
symbol still resolves. The internal `errorsAs` helper in
`middleware/service/layer.go` was replaced by standard `errors.As`,
but it was unexported so no user code was affected.

---

## Downstream: Barista

If Barista (or any downstream) hits the new rate-limit or JSON-body
defaults during v2.4 upgrade, both are one option-call each
(`WithTrustedProxies` / `WithJSONBodyLimit`) and both are the correct
defaults for the general case. The circuit breaker semantic changes
should make Barista's transient-error scenarios less noisy without any
code change.

---

## Full CHANGELOG

For the complete list of Added/Changed/Fixed items in `[2.4.0]` with
per-item references to the audit findings and PRs, see
[`CHANGELOG.md`](../CHANGELOG.md).
