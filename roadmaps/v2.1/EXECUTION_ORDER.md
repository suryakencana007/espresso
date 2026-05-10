# Execution Order for v2.1.0

This document provides the recommended execution order for the v2.1 roadmap tasks. The schedule fits in approximately one focused week.

## Overview

```
Day 1:    Task 1 (remove deprecated SSE) — narrow, mechanical
Day 2-3:  Task 2 (Stream pre-flight phase) — touches sse.go and tests
Day 4:    Task 3 (validator adapter) + Task 4 (bench refresh)
Day 5:    Task 5 (migration guide update) + Task 6 (CHANGELOG & release)
```

## Day 1 — Remove Deprecated SSE Types

Goal: subtract code, no new behavior.

- Step 1.1: Read `response.go` lines covering `SSE`, `SSEEvent`, `SSEWriter`, `NewSSEWriter` (each tagged `// Deprecated:` since v1.3).
- Step 1.2: Confirm no internal callers via grep. Test files referencing the deprecated types will need their tests deleted (the deprecated path no longer exists).
- Step 1.3: Delete the four type definitions and their methods. Run `go build ./...`.
- Step 1.4: Update `docs/api/espresso.md` — remove the Deprecated SSE / SSEWriter sections that v2.0's PR #26 left in place with banners.
- Step 1.5: Open Task 1 PR. Net diff should be solidly negative.

## Day 2-3 — `Stream` Pre-Flight Phase (F-02)

Goal: let `Stream[T]` / `StreamSimple` handlers return `*espresso.Error` before headers commit, so a "resource not found" decision surfaces as a real HTTP 404 instead of an `event: error` frame on a 200 stream.

### Day 2

- Step 2.1: Re-read `sse.go`'s `serveStream` flow as restructured by v2.0 task-01.
- Step 2.2: Design — pick **one** of:
  - **(a) Optional pre-flight return value** on the handler signature (e.g., `func(ctx, req) (*espresso.Error, error)` where the first return short-circuits before headers).
  - **(b) `StreamHandler` interface** with separate `PreFlight(ctx, req) error` and `Run(ctx, stream)` methods.
  - **(c) Wrapper helper** like `StreamWithPreFlight(preflight, run)` that composes a pre-flight closure with the existing `Stream`.
  
  Document the pick in the PR. Recommended: (c) — additive, doesn't reshape existing handler signatures, fastest migration for callers.
- Step 2.3: Implement the chosen design. Add a regression test that asserts a pre-flight rejection produces a real 404 (not a 200-OK SSE stream with an error frame).

### Day 3

- Step 2.4: Migrate Barista's `RequireAppAccess` / `RequireDeploymentAccess` example pattern into the migration guide (Task 5) so users have a clear before/after.
- Step 2.5: Update `docs/streaming.md` and `docs/api/espresso.md` to document the pre-flight knob.
- Step 2.6: Run `go test ./... -race` clean.
- Step 2.7: Open Task 2 PR.

## Day 4 — Validator Adapter + Bench Refresh

### Task 3 (~½ day)

- Step 3.1: Add `func AsDefaultValidator() func(v any) error` to the `validator/` package. The body wraps `Struct` with the `FieldErrors → ValidationErrors` adapter the v2.0 example already showed.
- Step 3.2: Update `cmd/example/validate/main.go` to use the new helper:
  ```go
  espresso.SetDefaultValidator(validator.AsDefaultValidator())
  ```
  vs. the inline closure that's there today.
- Step 3.3: Add a unit test that confirms the adapter returns `*espresso.Error` (status 400, code `VALIDATION_ERROR`) on a tagged-struct rejection.
- Step 3.4: Update `docs/guide/validation.md` to show the helper in the Auto-Validate section. Keep the inline-closure example for users who want a custom error mapper.

### Task 4 (~½ day)

- Step 4.1: From `bench/`, run `go test -bench . -benchmem -benchtime=3s -count=3` against v2.x and capture results.
- Step 4.2: Update `README.md` "Framework Comparison" tables with the new numbers + a footnote reminder that the harness is unchanged from v1.4.
- Step 4.3: Update `bench/README.md` if anything material changed in the methodology (it shouldn't; this is purely a number refresh).

## Day 5 — Migration Guide Update + Release

### Task 5 (~½ day)

- Step 5.1: Create `docs/migration-v2-to-v2.1.md` covering ONLY the v2.1 deltas:
  - SSE legacy types removed (Task 1) — list the four symbols and their replacements; mechanical search-and-replace recipe.
  - Stream pre-flight added (Task 2) — additive, documented for completeness.
  - `validator.AsDefaultValidator()` added (Task 3) — additive ergonomics.
- Step 5.2: Add a "v2 series migrations" entry in the docs sidebar that links both `migration-v1-to-v2.md` and `migration-v2-to-v2.1.md`.
- Step 5.3: Cross-link from v2.0 README and the new v2.1 README.

### Task 6 (~½ day)

- Step 6.1: Promote `[Unreleased]` → `[2.1.0] - 2026-MM-DD` in CHANGELOG.
- Step 6.2: Bump `package.json` version to `2.1.0` and `docs/.vitepress/config.ts` chip to `v2.1.0`.
- Step 6.3: Final quality gates: `go test -race ./...`, `golangci-lint run ./...`, bench module spot-check.
- Step 6.4: Tag `v2.1.0`, push, `gh release create v2.1.0` with the `[2.1.0]` body + link to `migration-v2-to-v2.1.md`.

## Contingency Planning

### Must Not Slip (hard requirements for v2.1 release)

- Task 1 (remove deprecated SSE) — paid v2.0's debt; not removing now means carrying it indefinitely.
- Task 6 (release).

### Can Slip to v2.1.1 (nice to have, not blocking)

- Task 4 (bench refresh) — purely informational; the README's v1.4-era numbers are old but not wrong.

### Can Slip to v2.2 (additive features)

- Task 3 (validator adapter) — current `cmd/example/validate/main.go` works; adapter is ergonomics.

### Must Not Compromise (quality gates)

- `go test -race ./...` clean.
- `golangci-lint run ./...` clean.
- The migration guide chain (v1→v2 + v2→v2.1) is discoverable from at least the README, the VitePress sidebar, and the GitHub release notes.

## Parallel Work Opportunities

**Parallel batch 1** (no cross-file conflicts):
- Task 1 (response.go, docs/api/espresso.md) and Task 3 (validator/, docs/guide/validation.md) touch disjoint files.

**Parallel batch 2** (after Task 2 lands):
- Task 4 (bench/) and Task 5 (docs/migration-v2-to-v2.1.md) — both are downstream of the substantive work.

## Notes

- All v2.1 work happens on the `/v2` module path; no module-bump considerations.
- The deprecated-SSE removal is the kind of change that's tempting to bundle with refactors. Resist — Task 1 is purely subtractive. If `response.go` shows other cleanup opportunities, capture them in a follow-up issue.
- v2.1's value to users: paying off task-02 carry-over + closing F-02 + small validator nick. None of these alone justify a release; the bundle does. Ship the bundle.
