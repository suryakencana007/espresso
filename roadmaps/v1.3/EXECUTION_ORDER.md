# Execution Order for v1.3.0

This document provides the recommended execution order for the v1.3 roadmap tasks. The schedule assumes one developer (human or AI agent) working full-time for approximately 3 weeks.

## Overview

```
Week 1: WebSocket Support (Task 1) — the highest-effort task, done first
Week 2: SSE Streaming (Task 2) + Errors (Task 3) + Shutdown (Task 4)
Week 3: Verification tests (Tasks 5-7) + Documentation + Release (Tasks 8-10)
```

## Week 1 — WebSocket Support

Goal: Complete Task 1 end-to-end.

### Day 1 (Monday)
- Read all referenced existing code (handler.go, core.go, state.go, router.go)
- Task 1, Step 1.1: Add `github.com/coder/websocket` dependency
- Task 1, Step 1.2: Create `websocket.go` with `WS` type and `WSConfig` struct
- Write basic godoc comments on all public API

### Day 2 (Tuesday)
- Task 1, Step 1.2 (continued): Implement `Read`, `Write`, `WriteText`, `WriteBinary`, `WriteJSON`, `ReadJSON`, `Close`, `Context`, `Subprotocol` methods
- Write unit tests for each method as implemented (test-along approach)

### Day 3 (Wednesday)
- Task 1, Step 1.3: Implement `WebSocket[T]()` and `WebSocketSimple()` handler wrappers
- Task 1, Step 1.4: Implement upgrade flow (HTTP→WebSocket)
- Write integration test: simple echo server

### Day 4 (Thursday)
- Task 1, Step 1.5: Verify state injection works; add tests
- Task 1, Step 1.6: Implement `wsRegistry` for tracking open connections
- Implement ping/pong keepalive goroutine

### Day 5 (Friday)
- Integrate with graceful shutdown (close all WebSockets on shutdown)
- Add remaining tests from the Tests Required section
- Run race detector, fix any issues found

### Weekend
- Create `cmd/example/websocket/` with echo server example
- Write `cmd/example/websocket/README.md`
- Update main `README.md` with WebSocket section
- Add CHANGELOG entry
- Open PR

**Deliverable:** Task 1 complete, PR merged or ready for review.

---

## Week 2 — SSE, Errors, Shutdown

Goal: Complete Tasks 2, 3, and 4.

### Day 1 (Monday)
- Task 2, Step 2.1: Create `sse.go` with `SSEStream` and `Event` types
- Task 2, Step 2.2: Implement `Stream[T]()` and `StreamSimple()` handler wrappers
- Task 2, Step 2.3: Implement SSE headers setup, extractor parsing

### Day 2 (Tuesday)
- Task 2, Step 2.4: Implement thread-safe `Send()` with mutex
- Task 2, Step 2.5: Implement SSE format (event, data, id, retry fields)
- Implement keepalive goroutine
- Write unit tests

### Day 3 (Wednesday)
- Task 2: Complete remaining tests (client disconnect, JSON events, Last-Event-ID)
- Create `cmd/example/sse/` with counter example
- Update README SSE section
- CHANGELOG entry for Task 2

### Day 4 (Thursday)
- Task 3, Step 3.1: Refactor `error.go` with `*Error` type and fluent builder
- Task 3, Step 3.2: Implement standard JSON response format
- Task 3, Step 3.3: Modify handler wrappers to handle `*Error` return

### Day 5 (Friday)
- Task 3, Step 3.4: Panic recovery in middleware wraps to `*Error`
- Task 3: Write all tests
- Update handler examples in README to show error handling
- CHANGELOG entry for Task 3

### Weekend
- Task 4, Step 4.1: Add `OnShutdown()` to router
- Task 4, Step 4.2: Update `Brew()` shutdown flow
- Task 4, Step 4.3: Wire up registry for SSE + WS close on shutdown
- Write tests for Task 4
- CHANGELOG entry

**Deliverable:** Tasks 2, 3, 4 complete. PRs merged or ready for review.

---

## Week 3 — Verification, Documentation, Release

Goal: Complete verification tasks, finalize documentation, cut release.

### Day 1 (Monday)
- Task 5: Create `tests/integration/` directory
- Task 5: Write `TestLongLived_SSE_1Hour` and `TestLongLived_WS_1Hour` tests
- Configure CI to run integration tests nightly (optional for v1.3, can defer)

### Day 2 (Tuesday)
- Task 5: Write `TestLongLived_SSE_100Concurrent` test
- Task 5: Add memory profiling script `scripts/profile-memory.sh`
- Task 5: Run all long-lived tests; document results in `docs/performance.md`

### Day 3 (Wednesday)
- Task 6: Write context cancellation tests for all handler types
- Task 7: Write streaming upload memory test
- Fix any bugs discovered during verification

### Day 4 (Thursday)
- Task 9: Create `docs/websocket.md` deep-dive document
- Task 9: Create `docs/streaming.md` deep-dive document
- Task 9: Update all README sections

### Day 5 (Friday)
- Task 8: Update CHANGELOG with all v1.3 changes
- Task 8: Version bump in relevant files
- Task 8: Final review of all v1.3 changes
- Task 10: Draft blog post

### Weekend
- Final polish: address any issues found during review
- Tag v1.3.0 release
- Publish release notes on GitHub
- Publish blog post on dev.to or personal site
- Announce on relevant channels

**Deliverable:** v1.3.0 released. Ready for Barista development.

---

## Contingency Planning

If a task runs long, here's the priority order for what can slip:

### Must Not Slip (hard requirements for v1.3 release)
- Task 1 (WebSocket)
- Task 2 (SSE)
- Task 3 (Errors)
- Task 8 (CHANGELOG & Release)

### Can Slip to v1.3.1 (nice to have, not blocking)
- Task 4 (Shutdown hooks) — acceptable if basic shutdown works, advanced hooks can wait
- Task 10 (Blog post) — can be published after the release

### Can Slip to v1.3.x patch releases (easy to add later)
- Task 5 (Long-lived tests) — can be added incrementally
- Task 6 (Context cancellation tests) — can be added incrementally
- Task 7 (Upload memory test) — can be added incrementally
- Task 9 (Deep-dive docs) — the README update is enough for initial release

### Must Not Compromise (quality gates)
- Test coverage minimums
- `go test -race` passing
- Backward compatibility
- godoc comments on public APIs

## Parallel Work Opportunities

If multiple agents or developers work on this in parallel, the following tasks have no dependencies and can be done concurrently:

**Parallel batch 1 (can start immediately, before Task 1 is done):**
- Task 3 (Structured Errors) — only touches `error.go` and handler wrappers

**Parallel batch 2 (after Task 1 and Task 2 are done):**
- Task 4 (Shutdown hooks) — depends on Task 1 and 2 for registry integration
- Task 9 (Documentation) — needs stable APIs from Tasks 1-4

**Parallel batch 3 (can run while others continue):**
- Task 5, 6, 7 (Verification tests) — only need the features to exist
- Task 10 (Blog post)

## Notes

- Time estimates assume focused work. Add 30-50% buffer for interruptions, reviews, and unforeseen issues.
- The "Weekend" entries in the schedule are flexible — they can be weekday overflow work.
- Code review time is not accounted for in this schedule. If working with other reviewers, add 1-2 days for review cycles per task.
- Integration tests (Task 5-7) can be done in batches rather than individually to reduce context switching.
