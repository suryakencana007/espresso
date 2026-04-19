# Task 7: CHANGELOG, Module Path Bump, and Release

**Priority:** 📦 Meta
**Estimated Effort:** 1 day
**Dependencies:** All other tasks must be merged

## Context

This task is the final gate before tagging v2.0.0. It is the only task that atomically changes the module path from `github.com/suryakencana007/espresso` to `github.com/suryakencana007/espresso/v2` — per Go's major-version module convention.

Everything else has been done; this task is pure release engineering.

## Acceptance Criteria

- [ ] `go.mod` module directive is `module github.com/suryakencana007/espresso/v2`
- [ ] Every internal import of `github.com/suryakencana007/espresso...` is rewritten to the `/v2` path
- [ ] `CHANGELOG.md` has a complete `[2.0.0]` section covering every task 1-5 change plus the module-path bump
- [ ] `bench/go.mod` `replace` directive still points at `../` and the replaced module line matches the new path
- [ ] `go test ./... -race` passes
- [ ] `golangci-lint run ./...` is clean
- [ ] `go build ./...` succeeds from a fresh clone
- [ ] Git tag `v2.0.0` created and pushed
- [ ] GitHub release created with release notes summarizing user-visible changes and linking the migration guide

## Technical Approach

### Step 7.1 — Verify All Tasks Are In

Before starting, confirm merge state:

- [ ] Task 1: Per-Router stream registries
- [ ] Task 2: Remove deprecated APIs
- [ ] Task 3: Handler-cache eviction
- [ ] Task 4: Typed Validation layer
- [ ] Task 5: Auto-validate on extract
- [ ] Task 6: Migration guide

If any is outstanding, stop — the release train does not leave with half the cargo.

### Step 7.2 — Module Path Rewrite

One atomic commit. Do not split.

```bash
# Rewrite the go.mod
go mod edit -module github.com/suryakencana007/espresso/v2

# Rewrite every import
find . -name '*.go' -not -path './bench/*' -exec \
    sed -i 's|github.com/suryakencana007/espresso|github.com/suryakencana007/espresso/v2|g' {} +

# Fix bench/go.mod replace directive
sed -i 's|github.com/suryakencana007/espresso =>|github.com/suryakencana007/espresso/v2 =>|' bench/go.mod
sed -i 's|github.com/suryakencana007/espresso |github.com/suryakencana007/espresso/v2 |g' bench/go.mod
# then:
cd bench && go mod tidy

# Back at root
go mod tidy
go build ./...
go test ./... -race
```

Verify nothing in the tree still references the v1 path:

```bash
grep -rn 'github.com/suryakencana007/espresso\b' --include='*.go' --include='*.mod'
# Should only return the v2-path references
```

### Step 7.3 — CHANGELOG

Using Keep-a-Changelog format, add:

```markdown
## [2.0.0] - 2026-MM-DD

### Breaking

- **Module path changed** to `github.com/suryakencana007/espresso/v2`. Update your imports.
- **Per-Router stream registries**. `defaultRegistry` and `defaultSSERegistry` are removed. See migration guide.
- **Removed deprecated APIs**: `ErrorResponse`, `NewBadRequest`, `NewInternal`, `SSEWriter`, `SSE`. See migration guide for replacements.
- **Typed `Validation[Req]` layer** replaces the `any`-typed `Validation`. Go inference handles most call sites.

### Added

- Handler-cache eviction with configurable bound + `OnEvict` hook.
- Opt-in auto-validation on extract via `espresso.DefaultValidator`.
- `cmd/example/validate/` demonstrating auto-validation.

### Changed

- Handler cache is bounded (default 1024 entries) instead of unbounded.

### Removed

- See Breaking section — specific symbols listed there.

See `docs/migration-v1-to-v2.md` for step-by-step migration.
```

Move the `[Unreleased]` section above `[2.0.0]` and leave it empty for future work.

### Step 7.4 — Documentation Touch-Up

- Bump every docs snippet showing the module path to `/v2`
- Update the VitePress version dropdown (`docs/.vitepress/config.ts` or similar) to include v2.0.0

### Step 7.5 — Tag and Release

```bash
git tag -s v2.0.0 -m "v2.0.0"
git push origin v2.0.0
```

GitHub release body: the `[2.0.0]` CHANGELOG section + link to migration guide.

## Tests Required

- Full test suite passes: `go test ./... -race`
- Lint clean: `golangci-lint run ./...`
- Bench module builds against the v2 path: `cd bench && go build ./...`
- A fresh `go get github.com/suryakencana007/espresso/v2@v2.0.0` in a throwaway project resolves cleanly after the tag is published (smoke test, done post-release).

## Breaking Changes

The module path change is itself a breaking change — users must update imports. Documented in the migration guide (Task 6).

## Definition of Done

- Tag pushed
- Release published
- `go.mod` of a freshly-cloned v2.0.0 references the `/v2` path
- Migration guide linked from the GitHub release
- CHANGELOG `[Unreleased]` is empty (ready for v2.1.x)
- Downstream projects (Barista etc.) have been pinged with upgrade instructions
