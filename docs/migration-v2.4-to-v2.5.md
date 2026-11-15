# Migrating from Espresso v2.4 to v2.5

v2.5 is an **additive minor release** — **Headers & Recovery**. The module
path is unchanged, every v2.4 symbol keeps its signature, and an app on
v2.4 compiles and runs unchanged. The release does two things:

1. **New:** handler-returned errors can set response headers via
   `(*Error).WithHeader` and `(*Error).WithHeaderAdd` — so a `429` can send
   the `Retry-After` its status implies.
2. **Fixed:** `CircuitBreakerLayer` enforces `HalfOpenMaxProbes` again under
   concurrency.

For the v2.3 → v2.4 jump (rate-limit `X-Forwarded-For` opt-in, the 1 MB JSON
body cap, the circuit-breaker failure-counter change), see
[`migration-v2.3-to-v2.4.md`](./migration-v2.3-to-v2.4.md) — that content is
not re-covered here.

If a section says **"Required"**, your build or behavior breaks until you
apply it. **"Recommended"** means the v2.4 form still works but the v2.5
form is safer or clearer. **"Informational"** means there is no action to
take.

---

## Five-Minute Upgrade Checklist

For an app already on v2.4 through the public API only:

1. **Bump the dependency** —
   `go get github.com/suryakencana007/espresso/v2@v2.5.0`.
2. **`go build ./...`** — there is no removal, rename, or signature change.
   Nothing in this release can produce a compile error. **Informational.**
3. **If you returned `429`/`401`/`503` errors and wanted to attach
   `Retry-After`, `WWW-Authenticate`, or `Set-Cookie` but could not** —
   adopt `WithHeader` / `WithHeaderAdd` (recipe below). **Optional.**
4. **If you raised `CircuitBreakerConfig.HalfOpenMaxProbes` above 1 to
   compensate for the stampede** — that stampede was the bug. Re-check
   whether you still need a value above 1; the bound is now enforced.
   **Recommended.**

---

## Response headers on handler-returned errors

**New API — optional.**

Handlers return `(Res, error)` and never see the `http.ResponseWriter`, so
before v2.5 a handler-returned `*Error` could not set a response header. A
`429 Too Many Requests` should carry `Retry-After` (RFC 9110 §10.2.3), but
there was no way to add it short of wrapping every handler.

### Attaching a header

`WithHeader(key, value)` sets a header and returns the error for chaining.
It uses **Set** semantics — a second call with the same key replaces the
first, matching `WithCode` / `WithDetail`.

```go
return espresso.ErrTooManyRequests("too many attempts").
    WithCode("TOO_MANY_REQUESTS").
    WithHeader("Retry-After", "30")
```

Complete runnable example:

```go
package main

import (
	"context"

	"github.com/suryakencana007/espresso/v2"
)

type LoginReq struct {
	User string `json:"user"`
}

type LoginRes struct {
	Token string `json:"token"`
}

// login always answers 429 with the Retry-After its status implies.
func login(ctx context.Context, req *espresso.JSON[LoginReq]) (espresso.JSON[LoginRes], error) {
	return espresso.JSON[LoginRes]{}, espresso.ErrTooManyRequests("too many attempts").
		WithCode("TOO_MANY_REQUESTS").
		WithHeader("Retry-After", "30")
}

func main() {
	router := espresso.Portafilter()
	router.Post("/login", login)
	router.Brew(espresso.WithAddr(":8080"))
}
```

### Repeatable headers

`WithHeaderAdd(key, value)` appends instead of replacing, mirroring
`http.Header.Add`. Use it for headers that legitimately repeat:

```go
return espresso.ErrUnauthorized("authentication required").
    WithHeader("WWW-Authenticate", `Bearer realm="api"`).
    WithHeaderAdd("WWW-Authenticate", `Basic realm="api"`)
```

A later `WithHeader` on the same key collapses everything a `WithHeaderAdd`
accumulated.

### What cannot be set

Headers the server owns are ignored, case-insensitively, so a handler
cannot corrupt response framing or emit hop-by-hop headers:

| Category | Headers |
|----------|---------|
| Framing (server-owned) | `Content-Type`, `Content-Length`, `Transfer-Encoding` |
| Hop-by-hop (RFC 9110 §7.6.1) | `Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailer`, `Upgrade` |

`Content-Type` stays writer-owned: the error envelope always serializes as
`application/json; charset=utf-8`. Everything else — `Retry-After`,
`WWW-Authenticate`, `Set-Cookie`, `X-*`, `Link`, and so on — passes through.

An error with no headers produces the exact same bytes as v2.4.

---

## Circuit breaker half-open probes are bounded again

**Behavior fix — recommended if you tune the breaker.**

v2.4 introduced `CircuitBreakerConfig.HalfOpenMaxProbes` (default `1`) to
bound how many requests probe a recovering upstream. Under concurrency the
bound was not enforced: several requests that observed `StateOpen` with the
open timeout elapsed each ran the `Open → HalfOpen` transition and reset the
in-flight probe counter, wiping the slot a peer had just taken. More than
`HalfOpenMaxProbes` probes could reach the upstream — the stampede the limit
exists to prevent.

v2.5 makes exactly one goroutine own the transition and its reset; the
atomic counter is again the only admission gate.

```go
config := servicemiddleware.CircuitBreakerConfig{
    ServiceName:       "payments",
    FailureThreshold:  5,
    Timeout:           30 * time.Second,
    SuccessThreshold:  2,
    HalfOpenMaxProbes: 1, // at most one request probes while HalfOpen
}
```

**What you might observe:** if your breaker previously sent a burst through
on recovery and your downstream coped, you will now see at most
`HalfOpenMaxProbes` in flight. That is the intended behavior. If you had
raised the value to make recovery work around the extra traffic, lower it
back toward `1` and let the breaker recover on a single probe.

No API change — this is the same field, now honored.

---

## Go 1.27 and sonic

**Informational.**

`github.com/bytedance/sonic` is now v1.15.3 (and `sonic/loader` v0.5.2).
sonic v1.15.0 did not support Go 1.27: every process start printed a
fallback warning and JSON encoding silently fell back to `encoding/json`,
losing the `JSON[T]` fast path. Encoded output is byte-for-byte identical;
only the warning and the fallback are gone. No code change is required.

CI now runs the test and vulnerability-scan jobs on Go 1.27 as well.

---

## Rollback

v2.5 is additive; reverting to v2.4 only affects the new error-header
methods (which would fail to compile away) and restores the circuit-breaker
probe race. No data or wire-format migration is involved.
