# Task 2: Remove Deprecated APIs

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 1-2 days
**Dependencies:** None (can run in parallel with Task 1)

## Context

v1.x accumulated deprecation aliases that cannot be removed under the "no breaking changes" promise. v2.0 is the window to drop them. Each removal below is backed by a `// Deprecated:` marker in v1.3 code or by in-repo grep confirming zero callers.

## Inventory

### From `error.go`

- `ErrorResponse` (type alias for `Error`, line ~106) — marked "for backward compatibility". Callers: zero in-repo.
- `NewBadRequest` (line ~302) — docstring says "prefer `ErrBadRequest` for new code".
- `NewInternal` (line ~362) — same.

### From `response.go`

- `SSE` response type (line ~156) — marked "Deprecated: Use the new Stream[T]() and StreamSimple() handlers with SSEStream instead".
- `SSEWriter` helper (line ~233) — marked "Deprecated: Use the new SSEStream type with Stream[T]() or StreamSimple() instead".

### From `websocket.go`

- `closeErr` field — unused (set, never read). Already removed in session work; verify.

Grep the repo fresh before removal to confirm the inventory; if any symbol has internal callers, migrate them first.

## Acceptance Criteria

- [ ] All symbols listed above are deleted from the tree.
- [ ] Any internal caller is migrated to the replacement (not shimmed).
- [ ] `go build ./...` succeeds.
- [ ] `go test ./... -race` passes.
- [ ] `golangci-lint run ./...` is clean.
- [ ] Migration-guide section drafted for each removed symbol.

## Technical Approach

### Step 2.1 — Confirm Inventory

```bash
# Should return only the declarations themselves
grep -rn "ErrorResponse\b" --include='*.go'
grep -rn "NewBadRequest\b" --include='*.go'
grep -rn "NewInternal\b" --include='*.go'
grep -rn "SSEWriter\b" --include='*.go'
```

Any hit outside the declaring file is an internal caller that must be migrated first.

### Step 2.2 — Migrate Internal Callers

For each internal caller of a removed symbol, rewrite to use the v2 replacement:

| v1 symbol | v2 replacement |
|-----------|----------------|
| `ErrorResponse` | `Error` |
| `NewBadRequest("msg")` | `ErrBadRequest("msg")` |
| `NewInternal("msg")` | `ErrInternal("msg")` |
| `SSE{...}` response | `StreamSimple(handler)` + `SSEStream.Send(Event{...})` |
| `SSEWriter` | `SSEStream` |

If a test file is the only caller, update the test.

### Step 2.3 — Delete Declarations

Remove the type/function declarations. Compile. Fix any downstream.

### Step 2.4 — Migration Guide Entries

For each removed symbol, write a migration recipe — see Task 6 for format.

## Tests Required

- Existing tests should already exercise the replacement paths. No new tests needed; the value here is removal, not addition.
- Spot-check that response tests still cover JSON error shape and SSE streaming end-to-end (these were validated in v1.3 session work).

## Breaking Changes

All symbols listed in the inventory section. Each gets a migration-guide entry in Task 6.

## Definition of Done

- Inventory grep returns only declaration-site hits (i.e. none, after deletion)
- `go test ./... -race` passes
- `golangci-lint run ./...` clean
- Migration entries written
- CHANGELOG `[Unreleased]` entry under `Removed`
