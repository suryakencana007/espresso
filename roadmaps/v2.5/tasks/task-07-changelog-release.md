# Task 7: CHANGELOG + v2.5.0 Release + Migration Guide

**Priority:** 📦 Meta
**Estimated Effort:** 0.5 day
**Dependencies:** Tasks 1–6

> **Status: ✅ Shipped 2026-11-15 (v2.5.0).** Delivered via #84 —
> CHANGELOG `[Unreleased]` → `[2.5.0]`; `package.json` +
> `docs/.vitepress/config.ts` version chips bumped;
> `docs/migration-v2.4-to-v2.5.md` added; tag `v2.5.0` cut from the merge
> commit `bb50ab5`; GitHub release published.

## Context

Final task. Follows the atomic-release discipline of v2.0 task-07 / v2.1
task-06 / v2.2 task-05 / v2.3 task-07 / v2.4 task-12: a single commit
promoting `[Unreleased]` → `[2.5.0]`, bumping the version chips, and adding a
migration guide.

Unlike v2.4, v2.5 has **no breaking changes**. The migration guide's job is to
say so clearly and document the one observable behavior change (the circuit
breaker probe bound) plus the new API.

## Acceptance Criteria

- [x] `CHANGELOG.md`'s `[Unreleased]` is promoted to `[2.5.0] - 2026-11-15`
      in a single atomic commit; a new empty `[Unreleased]` section is added.
- [x] `package.json`'s `"version"` is `2.5.0`.
- [x] `docs/.vitepress/config.ts` version chip is `v2.5.0`.
- [x] `docs/migration-v2.4-to-v2.5.md` exists, covers the new API and the
      probe-bound fix, and states "no action required".
- [x] `docs/.vitepress/config.ts` nav + sidebar link the new migration guide.
- [x] Tag `v2.5.0` cut from the merge commit.
- [x] GitHub release created with the `[2.5.0]` CHANGELOG body.
- [x] Post-release smoke build against `v2.5.0`.

## Technical Approach

### Step 7.1 — Promote `[Unreleased]`

Group per Keep-a-Changelog:

- **Added** — `(*Error).WithHeader`, `(*Error).WithHeaderAdd`.
- **Fixed** — circuit-breaker half-open probe bound; sonic v1.15.3 for Go 1.27.
- **Changed** — CI test/vulncheck on Go 1.27.

### Step 7.2 — Bump version chips

`package.json` → `2.5.0`; `docs/.vitepress/config.ts` nav chip → `v2.5.0` and
the migration link in the nav dropdown + sidebar "Upgrading" section.

### Step 7.3 — Write `docs/migration-v2.4-to-v2.5.md`

Structure: five-minute checklist ("`go build ./...` unchanged"), the new
error-header API with a runnable example, the reserved-header table, the
circuit-breaker behavior change, and the sonic/Go 1.27 note.

### Step 7.4 — Tag + release

```sh
git checkout main && git pull
git tag v2.5.0
git push origin v2.5.0
gh release create v2.5.0 --title "v2.5.0 — Headers & Recovery" \
  --notes-file <([awk '/^## \[2\.5\.0\]/{f=1} /^## \[2\.4\.0\]/{f=0} f' CHANGELOG.md])
```

### Step 7.5 — Post-release smoke

A throwaway project exercising `WithHeader`/`WithHeaderAdd`:

```sh
go mod init smoke-v2.5
go get github.com/suryakencana007/espresso/v2@v2.5.0
go build .
```

## Tests Required

None new — the release itself is the deliverable.

## Definition of Done

- [x] All acceptance criteria ticked.
- [x] Tag pushed; GitHub release published.
- [x] Smoke project builds against `v2.5.0`.
- [x] `roadmaps/v2.5/` back-fill (this directory) lands as a follow-up.
