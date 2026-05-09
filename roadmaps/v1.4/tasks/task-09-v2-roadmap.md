# Task 9: v2.0 Roadmap Scaffolding

**Priority:** 📦 Meta
**Estimated Effort:** 1 day
**Dependencies:** None — but should land near the end of v1.4 so the roadmap reflects what v1.4 actually did or didn't carry

## Context

Through the v1.x line, several "this is wrong but breaking changes are forbidden" items accumulated: process-global stream registries, duplicate symbol names (`NewBadRequest` vs `ErrBadRequest`, `ErrorResponse` alias, deprecated `SSEWriter` / `SSE` response), untyped `Validation(any)`, unbounded `handlerCache`.

v1.4's hardening work made the cost of those items concrete enough to scope a deliberate break. v2.0 will be that break. v1.4 publishes the roadmap for it.

Scaffold `roadmaps/v2.0/` mirroring the `roadmaps/v1.3/` layout so contributors and AI agents have one consistent format across versions.

## Acceptance Criteria

- [x] New directory `roadmaps/v2.0/` with:
  - `README.md` — why v2, principles, task index, out-of-scope list, project info.
  - `AGENT_GUIDELINES.md` — delta-only against v1.3 guidelines (breaking allowed-with-justification, module path bump, migration guide as first-class deliverable).
  - `EXECUTION_ORDER.md` — week-by-week schedule with contingency planning.
  - `tasks/` directory containing seven task files.
- [x] Seven tasks scoped:
  - `task-01-per-router-registries.md` (P0, kills `defaultRegistry` / `defaultSSERegistry`).
  - `task-02-remove-deprecated-apis.md` (P0, removes `Deprecated:` and dead duplicates).
  - `task-03-handler-cache-eviction.md` (P1, bounded cache + metrics hook).
  - `task-04-typed-validation-layer.md` (P1, `Validation(any)` → `Validation[Req](Validator[Req])`).
  - `task-05-auto-validate-on-extract.md` (P1, opt-in router flag).
  - `task-06-migration-guide.md` (Meta, `docs/migration-v1-to-v2.md`).
  - `task-07-changelog-release.md` (Meta, version bump + module path to `/v2`).
- [x] Each task file follows the v1.3 task-template structure (Context, Acceptance Criteria, Technical Approach, Tests Required, Definition of Done).
- [x] README's "Out of Scope" section explicitly lists items NOT in v2.0 (GraphQL/gRPC adapters, WebSocket pool, fasthttp).

## Technical Approach

### Step 9.1: Pull principles forward

Copy v1.3's design principles into v2.0's README, then mark the one that flips: backward compatibility. Document the carve-out: deprecated APIs, zero-caller-internal symbols.

### Step 9.2: Module-path bump

Document up front in `README.md` and `AGENT_GUIDELINES.md`: v2 changes the import path from `github.com/suryakencana007/espresso` to `github.com/suryakencana007/espresso/v2`. This is per Go's module convention, not a stylistic choice. It happens as part of Task 7 in a single atomic commit.

### Step 9.3: Migration guide as first-class

Make Task 6 a real task with its own file, not a footnote. Every breaking change in Tasks 1-5 must contribute a migration recipe to `docs/migration-v1-to-v2.md` in the same PR that lands the break. Recipes include `gofmt -r` rewrites where mechanical.

### Step 9.4: Out of scope

Explicitly list:

- GraphQL / gRPC adapters (TODOS #8, #9) — additive, ship as v2.x minors.
- WebSocket connection pool (TODOS #5 carryover) — additive, v2.x minor.
- Multi-tier handler-cache partitioning — speculative, no user pain.
- `net/http` → `fasthttp` — too disruptive, doesn't match Espresso's design posture.

## Tests Required

None — this task ships documentation only.

## Definition of Done

- [x] All 10 files present (`README.md`, `AGENT_GUIDELINES.md`, `EXECUTION_ORDER.md`, 7 task files).
- [x] `roadmaps/v2.0/README.md` `Task Index` cross-links each task by file name.
- [x] CHANGELOG `[Unreleased]` → `Added` entry referencing `roadmaps/v2.0/`.
- [x] No code changes ship under this task — pure documentation.
