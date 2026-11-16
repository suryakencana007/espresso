# Migrating from Espresso v2.4 to v2.5

Upgrading from v2.4 to v2.5 is easy: **nothing you already have breaks.**
There is no rename, no removed function, no changed signature. Your app
should compile and run exactly as it did before.

So why a new version? Because v2.5 adds something genuinely new and fixes a
bug that only appeared under load:

1. **New — headers on error responses.** A handler that returns an error can
   now attach a response header. That matters for status codes that are
   expected to carry one: a `429 Too Many Requests` should tell the client how
   long to wait (`Retry-After`), and a `401` should advertise how to
   authenticate (`WWW-Authenticate`).
2. **Fixed — the circuit breaker's probe limit.** When a failing dependency
   starts recovering, the breaker lets a small number of requests through to
   test the water. Under concurrency, more than the allowed number could slip
   through at once. Fixed.

If you are coming from v2.3, read the
[v2.4 migration guide](./migration-v2.3-to-v2.4.md) first. It covers two
changes that *do* need action — the `X-Forwarded-For` opt-in for rate
limiting, and the 1 MB JSON body cap.

Notes in this guide are labeled, so you know at a glance whether to act:

- **Required** — you must change something, or your build or behavior breaks.
  (v2.5 has none of these.)
- **Recommended** — worth checking, in case the change affects how you tune
  things.
- **Informational** — read it so you know what changed; no action to take.

---

## Upgrade in a minute

If your app already runs on v2.4 and uses the public API:

1. **Update the dependency:**

   ```sh
   go get github.com/suryakencana007/espresso/v2@v2.5.1
   ```

2. **Build it:** `go build ./...`. It will. Nothing in v2.5 can produce a
   compile error.

3. **That's it.** The rest of this guide is optional reading: how to use the
   new error headers, and one thing to re-check if you tune your circuit
   breaker.

---

## Put headers on an error response

**Optional — new in v2.5.**

Handlers return `(Res, error)` and never see the `http.ResponseWriter`, so
until now a handler-returned error had no way to set a response header. A
`429 Too Many Requests` should carry `Retry-After` (RFC 9110 §10.2.3), but
there was no way to add it short of wrapping every handler or moving the
header logic into middleware that can't see the error.

Now the error carries its own headers.

### Set a header

Call `.WithHeader(key, value)`:

```go
return espresso.ErrTooManyRequests("too many attempts").
    WithCode("TOO_MANY_REQUESTS").
    WithHeader("Retry-After", "30")
```

The client gets an ordinary `429` with the usual JSON body, plus the header
`Retry-After: 30`.

Calling `.WithHeader` again with the same key **replaces** the first value —
the same "last one wins" behavior as `.WithCode(...)` and `.WithDetail(...)`.

Here it is as a complete, runnable program:

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

### Add headers that can repeat

Some headers are allowed to appear more than once — two `Set-Cookie` lines,
or several `WWW-Authenticate` challenges. For those, use
`.WithHeaderAdd(key, value)`, which appends instead of replacing:

```go
return espresso.ErrUnauthorized("authentication required").
    WithHeader("WWW-Authenticate", `Bearer realm="api"`).
    WithHeaderAdd("WWW-Authenticate", `Basic realm="api"`)
```

A later `.WithHeader` on the same key discards everything `.WithHeaderAdd`
added and keeps only the new value.

### Headers that are ignored

A few headers are off-limits, because letting a handler set them could break
the response. Espresso quietly drops these — capitalization doesn't matter:

| Group | Headers | Why |
|-------|---------|-----|
| Owned by the framework | `Content-Type`, `Content-Length`, `Transfer-Encoding` | Setting them by hand would malform the response |
| Hop-by-hop (RFC 9110 §7.6.1) | `Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailer`, `Upgrade` | They describe a single network hop, not the response itself |

Everything else goes through untouched: `Retry-After`, `WWW-Authenticate`,
`Set-Cookie`, `Link`, and any `X-*` header of your own.

And if you never call these methods, the response is byte-for-byte identical
to v2.4 — the change can't surprise existing clients.

---

## The circuit breaker now keeps to its probe limit

**Recommended — worth a look if you tune the breaker.**

### What was wrong

`CircuitBreakerConfig.HalfOpenMaxProbes` (added in v2.4, default `1`) is meant
to cap how many requests test a recovering dependency at the same time. Under
concurrency the cap did not hold: several requests would each open the gate
for themselves and reset the in-flight counter, so more than the limit got
through. That is exactly the stampede the limit exists to avoid.

v2.5 lets only one request open the gate. The shared counter is once again the
single thing deciding who gets in.

### What it looks like in code

Nothing changes on your side — same field, same config:

```go
config := servicemiddleware.CircuitBreakerConfig{
    ServiceName:       "payments",
    FailureThreshold:  5,
    Timeout:           30 * time.Second,
    SuccessThreshold:  2,
    HalfOpenMaxProbes: 1, // at most one request probes while HalfOpen
}
```

### What you might notice

If your breaker used to send a burst of requests once it half-opened, you will
now see at most `HalfOpenMaxProbes` at a time. That is the intended behavior.
If you had raised the value above `1` to make recovery work despite the extra
traffic, you can lower it back to the default and let a single probe do the
job.

---

## Go 1.27 and the JSON library

**Informational — no action needed.**

Espresso's JSON library (`bytedance/sonic`) was pinned to a version that did
not know about Go 1.27. On Go 1.27 that caused two problems: a warning printed
on every startup, and a silent fall back to the slower standard-library
encoder. v2.5 bumps it to a version that supports Go 1.27.

The JSON you send and receive is byte-for-byte identical. You simply get the
fast path back, and the warning stops. On older Go versions, nothing changes
at all.

---

## If you need to roll back

v2.5 only adds API and fixes a bug, so there is no data or wire-format change
to undo. Going back to v2.4 means the two new `.WithHeader` methods no longer
exist (any code using them won't compile), and the circuit-breaker probe race
returns. Nothing else.
