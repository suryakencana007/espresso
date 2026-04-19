# Task 8: CHANGELOG & Version Bump

**Priority:** 📦 Meta
**Estimated Effort:** 0.5 day
**Dependencies:** All other tasks must be complete

## Context

Before tagging v1.3.0 as a release, we need to:

- Finalize the CHANGELOG with all new features
- Bump version numbers in any relevant files
- Verify backward compatibility
- Create a Git tag
- Publish release notes on GitHub

This task is the final gate before announcing v1.3.0.

## Acceptance Criteria

- [ ] `CHANGELOG.md` has complete `[1.3.0]` section with all changes
- [ ] Version number bumped in any file that tracks it (go module is auto-detected from tag)
- [ ] Breaking changes documented (should be zero for v1.3)
- [ ] Migration notes written for any behavioral changes
- [ ] Git tag `v1.3.0` created and pushed
- [ ] GitHub release created with release notes
- [ ] Release notes published on the blog (part of Task 10)

## Technical Approach

### Step 8.1: Verify All Tasks Complete

Before starting, confirm all tasks in the roadmap have been merged:

- [ ] Task 1: WebSocket Handler Support
- [ ] Task 2: Typed Streaming Response (SSE)
- [ ] Task 3: Structured Error Response
- [ ] Task 4: Graceful Shutdown Hooks
- [ ] Task 5: Long-Lived Connection Stress Test
- [ ] Task 6: Context Cancellation Propagation Test
- [ ] Task 7: Streaming Upload Memory Test
- [ ] Task 9: Documentation Update

### Step 8.2: Update CHANGELOG

Update `CHANGELOG.md` with a proper v1.3.0 section. Use Keep a Changelog format:

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.3.0] - 2026-MM-DD

### Added

- **WebSocket handler support** with the new `espresso.WS` type and
  `espresso.WebSocket[T]()` wrapper. Supports text and binary frames,
  ping/pong keepalive, state injection, and graceful shutdown integration.
  Uses `github.com/coder/websocket` as the underlying library.
- **Typed SSE streaming** with the new `espresso.SSEStream` type and
  `espresso.Stream[T]()` wrapper. Replaces the low-level `SSEWriter` with
  a first-class typed handler pattern. Supports Last-Event-ID for
  reconnection, configurable keepalive, and concurrent-safe sends.
- **Structured error responses** via `*espresso.Error` with fluent builders
  (`WithCode`, `WithDetail`, `Wrap`). Handlers can return `*Error` directly;
  the framework serializes to a consistent JSON format including request ID.
  Common constructors: `ErrNotFound`, `ErrBadRequest`, `ErrUnauthorized`, etc.
- **Graceful shutdown hooks** via `router.OnShutdown(fn)`. Multiple hooks run
  in registration order. SSE streams and WebSockets are auto-closed during
  shutdown with appropriate final messages.
- New examples: `cmd/example/websocket/` and `cmd/example/sse/`
- Documentation: `docs/websocket.md`, `docs/streaming.md`, `docs/performance.md`

### Changed

- `error.go` refactored to use the new structured `Error` type.
  Plain `error` returns from handlers are still supported (backward compat).
- Panic recovery now produces structured error responses with `PANIC` code.
- README expanded with sections on WebSocket, Streaming, Error Handling,
  and Graceful Shutdown.

### Deprecated

- `SSEWriter` low-level API is deprecated in favor of `SSEStream`.
  Will be removed in v2.0. No removal in v1.x.

### Fixed

(None specific to v1.3 — existing bugs fixed in patch releases.)

### Verification

- Long-lived SSE streams tested for 1-hour duration with stable memory
- WebSocket connections tested for 1-hour idle duration
- 100 concurrent streams tested for 10-minute duration
- Context cancellation verified to propagate within 1 second
- 1 GB multipart uploads verified to use bounded memory

See [docs/performance.md](./docs/performance.md) for details.

### Migration Notes

v1.3 is backward compatible with v1.2. No code changes required to upgrade.

To take advantage of new features:

- Replace manual SSE handling with `espresso.Stream[T]()` for better integration
- Use `*espresso.Error` in handlers for consistent error responses
- Register cleanup hooks with `router.OnShutdown(fn)`

[Unreleased]: https://github.com/suryakencana007/espresso/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/suryakencana007/espresso/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/suryakencana007/espresso/compare/v1.1.0...v1.2.0
```

### Step 8.3: Verify Backward Compatibility

Run the v1.2.0 examples (if any exist in a separate repo or test project) against v1.3.0 to verify they still work. Specifically check:

- Existing `JSON[T]`, `Doppio`, `Solo`, `Ristretto` handlers compile and run
- Existing middleware still functions
- Existing error handling (returning plain `error`) still works
- State injection pattern unchanged

Document any edge cases in the CHANGELOG migration notes.

### Step 8.4: Version-Referenced Files

Go modules infer version from Git tags, so no `version.go` file needs updating. However, check these locations:

- `README.md` — if it references a version, update it
- `package.json` — if it exists and has a version (unusual for a Go project, but check)
- `docs/*.md` — if any reference a "current version"

Find references with:

```bash
grep -r "1\.2\.0" --include="*.md" --include="*.go" --include="*.json"
```

### Step 8.5: Pre-Release Checklist

Before tagging, verify:

- [ ] All CI checks pass on `main`
- [ ] `go test ./... -race` passes
- [ ] `golangci-lint run ./...` passes
- [ ] `go build ./...` succeeds
- [ ] Examples all compile and run
- [ ] Documentation rendered correctly (check GitHub rendering)
- [ ] No TODO or FIXME comments left in the code referencing v1.3

### Step 8.6: Create Git Tag

```bash
git checkout main
git pull
git tag -a v1.3.0 -m "Release v1.3.0

Highlights:
- WebSocket handler support
- Typed SSE streaming
- Structured error responses
- Graceful shutdown hooks

See CHANGELOG.md for full details."
git push origin v1.3.0
```

### Step 8.7: GitHub Release

Go to https://github.com/suryakencana007/espresso/releases/new

- Tag: `v1.3.0`
- Title: `v1.3.0 - WebSocket, Streaming, and the Road to Barista`
- Description: Copy the `[1.3.0]` section from CHANGELOG with light formatting
- Add highlight banner:

```markdown
## 🎉 v1.3.0 Highlights

This release adds **WebSocket handler support**, **typed SSE streaming**,
**structured error responses**, and **graceful shutdown hooks** — features
developed to power [Barista](https://github.com/YOUR_USERNAME/barista),
a self-hosted PaaS built on Espresso.

## 📦 Installation

    go get github.com/suryakencana007/espresso@v1.3.0

## What's New
...
```

- Check "Set as the latest release"
- Publish

### Step 8.8: Update Documentation Site

If there's a docs site (e.g., GitHub Pages, pkg.go.dev):

- Wait a few minutes for pkg.go.dev to index v1.3.0
- Verify https://pkg.go.dev/github.com/suryakencana007/espresso@v1.3.0 shows new docs
- Update any external links if applicable

## Tests Required

None — this is a release task, not a feature task.

Verify that all existing tests pass one final time:

```bash
go test ./... -race -count=1
go test -tags=integration -timeout=2h ./tests/integration/...
```

## Definition of Done

- [ ] CHANGELOG.md `[1.3.0]` section complete and accurate
- [ ] Version references updated where applicable
- [ ] All CI checks pass on `main`
- [ ] Git tag `v1.3.0` created and pushed
- [ ] GitHub release published with release notes
- [ ] pkg.go.dev shows v1.3.0 docs (may take up to 30 min after push)
- [ ] No critical issues reported in the first 24 hours post-release

## Rollback Plan

If a critical issue is discovered within 24 hours:

1. **Minor issue (typo, doc error):** Ship v1.3.1 patch with fix
2. **Major issue (broken API, data loss risk):**
   - Do NOT delete the v1.3.0 tag (it may already be cached by proxies)
   - Ship v1.3.1 with the fix immediately
   - Add a warning to the v1.3.0 GitHub release pointing to v1.3.1
   - Document the issue in the v1.3.1 changelog

## Post-Release Tasks (handled in other tasks)

- Task 10: Blog post announcing v1.3.0
- Barista development can begin in earnest
- Monitor GitHub issues for bug reports
- Collect feedback for v1.4 roadmap
