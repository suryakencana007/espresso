# Migrating from Espresso v1 to v2

This guide consolidates every breaking change and notable addition from
v2.0 into one place. Follow it section-by-section or jump to the parts
that match the symbols your codebase uses.

If a section says **"Mechanical fix"**, the rewrite is safe to run
unconditionally — the replacement has shipped on equivalent behavior
since v1.x. If it says **"Manual"**, you'll need to read each call site.

---

## Five-Minute Upgrade Checklist

For an app that uses Espresso through the public API only:

1. **Bump the module path** — see [Module Path Change](#module-path-change).
2. **Mechanically rewrite legacy error constructors** —
   `gofmt -r 'espresso.BadRequest(x) -> espresso.ErrBadRequest(x)' -w .`
   (and similar for `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`,
   `InternalError`, `ServiceUnavailable`).
3. **Add `context.Context` to every `Ristretto` handler.** No automation;
   `go build` flags every call site with a clear type-mismatch message.
4. **`go build ./...`** — fix anything else that breaks. The remaining
   breaks are likely (a) external callers of `defaultRegistry` /
   `defaultSSERegistry` (rare; see [Per-Router Registries](#per-router-stream-registries-internal-but-may-leak-out));
   or (b) explicit type parameters on the `Validation` layer config
   (Go infers them in most call sites; see
   [Generic Validation](#validation-is-now-generic-validationreq)).
5. **Run your test suite**, paying attention to anything that touched
   the now-bounded handler-cache or asserted on the deprecated SSE
   types (`SSE`, `SSEWriter`, `SSEEvent`, `NewSSEWriter` — still present,
   slated for removal in v2.1).

Most apps are done in one sitting. Continue below for context on each
change.

---

## Module Path Change

Per Go's major-version convention:

```
v1.x: github.com/suryakencana007/espresso
v2.x: github.com/suryakencana007/espresso/v2
```

**Mechanical fix:**

```bash
# Update go.mod and pull v2.0.0
go mod edit -require=github.com/suryakencana007/espresso/v2@v2.0.0
go mod edit -droprequire=github.com/suryakencana007/espresso
go mod tidy

# Rewrite imports across your tree
gofmt -r '"github.com/suryakencana007/espresso" -> "github.com/suryakencana007/espresso/v2"' -w .
gofmt -r '"github.com/suryakencana007/espresso/extractor" -> "github.com/suryakencana007/espresso/v2/extractor"' -w .
gofmt -r '"github.com/suryakencana007/espresso/middleware/http" -> "github.com/suryakencana007/espresso/v2/middleware/http"' -w .
gofmt -r '"github.com/suryakencana007/espresso/middleware/service" -> "github.com/suryakencana007/espresso/v2/middleware/service"' -w .
gofmt -r '"github.com/suryakencana007/espresso/openapi" -> "github.com/suryakencana007/espresso/v2/openapi"' -w .
gofmt -r '"github.com/suryakencana007/espresso/pool" -> "github.com/suryakencana007/espresso/v2/pool"' -w .
gofmt -r '"github.com/suryakencana007/espresso/validator" -> "github.com/suryakencana007/espresso/v2/validator"' -w .
```

The package alias `espresso` (used in `import espresso "..."`) does not
need to change.

---

## Removed: Legacy Error Constructors

v1.x carried two parallel constructor families: the lowercase-prefix
form (`BadRequest`, `Unauthorized`, etc.) and the `Err`-prefix form
(`ErrBadRequest`, `ErrUnauthorized`, etc.). Their godoc said *"prefer
`Err*` for new code"* but they lacked the machine-readable
`// Deprecated:` tag, so users got no migration runway.

v2.0 removes the lowercase-prefix forms.

| Removed | Replacement |
|---|---|
| `espresso.BadRequest(msg, ...)` | `espresso.ErrBadRequest(msg)` |
| `espresso.Unauthorized(msg, ...)` | `espresso.ErrUnauthorized(msg)` |
| `espresso.Forbidden(msg, ...)` | `espresso.ErrForbidden(msg)` |
| `espresso.NotFound(msg, ...)` | `espresso.ErrNotFound(msg)` |
| `espresso.Conflict(msg, ...)` | `espresso.ErrConflict(msg)` |
| `espresso.InternalError(msg, ...)` | `espresso.ErrInternal(msg)` |
| `espresso.ServiceUnavailable(msg, ...)` | `espresso.ErrServiceUnavailable(msg)` |
| `espresso.ErrorResponse` (type alias) | `*espresso.Error` directly |

The legacy constructors took an optional `details ...any` variadic; the
`Err*` constructors do not. Callers that used the variadic should chain
`.WithDetail(key, value)` or `.WithDetails(map[string]any{...})`:

**Before (v1.x):**

```go
return espresso.BadRequest("invalid email", map[string]any{"field": "email"})
```

**After (v2.0):**

```go
return espresso.ErrBadRequest("invalid email").WithDetail("field", "email")
```

**Mechanical fix (no details):**

```bash
gofmt -r 'espresso.BadRequest(x) -> espresso.ErrBadRequest(x)' -w .
gofmt -r 'espresso.Unauthorized(x) -> espresso.ErrUnauthorized(x)' -w .
gofmt -r 'espresso.Forbidden(x) -> espresso.ErrForbidden(x)' -w .
gofmt -r 'espresso.NotFound(x) -> espresso.ErrNotFound(x)' -w .
gofmt -r 'espresso.Conflict(x) -> espresso.ErrConflict(x)' -w .
gofmt -r 'espresso.InternalError(x) -> espresso.ErrInternal(x)' -w .
gofmt -r 'espresso.ServiceUnavailable(x) -> espresso.ErrServiceUnavailable(x)' -w .
```

Call sites that passed `details` need a manual rewrite to `.WithDetail`
or `.WithDetails`.

---

## `Ristretto` Now Takes `context.Context`

The v1.x signature `Ristretto[Res any](fn func() Res)` was a foot-gun:
the prototypical use case is `/healthz`, but with no `context.Context`
the handler couldn't reach `MustGetState[T]` and immediately had to drop
back to `HandlerCtx` to verify the DB. v2.0 fixes the signature.

**Before (v1.x):**

```go
func ping() espresso.Text {
    return espresso.Text{Body: "pong"}
}

router.Get("/ping", espresso.Ristretto(ping))
```

**After (v2.0):**

```go
func ping(_ context.Context) espresso.Text {
    return espresso.Text{Body: "pong"}
}

router.Get("/ping", espresso.Ristretto(ping))
```

**Manual fix.** Function-literal parameter signatures vary too much for a
mechanical rewrite. `go build` flags every call site:

```
type func() Text of (func() Text literal) does not match func(context.Context) Res
```

If you want a 0-arg handler with no error and no context, use
`HandlerNoReqNoErr` (still available, unchanged).

If your handler must return an error, use `Doppio` or `HandlerCtx`
instead. `Ristretto` keeps its no-error stance.

---

## Per-Router Stream Registries (internal, but may leak out)

In v1.x, open WebSocket connections and SSE streams were tracked in
package-level globals (`defaultRegistry`, `defaultSSERegistry`). v2.0
removes both globals; each `*Router` now owns its registries.

Most apps don't touch these directly. If you do, the migration is:

| Removed | Replacement |
|---|---|
| `defaultRegistry.len()` | `router.wsReg.len()` |
| `defaultRegistry.closeAll(code, reason)` | `router.wsReg.closeAll(code, reason)` |
| `defaultSSERegistry.len()` | `router.sseReg.len()` |
| `defaultSSERegistry.closeAll(reason)` | `router.sseReg.closeAll(reason)` |

The wrappers themselves (`espresso.WebSocketSimple`, `espresso.StreamSimple`,
etc.) keep their package-level form — they look up the per-Router
registry via the request context. If you wire the wrappers into a
non-Espresso `http.ServeMux`, registry registration becomes a silent
no-op (the connection still works; you just lose graceful-shutdown
integration for those connections).

The change makes multi-router programs (multi-tenant hosts, embedding
libraries) shut down independently. Two `Portafilter()` instances in the
same process no longer share connection state.

---

## Handler Cache Is Now Bounded

The v1.x reflection cache was an unbounded `sync.Map`. v2.0 replaces it
with an LRU-bounded cache. **Default upper bound: 1024 entries.**

For static apps (routes registered at startup), the cache stays well
under the default bound and never evicts — no observable change. The
LRU bookkeeping adds about 24 ns per registration but does not touch
the per-request hot path.

For apps that synthesize handler types at runtime (plugin hosts,
per-tenant codegen, `reflect.MakeFunc`), v2.0 prevents unbounded memory
growth. New tuning surface:

```go
// Resize the bound. Pass 0 or negative to reset to the default.
espresso.SetHandlerCacheSize(2048)

// Observe evictions for telemetry.
espresso.OnHandlerCacheEvict(func(t reflect.Type) {
    metrics.Inc("handler_cache.evict", "type", t.String())
})
```

In-flight requests are unaffected by eviction: `*handlerInfo` values are
immutable, and request-side handlers hold a pointer captured at
registration time.

This change is **additive** — apps that don't call the new setters see
only the bound itself.

---

## `Validation` Is Now Generic — `Validation[Req](Validator[Req])`

The v1.x signature was `Validation(validator any) LayerConfig`. The
`any` predated Go generics and meant `buildLayer` had to runtime-assert
the validator's type; mismatched validators failed in opaque ways.

v2.0 tightens this to a generic:

```go
func Validation[Req any](validator servicemiddleware.Validator[Req]) LayerConfig
```

Now: a `Validator[Req1]` applied to a handler with `Req2` panics at
**registration time** with a descriptive message naming both types.

**Before (v1.x):**

```go
validator := servicemiddleware.ValidatorFunc[*JSON[CreateUserReq]](
    func(ctx context.Context, req *JSON[CreateUserReq]) error { ... },
)
Validation(validator)
```

**After (v2.0):**

```go
validator := servicemiddleware.ValidatorFunc[*JSON[CreateUserReq]](
    func(ctx context.Context, req *JSON[CreateUserReq]) error { ... },
)
Validation(validator)                          // Go infers Req
Validation[*JSON[CreateUserReq]](validator)    // explicit form (optional)
```

**Usually no syntactic change.** Go infers the type parameter from the
validator argument. The explicit form is recommended for readability
when the validator is constructed elsewhere.

If you wrote a `Validator[any]` that accepted *any* request type at
runtime, that pattern no longer compiles — the type parameter is now
required to match. Replace it with a type-specific validator per
handler.

---

## New: Opt-In Auto-Validation via `SetDefaultValidator`

(Additive — not a breaking change. Listed here so the migration story is
complete.)

A new package-level hook lets the framework auto-validate decoded
extraction values:

```go
import (
    "github.com/suryakencana007/espresso/v2"
    "github.com/suryakencana007/espresso/v2/validator"
)

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

When set, every built-in extractor — `JSON[T]`, `Query[T]`, `Path[T]`,
`Form[T]`, `Header[T]`, `Cookie[T]`, `XML[T]`, `Multipart[T]`,
`RawBodyWithHeaders[H]` — runs the validator after decode and rejects
malformed payloads as a structured 400 before the handler sees them.

Default is **nil** — v1.x behavior is preserved exactly when the hook
is unset. Hot-path overhead with the hook off is one atomic load
(2.24 ns/op, 0 allocations).

Fully composable with the `Validation[Req]` service layer — see
[`docs/guide/validation.md`](guide/validation.md#auto-validate-on-extract-since-v20).

---

## New: Bounded Handler Cache (additive)

See [Handler Cache Is Now Bounded](#handler-cache-is-now-bounded) above.
Listed twice in the table-of-contents-equivalent because it has both an
"old behavior changed" angle (the bound now exists) and a "new API"
angle (`SetHandlerCacheSize`, `OnHandlerCacheEvict`).

---

## Still Present, Deprecated — Removal Slated for v2.1

The following types are tagged `// Deprecated:` and continue to work in
v2.0. They will be removed in v2.1 (or later).

- `espresso.SSE` (response type) — use `espresso.Stream[T]` /
  `espresso.StreamSimple` and `*SSEStream`.
- `espresso.SSEWriter` and `espresso.NewSSEWriter` — use
  `*SSEStream`'s `SendText` / `SendJSON` / `SendData` / `Comment`.
- `espresso.SSEEvent` (the v1.0 fields-only struct) — use
  `espresso.Event` (the typed Event type used by `*SSEStream`).

If your code uses any of these, your build will keep working; the
`staticcheck SA1019` warning will guide migration when you're ready.

---

## Wire-Format Reminder (from v1.4, not new in v2.0)

This isn't a v2.0 change but it's worth restating because it's the only
recent wire-format change anyone might have missed.

Since **v1.4**, extractor failures (malformed JSON body, missing
required header, etc.) return JSON instead of `text/plain`:

```json
{"error":{"code":"BAD_REQUEST","message":"...","request_id":"..."}}
```

The HTTP status code is unchanged (still 400, 422, etc.). Most clients
treat 4xx as an error regardless of body shape; if your client parses
4xx bodies as text, switch to JSON parsing. This change is locked by
`TestWithLayers_ExtractorErrorReturnsStructuredJSON` so it cannot
regress.

---

## Known Incompatibilities

These are intentional behavioral changes that no recipe can mechanically
fix.

- **`Ristretto`'s no-error contract is preserved.** If you used
  `Ristretto` for handlers that returned an error via panic-recovery or
  similar, you must move to `Doppio` or `HandlerCtx`. The v2.0
  `Ristretto` does not return an error.
- **Mismatched `Validation[Req1]` on a handler with `Req2` now panics
  at registration.** v1.x silently skipped the validation in some
  composition paths; v2.0 makes the misconfiguration loud.
- **Per-Router shutdown means router A's `gracefulShutdown` no longer
  closes router B's connections.** Multi-tenant programs that relied
  on the implicit "shut down everything" behavior need to call
  shutdown on each Router explicitly.

---

## Getting Help

Open an issue at
[github.com/suryakencana007/espresso](https://github.com/suryakencana007/espresso/issues).
Reference the section heading from this guide in your issue title so we
can route quickly.

For the rationale behind each change, see the linked PRs in
[`CHANGELOG.md`](../CHANGELOG.md) under the `[2.0.0]` section.
