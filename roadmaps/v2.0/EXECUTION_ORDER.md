# Execution Order for v2.0.0

This document provides the recommended execution order for the v2.0 roadmap tasks. The schedule assumes one developer (human or AI agent) working full-time for approximately 3 weeks.

## Overview

```
Week 1: Task 1 (per-Router registries) — touches router, server, websocket, sse
Week 2: Task 2 (deprecated API removal) + Task 3 (cache eviction) + Task 4 (typed Validation)
Week 3: Task 5 (auto-validate) + Task 6 (migration guide) + Task 7 (CHANGELOG & release)
```

## Week 1 — Per-Router Registries

Goal: Complete Task 1 end-to-end. This is the riskiest task; it touches the most files.

### Day 1 (Monday)
- Read `websocket.go` and `sse.go` registry patterns end-to-end
- Read `server.go` graceful-shutdown ordering
- Task 1, Step 1.1: Add `wsRegistry` and `sseStreamRegistry` fields to `Router`
- Task 1, Step 1.2: Plumb `*wsRegistry` through to `WebSocket[T]` / `WebSocketSimple` handler wrappers

### Day 2 (Tuesday)
- Task 1, Step 1.3: Same plumbing for SSE (`Stream[T]` / `StreamSimple` wrappers)
- Update all call sites that previously referenced `defaultRegistry` / `defaultSSERegistry`
- Decide how wrappers receive the router reference: closure at registration time vs context.Value

### Day 3 (Wednesday)
- Task 1, Step 1.4: Update `gracefulShutdown` to drain the Router's registries (not globals)
- Delete the package-level `defaultRegistry` / `defaultSSERegistry` variables
- Run `go test -race` until clean

### Day 4 (Thursday)
- Task 1, Step 1.5: Add multi-router isolation test (two `Portafilter()` in same process, shutdown one does not touch the other)
- Update existing shutdown tests that assumed the global registry

### Day 5 (Friday)
- Open PR for Task 1; include migration note in the PR description
- Begin Task 2 (remove deprecated APIs) in parallel with review cycles

**Deliverable:** Task 1 merged or ready for review.

---

## Week 2 — Cleanup, Eviction, Typed Validation

### Day 1 (Monday)
- Task 2, Step 2.1: Remove `ErrorResponse` alias, `NewBadRequest`/`NewInternal` duplicates
- Task 2, Step 2.2: Remove deprecated `SSEWriter` and the old `SSE` response type
- Update every internal caller; compile and test

### Day 2 (Tuesday)
- Task 2, Step 2.3: Remove unused `closeErr` and any other fields flagged dead in v1.3 review
- Task 2 tests: ensure no path expects the removed symbols
- Open Task 2 PR

### Day 3 (Wednesday)
- Task 3, Step 3.1: Design the cache bound (LRU? size cap? TTL?)
- Task 3, Step 3.2: Implement bounded cache behind the existing cache API
- Task 3, Step 3.3: Add metrics hook (`OnCacheEvict(fn func(reflect.Type))`)

### Day 4 (Thursday)
- Task 3 tests: memory pressure + eviction correctness
- Open Task 3 PR
- Task 4, Step 4.1: Change `Validation(any)` → `Validation[Req](Validator[Req])`

### Day 5 (Friday)
- Task 4, Step 4.2: Update `LayerConfig` and downstream wiring for the typed generic
- Task 4 tests: existing validator implementations still compile (typed path)
- Open Task 4 PR

**Deliverable:** Tasks 2, 3, 4 merged or ready for review.

---

## Week 3 — Auto-Validate, Docs, Release

### Day 1 (Monday)
- Task 5, Step 5.1: Decide whether auto-validate runs from extractors or as a default layer (documented tradeoff; pick one and own it)
- Task 5, Step 5.2: Implement behind an opt-in router flag (`espresso.Portafilter().WithAutoValidate()`)

### Day 2 (Tuesday)
- Task 5 tests: struct-tag validation path exercised through all extractor variants
- Task 5, Step 5.3: Documentation + example
- Open Task 5 PR

### Day 3 (Wednesday)
- Task 6: Write `docs/migration-v1-to-v2.md` top to bottom
- Cover every breaking change introduced in Tasks 1-5
- Provide a "run this sed script" recipe for mechanical rename-style changes

### Day 4 (Thursday)
- Task 7, Step 7.1: Update `CHANGELOG.md` with a `[2.0.0]` section
- Task 7, Step 7.2: Bump module path to `github.com/suryakencana007/espresso/v2`
- Update all docs/examples to the new import path

### Day 5 (Friday)
- Task 7, Step 7.3: Final review; run full test suite + race detector + lint
- Task 7, Step 7.4: Tag `v2.0.0`; draft GitHub release notes

### Weekend
- Publish release notes; announce; monitor issue tracker for early adopters

**Deliverable:** v2.0.0 released; migration guide live.

---

## Contingency Planning

### Must Not Slip (hard requirements for v2.0 release)
- Task 1 (per-Router registries)
- Task 2 (deprecated API removal)
- Task 6 (migration guide)
- Task 7 (CHANGELOG & release)

### Can Slip to v2.1 (nice to have, not blocking)
- Task 4 (typed validation) — current `any`-typed Validation still works and can be tightened later as long as v2.0 doesn't lock it in the wrong shape
- Task 5 (auto-validate) — purely additive; fine to defer

### Can Slip to v2.0.x patch releases
- Task 3 (cache eviction) — only bites pathological dynamic-registration use cases; can land as a patch if the design needs more bake time

### Must Not Compromise (quality gates)
- `go test ./... -race` passing
- `golangci-lint run ./...` clean
- Migration guide covers every removed/renamed symbol
- Module path bump is present in `go.mod`

## Parallel Work Opportunities

**Parallel batch 1** (no cross-file conflicts):
- Task 2 (deprecated removal) and Task 3 (cache eviction) touch different files

**Parallel batch 2** (documentation + code):
- Task 6 (migration guide) can be drafted concurrently with Tasks 4-5

## Notes

- A v2 major bump renames the module path. This is a breaking change by itself and must land in the same commit set as `v2.0.0` tagging.
- Downstream consumers (Barista) should be pinged before `v2.0.0` ships so their migration can track.
