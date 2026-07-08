# Task 12: CHANGELOG + v2.4.0 Release + Migration Guide

**Priority:** 📦 Meta
**Estimated Effort:** 0.5 day
**Dependencies:** Tasks 1-11

> **Status: ✅ Shipped 2026-07-05 (v2.4.0).** Delivered via #74 — CHANGELOG [Unreleased] → [2.4.0]; package.json + docs/.vitepress/config.ts version chips bumped; docs/migration-v2.3-to-v2.4.md added; tag v2.4.0 cut from the merge commit.

## Context

Final task. Follows the atomic-release discipline established in v2.0 task-07 / v2.1 task-06 / v2.2 task-05 / v2.3 task-07: single commit promoting `[Unreleased]` → `[2.4.0]`, bumping version chips, plus a new migration guide covering the behavior changes v2.4 ships.

The behavior changes to call out in the migration guide:

- **SSE now survives default `WriteTimeout`** (Task 4). Users who worked around this via `WithWriteTimeout(0)` can remove that override.
- **`RateLimitMiddleware` no longer trusts `X-Forwarded-For` by default** (Task 8). Callers behind a reverse proxy must add `WithTrustedProxies(...)` naming the proxy CIDRs — otherwise all requests key on the proxy's IP.
- **`JSON[T].Extract` returns 413 on bodies > 1 MB** (Task 6). Callers with legitimate larger bodies must set `WithJSONBodyLimit(higher)`.
- **`CircuitBreakerLayer.FailureThreshold` semantics changed** (Task 7). Now resets on successful calls while Closed; previously accumulated over process lifetime.
- **`BrewContext` now drains in-flight requests on ctx cancellation** (Task 4). Callers who observed near-instant shutdown will see it take up to `ShutdownTimeout`.
- **`LoggingMiddleware` is now compatible with SSE and WebSocket** (Task 3). No caller action; previously mounting `LoggingMiddleware` broke stream routes.
- **Truth-in-docs sweep** (Task 10). No behavior change, but numerous doc examples now match reality — the previously-broken `docs/api/espresso.md` `Solo`/`Doppio` examples compile.

## Acceptance Criteria

- [x] `CHANGELOG.md`'s `[Unreleased]` section is promoted to `[2.4.0] - 2026-MM-DD` in a single atomic commit; a new empty `[Unreleased]` section is added on top.
- [x] `package.json`'s `"version"` field is `2.4.0`.
- [x] `docs/.vitepress/config.ts` version reference bumped to `v2.4.0`.
- [x] `docs/migration-v2.3-to-v2.4.md` exists and covers the seven behavior changes above with before/after examples for each.
- [x] `docs/.vitepress/config.ts` sidebar includes a link to the new migration guide.
- [x] The version-bump commit lands as a single atomic commit (not split across multiple PRs); tag `v2.4.0` from the merge commit.
- [x] GitHub release created (`gh release create v2.4.0`) with the `[2.4.0]` CHANGELOG body.
- [x] Post-release smoke: `go get github.com/suryakencana007/espresso/v2@v2.4.0` resolves and builds against a throwaway project exercising `WithJSONBodyLimit`, `WithTrustedProxies`, and a corrected `Solo` example.

## Technical Approach

### Step 12.1 — Promote [Unreleased] → [2.4.0]

Group entries per Keep-a-Changelog:

- **Added** — `WithJSONBodyLimit`, `WithTrustedProxies`, `WithBucketTTL`, `HalfOpenMaxProbes` on `CircuitBreaker` config, `ErrRequestEntityTooLarge`, `413` in `defaultCodeForStatus`.
- **Changed** — `RateLimitMiddleware` no longer trusts XFF by default; `JSON[T].Extract` now respects a body limit (default 1 MB); `CircuitBreaker` failure count resets on Closed-state successes; SSE now survives default `WriteTimeout`; truth-in-docs sweep (numerous docs corrected).
- **Fixed** — Six P0 correctness defects (TimeoutLayer race, TokenBucket refill starvation, LoggingMiddleware Flusher/Hijacker, BrewContext drain, OpenAPI generator race + recursive-type overflow, JSON body cap); four P1 defects (CircuitBreaker four state-machine issues, RateLimit XFF, Validator pointer fields, cmd/example goroutine-Brew antipattern).
- **Removed** — (none expected; if Task 10 deletes phantom docs symbols, count as Fixed docs, not Removed API).

### Step 12.2 — Bump version chips

In the same commit as Step 12.1:

```json
// package.json
{ "version": "2.4.0" }
```

```ts
// docs/.vitepress/config.ts
// wherever the current-version string lives; bump to "v2.4.0"
```

### Step 12.3 — Write docs/migration-v2.3-to-v2.4.md

Structure follows `docs/migration-v2-to-v2.1.md`:

```markdown
# Migrating from v2.3 to v2.4

v2.4 is a correctness release. Most code compiles unchanged. Behavior changes
listed below.

## Rate limiting no longer trusts X-Forwarded-For by default

**Before (v2.3):** `RateLimitMiddleware` would take `X-Forwarded-For`'s value
as the client identity for per-key limiting, allowing any client to spoof.

**After (v2.4):** `X-Forwarded-For` is ignored unless you opt in with
`WithTrustedProxies(cidrs ...string)`.

If you run behind a reverse proxy:

```go
r.Use(httpmiddleware.RateLimitMiddleware(
    httpmiddleware.WithLimiter(limiter),
    httpmiddleware.WithTrustedProxies("10.0.0.0/8"), // your proxy's CIDR
))
```

## JSON bodies capped at 1 MB by default

... (repeat for each of the seven changes) ...
```

### Step 12.4 — Sidebar link

`docs/.vitepress/config.ts` sidebar entries: add `{ text: 'v2.3 → v2.4', link: '/migration-v2.3-to-v2.4' }` in the "Migration" section (following the v2.1 pattern).

### Step 12.5 — Final quality gates + tag + release

```sh
go test -race -shuffle=on ./... -count=1
golangci-lint run ./...
govulncheck ./...
go test -tags=integration ./tests/integration/...
cd bench && go test -bench . -benchmem -benchtime=3s -count=1  # spot-check
```

After merge:

```sh
git checkout main && git pull
git tag v2.4.0
git push origin v2.4.0
gh release create v2.4.0 --title "v2.4.0 — Ground Truth" --notes-from-tag  # or with the [2.4.0] body inline
```

### Step 12.6 — Post-release smoke

Create a throwaway `smoke-v2.4/` project:

```go
package main
import (
    "context"
    "github.com/suryakencana007/espresso/v2"
    "github.com/suryakencana007/espresso/v2/middleware/http"
)

type CreateReq struct { Name string `json:"name"` }
type CreateRes struct { ID string `json:"id"` }

func create(ctx context.Context, req *espresso.JSON[CreateReq]) (espresso.JSON[CreateRes], error) {
    return espresso.JSON[CreateRes]{Data: CreateRes{ID: "1"}}, nil
}

func main() {
    r := espresso.Portafilter()
    r.Use(httpmiddleware.RateLimitMiddleware(
        httpmiddleware.WithLimiter(httpmiddleware.NewTokenBucketLimiter(100, 10)),
        httpmiddleware.WithTrustedProxies("127.0.0.0/8"),
    ))
    r.Post("/", create)
    r.Brew(espresso.WithAddr(":0"), espresso.WithJSONBodyLimit(1<<20))
}
```

`go mod init smoke-v2.4 && go get github.com/suryakencana007/espresso/v2@v2.4.0 && go build .` must succeed.

## Tests Required

None new — the release itself is the deliverable.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `git tag v2.4.0` pushed, GitHub release published.
- [x] Smoke project builds against `v2.4.0`.
- [x] The `roadmaps/v2.4/` retrospective marking (pre-checked boxes + ship-date banner) can happen in a follow-up PR after the release lands, following the v2.3 pattern.
