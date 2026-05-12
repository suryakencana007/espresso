# Task 6: CHANGELOG & v2.1.0 Release

**Priority:** 📦 Meta
**Estimated Effort:** 0.5 day
**Dependencies:** All other tasks must be merged


> **Status: ✅ Shipped 2026-05-12.** Delivered via #35 — tagged v2.1.0.

## Context

Final gate before tagging v2.1.0. Promotes `[Unreleased]` to `[2.1.0] - <date>`, bumps the version chip + `package.json`, runs the full quality-gate set, tags, publishes the GitHub release. Single atomic commit (mirrors v2.0 task-07).

## Acceptance Criteria

- [x] `CHANGELOG.md` has a complete `[2.1.0] - 2026-MM-DD` section covering Tasks 1, 2, 3 (and noting the Task 4 bench refresh + Task 5 migration guide as `Documentation` entries).
- [x] `package.json` version bumped to `2.1.0`.
- [x] `docs/.vitepress/config.ts` version chip updated to `v2.1.0`.
- [x] `[Unreleased]` retained empty for v2.2.x work.
- [x] `go test ./... -race` clean.
- [x] `golangci-lint run ./...` clean.
- [x] `bench/` module compiles cleanly.
- [x] Git tag `v2.1.0` created and pushed.
- [x] GitHub release published with the `[2.1.0]` body + link to `docs/migration-v2-to-v2.1.md`.

## Technical Approach

### Step 6.1 — Verify All Tasks Complete

Walk the v2.1 roadmap. Every task file's Acceptance Criteria boxes are ticked.

- [x] Task 1: Remove deprecated SSE types
- [x] Task 2: Stream pre-flight phase
- [x] Task 3: validator.AsDefaultValidator()
- [x] Task 4: Bench refresh
- [x] Task 5: Migration guide v2.0 → v2.1

### Step 6.2 — CHANGELOG

Promote `[Unreleased]` content into `[2.1.0] - 2026-MM-DD`. Expected shape:

```markdown
## [Unreleased]

## [2.1.0] - 2026-MM-DD

A maintenance release: pays off the v2.0 deferred SSE-removal debt,
closes Barista F-02 (Stream pre-flight phase), and ships one validator
ergonomics nick. See [`docs/migration-v2-to-v2.1.md`](docs/migration-v2-to-v2.1.md)
for upgrade recipes.

### Removed (BREAKING)

- **Deprecated SSE types removed** (#NN). `SSE`, `SSEEvent`, `SSEWriter`,
  `NewSSEWriter` are gone. Replace with `Stream` / `StreamSimple` and
  `*SSEStream`. These were tagged `// Deprecated:` since v1.3, so users
  have had two minor releases plus all of v2.0 to migrate. Recipes in
  the migration guide.

### Added

- **`Stream` pre-flight phase** (#NN, closes Barista F-02). New
  `WithPreFlight(fn)` option lets `Stream[T]` / `StreamSimple` handlers
  return `*espresso.Error` before headers commit. A "resource not found"
  decision now surfaces as a real HTTP 404 with structured JSON body
  instead of a 200-OK SSE stream containing an `event: error` frame.
  Backward-compatible — existing handlers that don't pass `WithPreFlight`
  see no change.
- **`validator.AsDefaultValidator()`** (#NN). Returns the canonical
  `validator.Struct` + `FieldErrors → ValidationErrors` adapter for use
  with `espresso.SetDefaultValidator`. Replaces the ~10-line closure the
  v2.0 example shipped with a one-liner.

### Documentation

- **`docs/migration-v2-to-v2.1.md`** (#NN, Task 5) covers the v2.1
  deltas; cross-linked from the docs nav alongside the v1→v2 guide.
- **Framework benchmarks refreshed** (#NN, Task 4) — README "Framework
  Comparison" tables run against v2.1.x. No methodology changes.
```

### Step 6.3 — Bumps

```
package.json:                        "1.5.0" → wait, current is "2.0.0" → "2.1.0"
docs/.vitepress/config.ts version:   "v2.0.0" → "v2.1.0"
```

### Step 6.4 — Final Quality Gates

```bash
go test ./... -race
golangci-lint run ./...
go test -tags=integration ./tests/integration/...
cd bench && go test -bench . -benchmem -benchtime=3s -count=1 && cd ..
```

All four pass before tagging.

### Step 6.5 — Tag and Release

```bash
git tag v2.1.0
git push origin v2.1.0

gh release create v2.1.0 \
    --title "v2.1.0 — Finish the Pull" \
    --notes-from-tag
# Or paste the [2.1.0] CHANGELOG body via --notes-file or here-doc.
```

GitHub release body: the `[2.1.0]` CHANGELOG section + a link to `docs/migration-v2-to-v2.1.md` + a "Full Changelog" compare link from `v2.0.0`.

### Step 6.6 — Post-Release Cleanup

- Mark v2.1 roadmap retrospective (mirror what we did for v2.0 in PR #28).
- Update `roadmaps/USAGE_ESPRESSO.md` to mark F-02 as `(closed)` referencing the v2.1 task-02 PR.
- Smoke test: `go get github.com/suryakencana007/espresso/v2@v2.1.0` in a throwaway project.
- Notify Barista — F-02 is closed; they can retire `RequireAppAccess` / `RequireDeploymentAccess` middleware in favor of `WithPreFlight`.

## Tests Required

The full test suite, plus the bench module spot-check.

## Definition of Done

- [x] `CHANGELOG.md` `[2.1.0]` section finalized with the ship date.
- [x] `package.json` version bumped to `2.1.0`.
- [x] Docs version chip updated to `v2.1.0`.
- [x] Git tag `v2.1.0` pushed.
- [x] GitHub release published.
- [x] `roadmaps/USAGE_ESPRESSO.md` F-02 marked closed.
- [x] Barista pinged with the migration recipe for `RequireAppAccess` / `RequireDeploymentAccess` retirement.
