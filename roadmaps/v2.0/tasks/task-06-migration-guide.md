# Task 6: v1 → v2 Migration Guide

**Priority:** 📦 Meta
**Estimated Effort:** 2 days
**Dependencies:** Tasks 1-5 substantially complete


> **Status: ✅ Shipped 2026-05-10.** Delivered via #25 (+#26 API ref sync).

## Context

A major version bump is pointless without a clear migration path. v2.0's value to existing v1.x users depends on whether an afternoon's work upgrades their codebase. This task produces `docs/migration-v1-to-v2.md` — a single authoritative document covering every breaking change in v2.0.

## Acceptance Criteria

- [x] `docs/migration-v1-to-v2.md` exists and is linked from the docs nav.
- [x] Every breaking change introduced by Tasks 1-5 has an entry.
- [x] Each entry has: **Before** code snippet, **After** code snippet, and — where the transformation is mechanical — a **sed** / `gofmt -r` recipe.
- [x] A "Five-minute upgrade" checklist at the top for readers who just want the TL;DR.
- [x] A "Known incompatibilities" section listing any behavioral changes that are intentional but not mechanically safe to rewrite.
- [x] The module path bump (`github.com/suryakencana007/espresso` → `.../espresso/v2`) has its own dedicated section with the `go mod edit` commands.

## Technical Approach

### Step 6.1 — Collect Migration Entries from Each Task

Each Task 1-5 PR already wrote its migration entry inside the PR description. Task 6 harvests those into the canonical document, normalizes formatting, and sorts by "impact on a typical app" rather than by task number.

### Step 6.2 — Document Structure

```markdown
# Migrating from Espresso v1 to v2

## Five-Minute Upgrade Checklist

1. Update `go.mod`: `go get github.com/suryakencana007/espresso/v2@v2.0.0`
2. Rewrite imports: `gofmt -r '"github.com/suryakencana007/espresso" -> "github.com/suryakencana007/espresso/v2"' -w .`
3. Run the sed recipes in each section below for the symbols you use
4. `go build ./...` → fix what breaks
5. Run your test suite

## Module Path Change

[go mod recipe]

## Stream Registries Are Per-Router

[code snippets + rationale from Task 1]

## Removed: `ErrorResponse`, `NewBadRequest`, `NewInternal`, `SSEWriter`, `SSE`

[code snippets from Task 2]

## Handler Cache Is Now Bounded

[description + opt-in size config from Task 3]

## `Validation` Is Now Generic

[before/after from Task 4]

## New: Opt-In Auto-Validation via `DefaultValidator`

[opt-in wiring from Task 5 — listed here for completeness even though not breaking]

## Known Incompatibilities

- [Anything behavioral that's not mechanical]

## Getting Help

- Open an issue at github.com/suryakencana007/espresso
- Reference this guide's section number in your issue
```

### Step 6.3 — Validate by Running the Recipes

Pick a v1.x-era sample — `cmd/example/main.go` at the `v1.3.0` tag works — and apply every recipe mechanically. If the result compiles and runs, the guide is correct.

### Step 6.4 — Publish

Drop the file into `docs/migration-v1-to-v2.md`. Add a link from:

- Top-level `README.md` under a "Upgrading" section
- `docs/index.md` as a sidebar entry
- GitHub release notes for v2.0.0

## Tests Required

This is a documentation task. The "tests" are:

- The sample-codebase migration described in Step 6.3.
- A colleague (or AI reviewer) reads the guide cold and reports any confusion.
- Every `gofmt -r` / sed recipe in the guide is actually run against the test codebase during review.

## Breaking Changes

None — this task only produces documentation.

## Definition of Done

- Guide lives at `docs/migration-v1-to-v2.md`
- Every Task 1-5 breaking change has an entry
- Module-path section is first and cannot be missed
- Linked from README + docs index + release notes
- Verified by running all recipes against a v1.x sample
