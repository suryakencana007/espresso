# Task 8: Handler-Cache Growth Documentation

**Priority:** ⚪ Cleanup
**Estimated Effort:** 0.5 day
**Dependencies:** None

## Context

`handlerCache` (`handler.go`) is a process-global `sync.Map[reflect.Type]*handlerInfo` that caches reflection results so each route's signature is parsed once. It has no eviction.

For the typical Espresso application — routes registered statically at startup, never changed — this is fine and finite. But applications that register dynamically-generated function types per request (plugin systems, per-tenant code generation, `reflect.MakeFunc` scenarios) will accumulate one cache entry per unique `reflect.Type` for the life of the process.

This is a known limitation. v2.0 plans to fix it (`roadmaps/v2.0/tasks/task-03-handler-cache-eviction.md`). v1.4 documents it.

## Acceptance Criteria

- [x] `handler.go` doc comment on `handlerCache` explains:
  - The cache is process-global with no eviction.
  - Size is bounded by distinct handler `reflect.Type`s registered across the process lifetime.
  - When this is fine (static routes at startup).
  - When it could grow unboundedly (dynamic handler registration).
  - The escape hatch: typed handlers (`Ristretto`/`Solo`/`Doppio`/`Lungo`, `HandlerCtx*` wrappers) skip this cache entirely.
- [x] `docs/performance.md` carries the same content under a new "Known Limitations" section, with a forward pointer to v2.0's eviction work.

## Technical Approach

### Step 8.1: handler.go doc comment

Drop a multi-paragraph comment above the `handlerCache` declaration. Cover, in order:

1. What the cache stores and why (reflect-parsed signatures, pools, extractor metadata).
2. Growth: bounded by distinct registered types in normal use; never evicted.
3. When it could grow without bound (dynamic registration scenarios).
4. The escape hatch: typed handler functions don't touch the cache.

### Step 8.2: docs/performance.md

Add a "Known Limitations" section. Same content as the doc comment, in user-facing prose. Link forward to `roadmaps/v2.0/tasks/task-03-handler-cache-eviction.md` so users following the issue can see when it's planned to land.

## Tests Required

None (documentation only).

## Definition of Done

- [x] `handler.go` doc comment present and clear.
- [x] `docs/performance.md` "Known Limitations" section present.
- [x] Both documents agree on the escape hatch (typed handlers).
- [x] CHANGELOG `[Unreleased]` → `Added` entry mentioning the documentation.
