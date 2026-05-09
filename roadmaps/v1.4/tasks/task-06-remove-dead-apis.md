# Task 6: Remove Dead APIs (`Routes`, `closeErr`)

**Priority:** ⚪ Cleanup
**Estimated Effort:** 0.5 day
**Dependencies:** None

## Context

Two members of the public/internal surface had no live callers and no working behavior:

1. **`(*Router).Routes() []Route` (and the `Route` type itself)** — always returned `nil`. The doc comment said "ServeMux doesn't expose routes" — i.e., it was scaffolding for a feature that was never wired up. No caller inside the repo. Any external caller was getting `nil` and nothing else.
2. **`closeErr` field on `*WS`** — set in `readLoop` but never read. Unexported, zero callers, dead memory.

v1.4 removes both. Normally we wait for v2.0 to remove anything, but these qualify under the "zero callers + replacement is documented (or no replacement is needed)" carve-out in `roadmaps/v2.0/AGENT_GUIDELINES.md`'s carry-over rules.

## Acceptance Criteria

- [x] `(*Router).Routes()` removed.
- [x] The `Route` type removed.
- [x] `closeErr` field on `*WS` removed.
- [x] `readLoop`'s assignment to `closeErr` removed (it's now dead code).
- [x] `go build ./...` compiles.
- [x] `go test ./... -race` clean.
- [x] CHANGELOG `[Unreleased]` → `Removed` entries with migration notes.

## Technical Approach

### Step 6.1: Routes / Route removal

Delete from `router.go` (or wherever `Routes()` lives). Grep for any caller — there should be zero.

Migration note for the CHANGELOG:

> `(*Router).Routes() []Route` and the `Route` type — the method always returned `nil` (documented as "ServeMux doesn't expose routes"). No caller inside the repo; any external caller was getting no data. Migration: delete the call. If you need route introspection, track it yourself at registration time.

### Step 6.2: closeErr removal

Delete the field, the assignment in `readLoop`, and any reference. This is unexported, so no migration note needed in the user-facing CHANGELOG; mention it under `Removed` for completeness.

### Step 6.3: Verify

- `go build ./...` — must compile.
- `go vet ./...` — must pass.
- `go test ./... -race` — must pass.
- `golangci-lint run ./...` — must be clean (the `unused` linter will flag any leftover dead code).

## Tests Required

No new tests. Existing tests must continue to pass.

## Definition of Done

- [x] Both removals shipped in a single commit (`refactor: remove dead Routes() and closeErr`).
- [x] CHANGELOG `Removed` section documents both with migration notes.
- [x] `golangci-lint run ./...` clean — `unused` linter has nothing to say about either.
