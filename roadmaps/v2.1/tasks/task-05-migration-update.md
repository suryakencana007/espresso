# Task 5: Migration Guide v2.0 → v2.1

**Priority:** 📦 Meta
**Estimated Effort:** 0.5 day
**Dependencies:** Tasks 1-4 substantially complete

## Context

`docs/migration-v1-to-v2.md` (shipped in v2.0 task-06) covers the v1 → v2.0 jump. Users on v2.0 don't need to re-read it for v2.1; they need their own short migration. This task creates `docs/migration-v2-to-v2.1.md` covering ONLY the v2.1 deltas, plus a small docs-nav update so users on either v1.x or v2.0 land in the right guide.

## Acceptance Criteria

- [ ] `docs/migration-v2-to-v2.1.md` exists.
- [ ] Sections for each v2.1 change:
  - Removed: deprecated SSE types (Task 1) — Before/After + mechanical recipe.
  - Added: `Stream` pre-flight phase (Task 2) — Before/After showing the Barista preflight-middleware pattern collapsing into a single call.
  - Added: `validator.AsDefaultValidator()` helper (Task 3) — Before/After showing the closure → helper transformation.
- [ ] A "Five-Minute Upgrade Checklist" at the top, like the v1→v2 guide.
- [ ] Cross-link from:
  - `docs/.vitepress/config.ts` Upgrading sidebar group (add a second entry alongside the v1→v2 entry).
  - `roadmaps/v2.0/README.md` "Status" banner (mention v2.1 ships these deferred items).
  - `roadmaps/v2.1/README.md` (same).
  - `README.md` "Upgrading" section (add a v2.0 → v2.1 line).
- [ ] No code changes — pure documentation.

## Technical Approach

### Step 5.1 — Outline

```markdown
# Migrating from Espresso v2.0 to v2.1

## Five-Minute Upgrade Checklist
1. `go get github.com/suryakencana007/espresso/v2@v2.1.0`
2. If you used SSE/SSEWriter/etc: switch to Stream/SSEStream (recipe below).
3. If you wired SetDefaultValidator with the closure-style adapter: swap for validator.AsDefaultValidator().
4. If you wrote per-route SSE preflight middleware: consider replacing with espresso.WithPreFlight (recipe below).
5. go build ./... → fix what breaks (likely only #2 above).

## Removed: Deprecated SSE Types
[Before/After + recipe]

## Added: Stream Pre-Flight Phase
[Before/After: Barista's RequireAppAccess pattern → WithPreFlight]

## Added: validator.AsDefaultValidator()
[Before/After: closure → helper]

## Module Path
Unchanged. v2.x stays on github.com/suryakencana007/espresso/v2.

## Getting Help
[Same template as v1→v2]
```

### Step 5.2 — Recipes

For the SSE removal, the migration is structural (not a `gofmt -r` rewrite). Provide a worked example:

```go
// Before (v2.0)
func handler(w http.ResponseWriter, r *http.Request) {
    sse := espresso.NewSSEWriter(w)
    sse.Event("update", "hello")
    sse.EventJSON("data", map[string]int{"count": 42})
    sse.KeepAlive()
}
router.Get("/stream", handler)

// After (v2.1)
router.Get("/stream", espresso.StreamSimple(func(ctx context.Context, s *espresso.SSEStream) error {
    if err := s.SendText("update", "hello"); err != nil {
        return err
    }
    if err := s.SendJSON("data", map[string]int{"count": 42}); err != nil {
        return err
    }
    return s.Comment("keepalive")
}, espresso.WithKeepAlive(30*time.Second)))
```

For the pre-flight phase, the recipe collapses Barista's RequireAppAccess middleware:

```go
// Before (v2.0)
router.Get("/apps/{id}/logs",
    middleware.RequireAppAccess(  // resolves App via DB; returns 404 if missing
        espresso.Stream[AppLogReq](logHandler, espresso.WithKeepAlive(30*time.Second)),
    ))

// After (v2.1)
router.Get("/apps/{id}/logs", espresso.Stream[AppLogReq](logHandler,
    espresso.WithPreFlight(func(ctx context.Context, req AppLogReq) error {
        if _, err := state.Apps.Get(ctx, req.ID); errors.Is(err, ErrNotFound) {
            return espresso.ErrNotFound("app not found")
        }
        return nil
    }),
    espresso.WithKeepAlive(30*time.Second),
))
```

### Step 5.3 — Docs Nav

`docs/.vitepress/config.ts` currently has:

```ts
{
  text: "Upgrading",
  collapsed: false,
  items: [
    { text: "v1 → v2 Migration", link: "/migration-v1-to-v2" },
  ],
},
```

Add the v2.0 → v2.1 entry:

```ts
{
  text: "Upgrading",
  collapsed: false,
  items: [
    { text: "v1 → v2 Migration", link: "/migration-v1-to-v2" },
    { text: "v2.0 → v2.1 Migration", link: "/migration-v2-to-v2.1" },
  ],
},
```

The top-level nav has `{ text: "v1 → v2", link: "/migration-v1-to-v2" }`. Replace with a dropdown:

```ts
{
  text: "Migrations",
  items: [
    { text: "v1 → v2", link: "/migration-v1-to-v2" },
    { text: "v2.0 → v2.1", link: "/migration-v2-to-v2.1" },
  ],
},
```

## Tests Required

None — pure documentation. Validation:

- All cross-links resolve.
- The recipes compile when copied into a scratch project (manual sanity).
- A reader on v2.0 lands on the right guide via the nav.

## Breaking Changes

None — this task ships documentation only.

## Definition of Done

- [ ] `docs/migration-v2-to-v2.1.md` written and cross-linked.
- [ ] `docs/.vitepress/config.ts` nav and sidebar updated.
- [ ] `README.md` Upgrading section gains a v2.0 → v2.1 line.
- [ ] PR description references each Task 1/2/3 PR by number for traceability.
