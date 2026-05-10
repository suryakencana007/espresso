# Task 1: Remove Deprecated SSE Types

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 1 day
**Dependencies:** None

## Context

`response.go` carries four types that have been tagged `// Deprecated:` since v1.3 with forward pointers to `Stream[T]` / `*SSEStream`:

- `SSE` (response type)
- `SSEEvent` (event struct)
- `SSEWriter` (helper)
- `NewSSEWriter` (constructor)

v2.0 task-02 was scoped to remove them but deferred (see [v2.0 task-02 retrospective](../../v2.0/tasks/task-02-remove-deprecated-apis.md)) because the removal had no functional payoff in v2.0 itself. That deferral expires here. Existing callers have had two minor releases (v1.4, v1.5) plus all of v2.0 to migrate; staticcheck SA1019 has been flagging them throughout.

## Acceptance Criteria

- [ ] `response.go` no longer defines `SSE`, `SSEEvent`, `SSEWriter`, or `NewSSEWriter`.
- [ ] All methods on those types (e.g., `SSE.WriteResponse`, `SSEWriter.Event`, `SSEWriter.EventJSON`, `SSEWriter.KeepAlive`, etc.) are removed along with the types.
- [ ] No internal callers remain — `go build ./...` clean.
- [ ] `response_test.go` tests covering the deprecated types are deleted.
- [ ] `docs/api/espresso.md` no longer has the Deprecated SSE / SSEWriter sections (currently shown with warning banners post-#26).
- [ ] CHANGELOG `[Unreleased]` → `Removed (BREAKING)` entry.

## Technical Approach

### Step 1.1 — Confirm No Live Callers

```bash
grep -rn '\bSSE\b\|SSEEvent\|SSEWriter\|NewSSEWriter' --include='*.go' .
# Expected: only references inside response.go and response_test.go.
```

The new SSE primitive — `*SSEStream`, `Event`, `Stream[T]`, `StreamSimple` — lives in `sse.go` and is unaffected.

### Step 1.2 — Delete

Remove from `response.go`:

- `SSEEvent` struct + its godoc.
- `SSE` struct + `WriteResponse`, `WriteEvent`, `WriteKeepAlive` methods.
- `SSEWriter` struct + `NewSSEWriter` constructor + all methods (`Event`, `EventWithID`, `Data`, `EventJSON`, `KeepAlive`, `Retry`).

Net diff: ~−180 lines in `response.go` and ~−250 in `response_test.go` (estimate).

### Step 1.3 — Test File Cleanup

`response_test.go` has tests like `TestSSE_WriteEvent`, `TestSSE_WriteKeepAlive`, `TestSSEWriter_*` (eight tests). Delete them. The remaining tests (JSON, Text, Status) are unaffected.

### Step 1.4 — Docs

`docs/api/espresso.md` has two sections that documented the deprecated types with `::: warning Deprecated since v1.3` banners (added in PR #26). Remove both sections entirely. The streaming guide (`docs/streaming.md`) and the API reference for `*SSEStream` already cover the modern path.

## Tests Required

- Existing `Stream[T]` / `StreamSimple` tests in `sse_test.go` continue to pass (they don't touch the removed types).
- A grep regression check (manual) confirms no doc page still mentions the removed symbols outside of migration guides.

## Breaking Changes

User-facing: any v1.x or v2.0 caller of `espresso.SSE`, `espresso.SSEEvent`, `espresso.SSEWriter`, or `espresso.NewSSEWriter` fails to compile after this lands. Migration recipes belong in `docs/migration-v2-to-v2.1.md` (Task 5):

```go
// Before
sse := espresso.NewSSEWriter(w)
sse.Event("update", "hello")

// After (use Stream/StreamSimple from sse.go)
router.Get("/stream", espresso.StreamSimple(func(ctx context.Context, s *espresso.SSEStream) error {
    return s.SendText("update", "hello")
}))
```

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -race ./...` clean.
- [ ] `golangci-lint run ./...` clean — no remaining `SA1019` warnings on the removed symbols (because they're gone).
- [ ] CHANGELOG `[Unreleased]` Removed (BREAKING) entry written.
- [ ] PR description references the v2.0 task-02 retrospective deferral note.
