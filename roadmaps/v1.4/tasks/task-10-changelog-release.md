# Task 10: CHANGELOG & v1.4.0 Release

**Priority:** 📦 Meta
**Estimated Effort:** 0.5 day
**Dependencies:** All other tasks must be complete

## Context

Final gate before tagging v1.4.0. Consolidate all changes from Tasks 1-9 into a coherent CHANGELOG entry, run quality gates one last time, tag the release, publish notes.

## Acceptance Criteria

- [x] `CHANGELOG.md` has complete `[1.4.0] - 2026-04-20` section in Keep-a-Changelog format.
- [x] Five subsections: `Added`, `Changed`, `Fixed`, `Removed`, `Migration Notes`.
- [x] Migration Notes call out the one wire-format change (extractor errors → JSON) and the `Routes()` removal.
- [x] No breaking changes in `Added` / `Changed` beyond the documented Migration Notes — v1.4 is a minor bump.
- [x] `go test ./... -race` clean.
- [x] `golangci-lint run ./...` clean.
- [x] Bench module independently verified: `cd bench && go test -bench . -benchmem -benchtime=3s -count=1` runs cleanly.
- [x] Git tag `v1.4.0` created and pushed.
- [x] GitHub release created with release notes mirroring the CHANGELOG entry.
- [x] `package.json` `version` field bumped to `1.4.0` (the docs site tracks framework version).

## Technical Approach

### Step 10.1: Verify all tasks complete

Walk the roadmap. Every task file has its acceptance checkboxes ticked.

- [x] Task 1: Validator subpackage
- [x] Task 2: Bench-comparison module
- [x] Task 3: Streaming concurrency hardening
- [x] Task 4: Structured extractor errors
- [x] Task 5: Shutdown / state-injection tests
- [x] Task 6: Remove dead APIs
- [x] Task 7: Go directive + G115
- [x] Task 8: Handler-cache docs
- [x] Task 9: v2.0 roadmap

### Step 10.2: Update CHANGELOG

Promote `[Unreleased]` content into `[1.4.0] - 2026-04-20`. Final shape:

```markdown
## [Unreleased]

## [1.4.0] - 2026-04-20

### Added
- validator/ subpackage (Task 1)
- bench/ framework-comparison module (Task 2)
- TestWebSocket_GracefulShutdown / TestShutdown_WebSocketsClosed / TestSSE_Stream_StateInjection / TestWithLayers_ExtractorErrorReturnsStructuredJSON (Tasks 3, 4, 5)
- v2.0 roadmap at roadmaps/v2.0/ (Task 9)
- Handler-cache growth documentation (Task 8)

### Changed
- Extractor failures produce structured JSON (Task 4)
- Unified serveStream / serveStreamSimple (Task 3)
- WS.closed → atomic.Bool (Task 3)
- WS.Close idempotent + always removes from registry (Task 3)
- WS.readLoop channel sends guarded by ctx.Done() (Task 3)
- go.mod go directive lowered from 1.25.6 to 1.23 (Task 7)

### Fixed
- Data race on WS.closed (Task 3)
- Gosec G115 in test code (Task 7)

### Removed
- (*Router).Routes() and Route type — always returned nil (Task 6)
- closeErr field on *WS — was never read (Task 6)

### Migration Notes
- Extractor 4xx bodies are now JSON, not text/plain. Status codes unchanged.
- Routes() returned nil in v1.x; remove the call.
```

### Step 10.3: Final quality gates

```bash
go test ./... -race
golangci-lint run ./...
go test -tags=integration ./tests/integration/...
cd bench && go test -bench . -benchmem -benchtime=3s -count=1 && cd ..
```

All four must pass. The integration test is gated by build tag — run it explicitly here even though it isn't in default CI.

### Step 10.4: Bump and tag

- `package.json` version → `1.4.0`.
- Commit: `chore(release): bump to v1.4.0`.
- Tag: `git tag v1.4.0 && git push origin v1.4.0`.
- GitHub release: paste the `[1.4.0]` CHANGELOG section verbatim into the release body.

## Tests Required

The full test suite, plus the bench module spot-check.

## Definition of Done

- [x] `CHANGELOG.md` `[1.4.0]` section finalized with date `2026-04-20`.
- [x] `package.json` version bumped.
- [x] Git tag `v1.4.0` pushed.
- [x] GitHub release published.
- [x] Downstream Barista pinged about the release (so they can plan their pin bump).
