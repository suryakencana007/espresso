# Changelog

Every release, explained in plain language — what changed and why you should
care. Espresso follows [Semantic Versioning](https://semver.org/), so the
version number tells you what to expect:

- **Major** (`2.x → 3.x`) — something breaking. Old code may need edits.
- **Minor** (`2.4 → 2.5`) — new features, nothing breaks. Upgrade when ready.
- **Patch** (`2.5.0 → 2.5.1`) — bug fixes only.

Looking for the exhaustive, line-by-line list? See the
[full CHANGELOG](https://github.com/suryakencana007/espresso/blob/main/CHANGELOG.md)
or browse [GitHub releases](https://github.com/suryakencana007/espresso/releases).

::: tip Latest release
**v2.5.0** — *Headers & Recovery* · 15 November 2026
:::

---

## v2.5.0 — Headers & Recovery

**15 November 2026 · minor release** · [upgrade guide](/migration-v2.4-to-v2.5)

A small, friendly release with no breaking changes. It does three useful
things.

**You can now set headers on error responses.** Until now a handler could
return an error but had no way to add a response header, because handlers
don't receive the `http.ResponseWriter`. That was a real problem for status
codes that are supposed to carry one — a `429 Too Many Requests` should tell
the client how long to wait:

```go
return espresso.ErrTooManyRequests("too many attempts").
    WithCode("TOO_MANY_REQUESTS").
    WithHeader("Retry-After", "30")
```

`WithHeader` sets a header (a second call replaces it). `WithHeaderAdd`
appends when a header may repeat, like two `Set-Cookie` lines or several
`WWW-Authenticate` challenges. Headers the server owns —
`Content-Type`, `Content-Length`, `Transfer-Encoding`, and hop-by-hop headers
like `Connection` — are ignored, so a handler can't accidentally break the
response.

**The circuit breaker keeps its promise again.** When a failing service starts
recovering, the breaker lets a limited number of requests through to test the
water — `HalfOpenMaxProbes`, one by default. A concurrency bug meant several
requests could sneak through at once, sending a burst to a service that was
barely back on its feet. Fixed, and locked with a test that fails on the old
code.

**Clean on Go 1.27.** The JSON library was pinned to a version that didn't
know Go 1.27, so it printed a warning on every startup and quietly fell back
to the slower standard-library encoder. Bumped to a version that supports
Go 1.27. Your JSON output is exactly the same — it's just fast again and the
warning is gone.

Nothing to change in your code. See the
[upgrade guide](/migration-v2.4-to-v2.5) for the details.

## v2.4.0 — Ground Truth

**5 July 2026 · minor release** · [upgrade guide](/migration-v2.3-to-v2.4)

A big correctness release. An audit compared what the framework *claimed* with
what it actually *did*, and this release fixed the serious gaps. **Two changes
need your attention when upgrading:**

- **Rate limiting no longer trusts `X-Forwarded-For` automatically.** If you
  run behind a reverse proxy, tell Espresso which proxies to trust with
  `WithTrustedProxies(...)`. Otherwise every request looks like it came from
  the proxy, and you end up rate-limiting the whole world as one user.
- **JSON request bodies are capped at 1 MB by default.** If a route
  legitimately accepts bigger uploads, raise the limit with
  `WithJSONBodyLimit(...)`. Otherwise those requests get a `413`.

Everything else is fixes under the hood: a data race in `TimeoutLayer`, a token
bucket that never refilled under steady traffic, logging middleware that broke
SSE and WebSocket routes, shutdown that didn't wait for in-flight requests,
an OpenAPI generator that raced itself, several circuit-breaker state bugs, and
validation that mishandled pointer fields. SSE streams also stopped dying after
the default 10-second write timeout.

## v2.3.0 — Backflush

**29 June 2026 · minor release**

The OpenAPI generator became trustworthy: it no longer races when you register
routes while serving, and it handles recursive types without crashing. The one
new feature is `openapi.AddSecurityScheme`, for documenting authentication in
your spec. Everything else tightened tests and corrected behavior.

## v2.2.0 — Dial It In

**28 June 2026 · minor release**

Focused on making behavior match expectations. Service-layer errors now map to
the right HTTP status instead of a blanket 500, and **every** framework error
path — handler errors, panics, auth failures, rate limits — emits the same
canonical JSON envelope. No new features.

## v2.1.0

**12 May 2026 · minor release**

A tidy-up release: it removed the old SSE API that v2.0 had deprecated, added a
`WithPreFlight` option for streams, and shipped a validator convenience helper.
Also refreshed the benchmarks against current hardware and Go.

## v2.0.0

**10 May 2026 · major release** · [upgrade guide](/migration-v1-to-v2)

The first major release after v1.0. It bundled five breaking changes and two
new features — the breaking changes were things v1.x had promised to avoid, so
this is where they landed. Every one came with a mechanical migration recipe.
This is also where path parameters moved to Go's standard `ServeMux` and the
modern typed-handler API settled in.

---

## Earlier releases

Short notes for the pre-2.0 line. The full detail is in the
[full CHANGELOG](https://github.com/suryakencana007/espresso/blob/main/CHANGELOG.md).

| Version | Date | What you got |
|---------|------|--------------|
| v1.5.0 | 10 May 2026 | `JSON[T].Cookies` — set cookies alongside JSON responses |
| v1.4.0 | 20 Apr 2026 | The `validator/` package — struct-tag validation |
| v1.3.0 | 19 Apr 2026 | WebSocket support (`espresso.WS`) |
| v1.2.0 | 9 Apr 2025 | Fluent OpenAPI builder (`openapi.New()`) |
| v1.1.0 | 8 Apr 2025 | Cookie and file-upload extractors, plus SSE |
| v1.0.2 | Jan 2025 | The `Lungo` handler for context + two extractors |
| v1.0.1 | Dec 2024 | The documentation site |
| v1.0.0 | Dec 2024 | First release — router, handlers, extractors, middleware, pooling |
