# Migrating from Espresso v2.0 to v2.1

v2.1 is a **minor** release that finishes the work v2.0 deliberately
deferred and ships one ergonomics nick that turned up while writing the
v1 → v2 guide. The module path is unchanged, the dispatch and extractor
surfaces are unchanged, and most apps will not need any code changes at
all — only callers of the now-removed deprecated SSE types must rewrite
to compile.

For the v1.x → v2.0 jump (module path bump, legacy error constructors,
`Ristretto` signature change, etc.), see
[`docs/migration-v1-to-v2.md`](./migration-v1-to-v2.md) first — that
content is not re-covered here.

If a section says **"Required"**, your build will break in v2.1 until
you apply it. **"Recommended"** means the v2.0 pattern still compiles
but the v2.1 form is cleaner. **"Informational"** means there's no
action to take — listed so the migration story is complete.

---

## Five-Minute Upgrade Checklist

For an app already on v2.0 through the public API only:

1. **Bump the dependency** —
   `go get github.com/suryakencana007/espresso/v2@v2.1.0`.
2. **If you imported `SSE`, `SSEEvent`, `SSEWriter`, or `NewSSEWriter`** —
   switch to `Stream` / `StreamSimple` and `*SSEStream` (recipe below).
   This is the only required code change in v2.1.
3. **If you wired `SetDefaultValidator` with the ~10-line closure from
   the v2.0 guide** — swap for `validator.AsDefaultValidator()`
   (recipe below). Optional but trivial.
4. **If you wrote per-route SSE preflight middleware** (e.g.
   `RequireAppAccess(Stream[...](...))`) — consider replacing with
   `espresso.WithPreFlight(...)` so rejections surface as real HTTP 4xx
   (recipe below). Optional; the existing middleware keeps working.
5. **`go build ./...`** — only step 2 above produces a compile error.
   The remaining steps are pure ergonomics.

Most apps are done in one sitting. Continue below for context on each
change.

---

## Required: Migrate Off the Deprecated SSE Types

The four symbols `SSE`, `SSEEvent`, `SSEWriter`, and `NewSSEWriter`
were tagged `// Deprecated:` in v1.3 and slated for removal in v2.0.
v2.0 deferred the removal so that release could stay tight; v2.1 ships
it. Callers have had two minor releases (v1.4, v1.5) plus all of v2.0
to migrate, and `staticcheck SA1019` has been guiding the way
throughout.

The replacement is the typed `Stream` / `StreamSimple` entry points
introduced in v1.3 and `*SSEStream`'s `SendText` / `SendJSON` /
`SendData` / `Comment` methods. The migration is structural rather
than mechanical (the old API exposed a low-level writer; the new API
is a handler shape), so plan to read each call site rather than
running `gofmt -r`.

### Removed Symbols

| Removed | Replacement |
|---|---|
| `espresso.SSE` (response type) | `espresso.Stream[T]` / `espresso.StreamSimple` |
| `espresso.SSEEvent` | `espresso.Event` (typed) |
| `espresso.SSEWriter` + methods | `*SSEStream` (`SendText` / `SendJSON` / `SendData` / `Comment`) |
| `espresso.NewSSEWriter` | n/a — use the typed handler entry points |

### Before (v2.0)

```go
func handler(w http.ResponseWriter, r *http.Request) {
    sse := espresso.NewSSEWriter(w)
    sse.Event("update", "hello")
    sse.EventJSON("data", map[string]int{"count": 42})
    sse.KeepAlive()
}

router.Get("/stream", handler)
```

### After (v2.1)

```go
router.Get("/stream", espresso.StreamSimple(
    func(ctx context.Context, s *espresso.SSEStream) error {
        if err := s.SendText("update", "hello"); err != nil {
            return err
        }
        if err := s.SendJSON("data", map[string]int{"count": 42}); err != nil {
            return err
        }
        return s.Comment("keepalive")
    },
    espresso.WithKeepAlive(30*time.Second),
))
```

What changed structurally:

- **Handler shape.** v2.0's deprecated path was a raw
  `http.HandlerFunc` that constructed an `SSEWriter` itself. v2.1's
  shape is `func(ctx context.Context, *SSEStream) error` — the
  framework constructs the stream, injects state, and registers the
  connection in the per-Router registry so `gracefulShutdown` can
  drain it.
- **Keepalive.** The old `KeepAlive()` was a one-shot comment frame
  the caller emitted manually. The new `WithKeepAlive(d)` option
  schedules comment-frame pings on a goroutine and stops cleanly when
  the request context is cancelled.
- **Event shape.** `SendText` / `SendJSON` / `SendData` / `Comment`
  cover the same wire format the deprecated `Event` / `EventJSON`
  emitted. There is no behaviour gap in either direction.

If your typed handler needs the extracted request body, use
`Stream[T]` instead of `StreamSimple` — the signature gains a
`*T` parameter and the rest of the migration is identical.

See [`docs/streaming.md`](./streaming.md) for the full streaming
guide, including reconnection patterns and concurrent-safe writes.

---

## Recommended: Adopt `WithPreFlight` for Stream Authorization

(Additive — not a breaking change. The existing per-route preflight
middleware pattern keeps working. This recipe is for the migration
most teams who run SSE behind authorization will want.)

In v2.0, `Stream` committed the HTTP response headers as part of
accepting the request, so a *resource not found* or *forbidden*
decision the handler wanted to surface as a real HTTP 4xx could only
be emitted as an `event: error` frame on a 200-OK stream. CDNs,
proxies, and standard REST clients don't treat that as an error.
The workaround was to put the authorization check in HTTP middleware
*before* `Stream` ran, so the 4xx happened before headers committed.

v2.1 adds `espresso.WithPreFlight(fn)` — a `StreamOption` that runs
**before** any response header writes. A non-nil return routes
through the same error pipeline JSON handlers use, so an
`*espresso.Error` (e.g. `ErrNotFound`, `ErrForbidden`) surfaces with
its declared status code and the framework's structured JSON envelope.
The closure receives the request context, so it can call
`MustGetState[T]` / `GetState[T]`.

### Before (v2.0) — preflight middleware per resource kind

```go
// middleware/apps.go
func RequireAppAccess(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        appID := r.PathValue("id")
        state := espresso.MustGetState[AppState](r.Context())
        if _, err := state.Apps.Get(r.Context(), appID); errors.Is(err, ErrNotFound) {
            espresso.ErrNotFound("app not found").WriteResponse(w)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// router setup
router.Get("/apps/{id}/logs",
    RequireAppAccess(
        espresso.Stream[AppLogReq](logHandler,
            espresso.WithKeepAlive(30*time.Second),
        ),
    ))
```

### After (v2.1) — one option, no per-resource middleware

```go
router.Get("/apps/{id}/logs", espresso.Stream[AppLogReq](logHandler,
    espresso.WithPreFlight(func(ctx context.Context) error {
        appID := espresso.MustGetState[AppState](ctx).RequestedAppID(ctx)
        if _, err := state.Apps.Get(ctx, appID); errors.Is(err, ErrNotFound) {
            return espresso.ErrNotFound("app not found")
        }
        return nil
    }),
    espresso.WithKeepAlive(30*time.Second),
))
```

The Barista `RequireAppAccess` / `RequireDeploymentAccess` middleware
collapses into one `WithPreFlight` call per route. The closure is the
same shape — context in, error out — so the body of the check moves
across verbatim.

### Design notes

- **No `Req` parameter.** The closure signature is the simple
  `func(ctx context.Context) error`. The typed extracted `Req` is
  deliberately **not** threaded into pre-flight — pre-flight checks
  stay tied to context-derivable identity / authorization state. For
  body-shape validation, return errors from `Extract` as usual.
- **Zero overhead on the happy path.** Existing `Stream[T]` /
  `StreamSimple` callers that don't opt in pay nothing.
- **Errors flow through `writeHandlerError`.** A return of
  `espresso.ErrNotFound("...")` produces the framework's standard
  `{"error":{"code":"NOT_FOUND","message":"...","request_id":"..."}}`
  shape with status 404. Any non-`*espresso.Error` becomes a generic
  500.

See [`docs/streaming.md`](./streaming.md#rejecting-requests-before-the-stream-opens)
for the canonical pattern and the Barista migration note.

---

## Recommended: Use `validator.AsDefaultValidator()`

(Additive — not a breaking change. The v2.0 inline closure keeps
working unchanged.)

The v2.0 auto-validate hook
([`docs/migration-v1-to-v2.md`](./migration-v1-to-v2.md#new-opt-in-auto-validation-via-setdefaultvalidator))
required every user to write the same ~10-line closure that wraps
`validator.Struct` and converts `espresso.FieldErrors` into
`espresso.ValidationErrors`. v2.1 ships that closure as a helper.

### Before (v2.0) — ~10 lines

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

### After (v2.1) — 1 line

```go
func init() {
    espresso.SetDefaultValidator(validator.AsDefaultValidator())
}
```

The behaviour is byte-identical to the v2.0 closure — invalid input
produces an `*espresso.Error` with status 400 and code
`VALIDATION_ERROR`, valid input returns nil. The helper is the
most-common-case shortcut, not a configuration surface; users who
need a different error code, extra detail keys, or a custom mapper
keep writing the inline closure.

---

## Module Path

Unchanged. v2.x stays on `github.com/suryakencana007/espresso/v2`.
`go get github.com/suryakencana007/espresso/v2@v2.1.0` is the only
dependency-management step. No `gofmt -r` import rewrite is needed
for the v2.0 → v2.1 jump (it was only needed for v1 → v2).

---

## Informational: Refreshed Framework Comparison Benchmarks

The three "Framework Comparison" tables in [`README.md`](../README.md#framework-comparison)
have been re-run against v2.1.x. The absolute numbers run roughly
2-3x higher than the v1.4 publication across **all four** frameworks
(Gin, Echo, Espresso, Fiber). This is a benchmark-runner hardware
shift — the v1.4 baseline ran on an Intel Core Ultra 7 155H; the
v2.1 refresh ran on an AMD Ryzen 7 4800H, with Go 1.25.6. Allocations
and B/op are essentially unchanged for the competitor frameworks and
only marginally up for Espresso, which rules out an allocation
regression.

There is no Espresso dispatch slowdown between v2.0 and v2.1 — the
relative ordering (Gin ≤ Echo ≤ Espresso ≪ Fiber) is unchanged. See
[`bench/README.md`](../bench/README.md) for the hardware-shift
explanation in detail.

No action required. Listed here so readers comparing the v1.4 and
v2.1 README numbers don't mistake the absolute shift for a v2.x
regression.

---

## Getting Help

Open an issue at
[github.com/suryakencana007/espresso](https://github.com/suryakencana007/espresso/issues).
Reference the section heading from this guide in your issue title so
we can route quickly.

For the rationale behind each change, see the linked PRs in
[`CHANGELOG.md`](../CHANGELOG.md) under the `[Unreleased]` section
(v2.1.0 entries) — specifically PR #30 (SSE removal), PR #33
(`WithPreFlight`), PR #31 (`AsDefaultValidator`), and PR #32 (bench
refresh).
