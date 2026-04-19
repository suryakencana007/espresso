# Guidelines for AI Agents

This document applies to any AI agent (Claude Code, Cursor agent, GitHub Copilot Workspace, etc.) working on Espresso v1.3.0 tasks. Read this file before starting any task.

## Core Rules

### 1. Read Before Writing

Before starting any task, read the existing code to understand established patterns:

- **`handler.go`** — How `Ristretto`, `Solo`, `Doppio` handler wrappers work
- **`core.go`** — Core router and handler interfaces
- **`state.go`** — How state injection works via context
- **`router.go`** — How routes are registered and middleware is applied
- **`extractor/*.go`** — How extractors like `JSON[T]`, `Path[T]` are implemented

New features must follow these patterns, not invent new ones. Consistency is more important than cleverness.

### 2. Follow the Coffee Metaphor

Espresso uses a consistent coffee-themed naming convention. New features should fit this metaphor:

- **`Portafilter()`** — Creates the router
- **`Ristretto()`** — 0-parameter handler
- **`Solo()`** — 1-parameter handler
- **`Doppio()`** — 2-parameter handler
- **`Brew()`** — Starts the server
- **`Use()`** — Adds middleware

When naming new features, ask: does this name fit the coffee theme? If you're unsure, note the naming decision in the PR description for maintainer review.

### 3. Type-Safety Over Flexibility

Espresso uses Go generics extensively. Prefer typed APIs over `interface{}`:

```go
// GOOD — type-safe
func Stream[T any](h StreamHandler[T], opts ...StreamOption) Handler

// AVOID — untyped
func Stream(h func(interface{}) error) Handler
```

If generics make an API awkward, reconsider the design. Occasionally untyped APIs are acceptable, but they should be justified.

### 4. Context Is Mandatory

All functions that perform I/O, wait for events, or may take more than a few microseconds must accept `context.Context`:

```go
// GOOD
func (w *WS) Read(ctx context.Context) (MessageType, []byte, error)

// AVOID
func (w *WS) Read() (MessageType, []byte, error)
```

Context enables cancellation, timeouts, and propagating request-scoped data. This is non-negotiable for v1.3.

### 5. Testing Requirements

Every task has a `Tests Required` section. Do not skip it.

- Use `go test -race` for all tests (streaming and WebSocket have concurrent access)
- Aim for minimum 80% coverage, target 85%+
- Include both happy path and error path tests
- Include tests for concurrent access where applicable
- If you're unsure how to test something, look at `handler_test.go` or `router_test.go` for reference patterns

### 6. Commit Message Convention

Use Conventional Commits format with clear scope:

```
feat(ws): add WS wrapper type with config
feat(ws): implement upgrade flow
feat(ws): add state injection support
test(ws): add echo server tests
docs(ws): add example and README section
fix(ws): correct close code for handler panic
refactor(sse): extract header-setting to helper
```

Scopes used in this roadmap:
- `ws` — WebSocket related changes
- `sse` — Server-Sent Events / streaming
- `error` — Error handling
- `shutdown` — Graceful shutdown
- `docs` — Documentation only
- `test` — Test-only changes
- `ci` — CI/CD configuration

### 7. Small, Focused Commits

One logical change per commit. If you find yourself writing "and" in a commit message, split it into two commits.

Good:
- "feat(ws): add WS wrapper type"
- "feat(ws): implement Read method"
- "feat(ws): implement Write method"

Bad:
- "feat(ws): add wrapper type and implement Read and Write methods"

### 8. Backward Compatibility

v1.3 is a **minor version bump** following semver. This means:

- **No breaking changes** to public APIs
- Existing code written for v1.2 must continue to compile and behave identically
- If you must deprecate something, add a deprecation comment but keep it working
- New parameters to existing functions: use variadic options pattern, not positional arguments

If you believe a breaking change is necessary, stop and discuss with the maintainer before proceeding.

### 9. Performance Matters

For hot paths (handler execution, WebSocket read/write, SSE send):

- Avoid allocations when possible
- Use `sync.Pool` for reusable objects (follow the existing pattern in handler root)
- Add benchmarks before claiming a task complete
- If introducing a new allocation, document why in a code comment

Run benchmarks before and after changes:

```bash
go test -bench=. -benchmem -count=3 > before.txt
# ... make changes ...
go test -bench=. -benchmem -count=3 > after.txt
benchstat before.txt after.txt
```

### 10. Error Handling Philosophy

- Return errors, don't panic (except in truly unrecoverable situations)
- Wrap errors with context using `fmt.Errorf("context: %w", err)` so callers can unwrap
- Panics in user-provided handlers must be recovered by middleware (not crash the server)
- Use the structured `*espresso.Error` type for HTTP responses (after Task 3 lands)

## Workflow

### Starting a Task

1. Read the task file in `tasks/` directory
2. Read the referenced existing code files
3. Check the Definition of Done section — this is your exit criteria
4. Create a feature branch: `git checkout -b feat/ws-wrapper-type` (match the scope of commits you'll make)
5. Make small, focused commits as you progress

### During a Task

- If you encounter ambiguity, document it in the PR description under a "Decisions Made" section
- If the task requires a decision not covered by the spec, make a reasonable choice and document it
- If you discover the task needs to be broken down further, add sub-issues to the task file

### Completing a Task

Before marking a task complete:

1. All Acceptance Criteria checkboxes are ticked
2. All Definition of Done items are completed
3. `go test ./... -race` passes
4. `golangci-lint run ./...` passes
5. Example code runs as documented
6. CHANGELOG.md has entry under `[Unreleased]`
7. Godoc comments exist on all new public APIs

### Pull Request Template

Use this structure for PR descriptions:

```markdown
## Task
Link to the task file, e.g., `docs/v1.3-roadmap/tasks/task-01-websocket.md`

## Summary
Brief description of what this PR adds/changes.

## Acceptance Criteria Status
- [x] Handler signature implemented
- [x] State injection works
- [ ] Graceful shutdown integration (deferred to follow-up PR)

## Decisions Made
Any non-obvious decisions, with rationale.

## Testing
How to manually verify this works beyond automated tests.

## Breaking Changes
None (v1.3 is backward compatible).
```

## What To Do When Stuck

If you cannot proceed:

1. **Check if the ambiguity is in the task spec** — if yes, propose a resolution in the PR description and proceed with your interpretation
2. **Check existing code for analogous patterns** — if a similar problem has been solved before, follow that solution
3. **Look at reference implementations** — for WebSocket, look at `coder/websocket` examples; for SSE, look at the SSE specification
4. **Escalate clearly** — if truly stuck, stop and open a discussion issue with:
   - What you're trying to do
   - What you tried
   - Where you got stuck
   - What you need clarified

Do not invent behavior that contradicts the task spec. Do not expand scope beyond the task. Do not make breaking changes without authorization.

## Files You Should NOT Touch

Unless the task explicitly says to modify these:

- `go.mod`, `go.sum` — only add dependencies listed in the task
- Existing test files — add new tests in new files
- `README.md` — only the sections explicitly called out in tasks
- Any file in `extractor/`, `middleware/`, `openapi/`, `pool/` unless the task says so

## Common Mistakes to Avoid

1. **Renaming existing types or functions** — this breaks backward compatibility
2. **Adding interfaces "for flexibility"** — Espresso prefers concrete types; add interfaces only when you have a concrete reason
3. **Over-configuring** — default values should work for 80% of users; only add options when there's a clear use case
4. **Skipping the example** — `cmd/example/` is part of Espresso's documentation; skipping it is skipping user-facing docs
5. **Ignoring `go test -race`** — race conditions in WebSocket or SSE code will bite users in production; always test with race detector
6. **Large, unfocused PRs** — prefer multiple small PRs over one large PR; easier to review and safer to revert if needed
