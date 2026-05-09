# Execution Order for v1.4.0

This document records the actual execution order used for the v1.4 roadmap tasks. v1.4 was scoped tighter than v1.3 (hardening + two bounded features) and ran in approximately 2 weeks of focused work.

## Overview

```
Week 1: Validator subpackage (Task 1) + structured extractor errors (Task 4)
Week 2: Streaming concurrency hardening (Task 3) + bench module (Task 2)
        + verification tests (Task 5) + cleanup (Tasks 6-8)
        + v2.0 roadmap (Task 9) + release (Task 10)
```

## Week 1 — Validator + Error Pipeline Alignment

### Day 1 (Monday)
- Read `validator/`-shaped designs in adjacent ecosystems (go-playground/validator, ozzo-validation) for tag conventions.
- Read existing `error.go` `*Error` and `FieldErrors` types — the validator must produce a value that flows into the existing pipeline, not a parallel one.
- Task 1, Step 1.1: Create `validator/validator.go` with `Struct(any) error` entry point and the rule registry.

### Day 2 (Tuesday)
- Task 1, Step 1.2: Implement `required`, `min`, `max`, `email`, `url`, `regex`, `oneof`.
- Task 1, Step 1.3: Implement nested-struct, pointer-to-struct, and slice-of-struct recursion with path tracking.
- Write unit tests as each rule is added.

### Day 3 (Wednesday)
- Task 1, Step 1.4: Wire `validator.Struct(...)` return into `espresso.FieldErrors` so the existing structured-error response handles it without new plumbing.
- Task 1: Author `docs/guide/validation.md` and `docs/api/validator.md`.
- Open Task 1 PR.

### Day 4 (Thursday)
- Task 4, Step 4.1: Audit every `http.Error()` call in `withlayers.go`, `sse.go`, `websocket.go`. Each represents a wire-format mismatch with the handler-error path.
- Task 4, Step 4.2: Replace each call with `writeExtractError` / `writeHandlerError` style helpers that emit the same `{"error":{...}}` shape.

### Day 5 (Friday)
- Task 4, Step 4.3: Add `TestWithLayers_ExtractorErrorReturnsStructuredJSON` to lock the JSON shape — the regression we're guarding against is reverting back to text/plain.
- Open Task 4 PR.

**Deliverable end of Week 1:** validator subpackage + extractor error path aligned.

---

## Week 2 — Hardening, Bench, Cleanup, Release

### Day 1 (Monday)
- Task 3, Step 3.1: Reproduce the `WS.closed` race under `go test -race` first — confirm before fixing.
- Task 3, Step 3.2: Migrate `WS.closed` to `atomic.Bool`; make `Close` idempotent via CAS.
- Task 3, Step 3.3: Guard `WS.readLoop` channel sends with `ctx.Done()` to prevent post-handler-return blocking.
- Task 3, Step 3.4: Extract the duplicated `serveStream` / `serveStreamSimple` transport into a single helper.

### Day 2 (Tuesday)
- Task 3, Step 3.5: Fix `WS.Close` registry leak — registry removal must happen even on client-initiated disconnects.
- Task 3 tests: re-run long-lived integration tests (`tests/integration/longlived_test.go`) under race detector for at least 30 seconds.
- Open Task 3 PR.

### Day 3 (Wednesday)
- Task 2, Step 2.1: Create `bench/` as a separate Go module with a `replace` directive back to the parent.
- Task 2, Step 2.2: Implement three scenarios across Espresso, Gin, Echo, Fiber: static text, JSON round-trip, path parameter.
- Task 2, Step 2.3: Write `bench/README.md` documenting the methodology, especially Fiber's `app.Test()` harness asymmetry.

### Day 4 (Thursday)
- Task 2, Step 2.4: Run benchmarks; populate the README "Framework Comparison" tables.
- Task 5: Add `TestWebSocket_GracefulShutdown`, `TestShutdown_WebSocketsClosed`, `TestSSE_Stream_StateInjection`.
- Tasks 6 + 7 + 8 in parallel:
  - Remove `Routes()` / `Route` (always returned `nil`); remove `closeErr` (never read).
  - Lower `go.mod` directive from 1.25.6 to 1.23, verify under 1.23.
  - Document `handlerCache` growth semantics in `handler.go` and `docs/performance.md`.

### Day 5 (Friday)
- Task 9: Scaffold `roadmaps/v2.0/` mirroring v1.3's layout. Write README, AGENT_GUIDELINES (delta-only), EXECUTION_ORDER, and the seven task files.
- Task 10, Step 10.1: Update `CHANGELOG.md` with the `[1.4.0] - 2026-04-20` section in Keep-a-Changelog format. Cover Added / Changed / Fixed / Removed / Migration Notes.
- Task 10, Step 10.2: Final `go test ./... -race`, `golangci-lint run ./...`, race-test the framework benchmarks.
- Task 10, Step 10.3: Tag `v1.4.0`, draft GitHub release notes.

**Deliverable end of Week 2:** v1.4.0 released; v2.0 roadmap published alongside.

---

## Contingency Planning

### Must Not Slip (hard requirements for v1.4 release)
- Task 3 (concurrency hardening) — the race fixes are the most user-visible reason to ship v1.4 at all.
- Task 4 (structured extractor errors) — wire-format consistency.
- Task 10 (CHANGELOG & release).

### Could Slip to v1.4.1 (nice to have, not blocking)
- Task 2 (bench module) — purely additive; could ship as a follow-up minor.
- Task 8 (handler-cache growth docs) — documentation-only; could ship as a docs PR.

### Could Slip to v1.5 (additive features)
- Task 1 (validator subpackage) — TODOS #7 has been open for releases; one more would not be fatal. Bundled into v1.4 because the integration into `FieldErrors` is small.

### Must Not Compromise (quality gates)
- `go test ./... -race` clean.
- `golangci-lint run ./...` clean.
- No new `http.Error()` calls in extractor / sse / websocket paths.
- v1.3 callers must compile against v1.4 unchanged.

## Parallel Work Opportunities

**Parallel batch 1** (no cross-file conflicts):
- Task 1 (validator) and Task 4 (structured errors) — touch disjoint files.

**Parallel batch 2** (after Task 3 lands):
- Task 5 (verification tests) — needs the registry/state-injection paths stable.
- Task 6 (dead API removal) — `Routes()` and `closeErr` are surgical.

**Parallel batch 3** (no code dependencies):
- Task 2 (bench module) — separate module, separate `go.sum`.
- Task 9 (v2.0 roadmap scaffolding) — pure documentation; can run alongside everything.

## Notes

- Time estimates assume focused work; add 30% buffer for review cycles.
- v1.4 is the last v1.x line release with the strict-backward-compat promise. v2.0 (`roadmaps/v2.0/`) lifts that promise for a single deliberate cut. Schedule v2.0 work to start when v1.4 closes.
