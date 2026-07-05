# Task 8: RateLimit — XFF Trust + Per-Key Bucket Eviction

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 0.5 day
**Dependencies:** Task 2 (both touch `middleware/http/middleware.go` `RateLimitMiddleware` neighborhood; coordinate on CHANGELOG merge order)

> **Status: ✅ Shipped 2026-07-05 (v2.4.0).** Delivered via #69 — clientKey via net.SplitHostPort; WithTrustedProxies opt-in for XFF (RFC 7239 rightmost-trusted-hop); TokenBucketLimiterPerKey background sweeper + Close/WithBucketTTL.

## Context

Two related defects in `RateLimitMiddleware` (`middleware/http/middleware.go:237-257`):

- **D1 — spoofable per-key keying.** The middleware keys on `r.RemoteAddr` (which includes the ephemeral port, so every new TCP connection gets a fresh bucket — an attacker bypasses per-key limits by reconnecting) and blindly overrides it with the client-controlled `X-Forwarded-For` header (`middleware.go:240-243`). Any direct client can send arbitrary XFF values to mint unlimited fresh keys or poison another client's key. There is no trusted-proxy configuration; the header is trusted unconditionally.

- **D2 — per-key bucket map never evicts.** `TokenBucketLimiterPerKey`'s buckets map (`middleware.go:265/337`) only ever grows. Combined with D1, spoofed XFF values become an unbounded memory-growth DoS. Contrast `SlidingWindowLimiter` (`middleware.go:391-411`) which does have TTL-based cleanup.

Also `SlidingWindowLimiter.Allow` holds one global mutex and does an O(n) scan plus a slice allocation per request (`middleware.go:414-442`) — a contention hot spot at high concurrency. Note in the godoc, not a fix.

## Acceptance Criteria

- [x] By default, `RateLimitMiddleware` keys on the **host** portion of `r.RemoteAddr` (via `net.SplitHostPort`) — not the full `host:port`. New TCP connections from the same client IP hit the same bucket.
- [x] `X-Forwarded-For` is **not trusted by default**. A `WithTrustedProxies(cidrs ...string)` option enables it and, when set, takes the **rightmost trusted hop** from the header — never the raw leftmost value.
- [x] `TokenBucketLimiterPerKey` evicts buckets idle for > TTL (configurable, default 10 minutes). Eviction runs lazily on `Allow` or via a background goroutine — pick one and document.
- [x] Regression tests reproduce both defects on the pre-fix code and lock the fixes.
- [x] `SlidingWindowLimiter` godoc gains a `// Note: single-mutex, O(n)-per-Allow — for high-concurrency use TokenBucketLimiter*` comment (documentation only, no behavior change).
- [x] Migration note: existing users of `RateLimitMiddleware` who rely on `X-Forwarded-For` must add `WithTrustedProxies(...)` — this is a **breaking behavior change** (correctly).

## Technical Approach

### Step 8.1 — Reproduce the defects

```go
// D1a: ephemeral port breaks per-key limiting.
func TestRateLimit_PerKeyKeysOnHostNotHostPort(t *testing.T) {
    // Simulate two requests from same IP different ports.
    // Pre-fix: two separate buckets. Post-fix: one bucket.
}

// D1b: unbounded XFF trust.
func TestRateLimit_XFFNotTrustedByDefault(t *testing.T) {
    // Two requests: one with X-Forwarded-For: 1.1.1.1, one with 2.2.2.2, same RemoteAddr.
    // Pre-fix: two buckets. Post-fix: one bucket (RemoteAddr).
}

// D1c: trusted proxy takes rightmost.
func TestRateLimit_TrustedProxyTakesRightmostHop(t *testing.T) {
    // With WithTrustedProxies("192.168.0.0/16"): X-Forwarded-For: attacker, real-client
    // and RemoteAddr in 192.168.x.x → key on "real-client".
}

// D2: per-key map eviction.
func TestRateLimitPerKey_EvictsIdleBuckets(t *testing.T) {
    // Configure TTL=100ms. Hit two different keys, wait 200ms, hit a third.
    // Assert the first two are evicted (map size <= 2 after the sweep).
}
```

Confirm all four fail on pre-fix code.

### Step 8.2 — Fix keying default

```go
func defaultKey(r *http.Request) string {
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil { return r.RemoteAddr } // fallback if malformed
    return host
}
```

Remove the unconditional `X-Forwarded-For` override (`middleware.go:240-243`).

### Step 8.3 — Add WithTrustedProxies

```go
type rateLimitConfig struct {
    // ...
    trustedProxies []*net.IPNet
}

func WithTrustedProxies(cidrs ...string) RateLimitOption {
    return func(c *rateLimitConfig) {
        for _, cidr := range cidrs {
            if _, n, err := net.ParseCIDR(cidr); err == nil { c.trustedProxies = append(c.trustedProxies, n) }
            // Also accept bare IPs by wrapping in /32 or /128.
        }
    }
}
```

When resolving the key, if `RemoteAddr`'s host is in a trusted CIDR **and** `X-Forwarded-For` is present, take the rightmost non-trusted hop (walk right-to-left, skip trusted hops, first non-trusted is the client). Standard proxy-chain resolution.

### Step 8.4 — Add TTL eviction to TokenBucketLimiterPerKey

Two shapes:

- **(a) Lazy per-Allow**: on each `Allow(key)`, if a random 1-in-N check fires (e.g. `rand.Intn(1000) == 0`), walk the map and delete keys with `time.Since(bucket.lastRefill) > ttl`. Amortized cost, no goroutine.
- **(b) Background sweeper**: `time.NewTicker(ttl/2)` in a goroutine started at limiter construction; sweep the map under write lock. Cleaner cost model, needs shutdown handling.

Prefer (b) — matches `SlidingWindowLimiter`'s pattern (`middleware.go:391-411`). Provide a `Close()` method on the limiter to stop the goroutine; document that users constructing the limiter directly must call `Close()`.

TTL default: 10 minutes. Configurable via a limiter constructor option (`WithBucketTTL(d time.Duration)`).

### Step 8.5 — Document SlidingWindow perf note

Add above `SlidingWindowLimiter`:

```go
// SlidingWindowLimiter provides a request-count-per-window rate limit.
//
// Note: uses a single mutex and performs an O(n) prune per Allow(), where n is
// the number of requests within the window. For sub-millisecond p99 at high
// concurrency, prefer TokenBucketLimiter or TokenBucketLimiterPerKey.
```

## Tests Required

- `TestRateLimit_PerKeyKeysOnHostNotHostPort` (D1a).
- `TestRateLimit_XFFNotTrustedByDefault` (D1b).
- `TestRateLimit_TrustedProxyTakesRightmostHop` (D1c).
- `TestRateLimit_UntrustedProxyIgnoresXFF`: RemoteAddr not in a trusted CIDR + XFF present → XFF ignored, key on RemoteAddr host.
- `TestRateLimitPerKey_EvictsIdleBuckets` (D2).
- Run with `-race`.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `go test -race ./middleware/http/... -count=2` clean.
- [x] `golangci-lint run ./...` clean.
- [x] CI's `Test (race)` job green on the PR.
- [x] CHANGELOG `[Unreleased]` entry under `Fixed`: `RateLimitMiddleware` no longer trusts `X-Forwarded-For` by default (breaking behavior change; opt in via `WithTrustedProxies`); keys default to the client host, not `host:port`; `TokenBucketLimiterPerKey` evicts idle buckets. Under `Added`: `WithTrustedProxies`, `WithBucketTTL`.
- [x] Migration note (Task 12): callers relying on X-Forwarded-For must add `WithTrustedProxies(...)` and set the reverse-proxy CIDRs; users that pass through a well-known LB (Cloudflare, AWS ALB, etc.) get a doc pointer to the current published CIDR list.
- [x] No public API signature changed on `RateLimitMiddleware` (only new option added).
