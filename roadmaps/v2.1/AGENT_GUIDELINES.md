# Guidelines for AI Agents — v2.1

Read this file **in addition to** `roadmaps/v1.3/AGENT_GUIDELINES.md`, `roadmaps/v1.4/AGENT_GUIDELINES.md`, `roadmaps/v1.5/AGENT_GUIDELINES.md`, and `roadmaps/v2.0/AGENT_GUIDELINES.md`. Most rules carry forward unchanged. This file only lists what is **different** for v2.1.

## What's Different in v2.1

### 1. Module Path Is Already `/v2`

Don't repeat the module-path bump. `go.mod` already declares `github.com/suryakencana007/espresso/v2` and every internal import already points at `/v2`. New code added in v2.1 follows the same convention; no rewriting required.

### 2. Backward Compat Bar Is Mid-Cycle

v2.0 lifted the strict no-breaking-changes promise for one major cut. v2.1 reverts to the v1.x posture: **no breaking changes** unless they're carry-over debt explicitly deferred from v2.0. The two breaking items in this roadmap (deprecated SSE removal, Stream pre-flight phase) both qualify under that carve-out — they're documented in v2.0 task-02 (deferral) and `roadmaps/USAGE_ESPRESSO.md` F-02 (deferral notes), respectively.

If you discover something v2.1 should *also* break, stop and ask. Drift here gives v2.0 callers a worse experience than they signed up for.

### 3. Migration-Guide Continuity

`docs/migration-v1-to-v2.md` exists and covers v1 → v2.0. v2.1 ships its own `docs/migration-v2-to-v2.1.md` with **only** the v2.1 deltas — do not duplicate v2.0 entries. Cross-link both guides from a top-level "Migration" entry in the docs nav so users on either old version land in the right place.

### 4. F-02's Fix Lands on v2.0's `serveStream` Restructure

v2.0 task-01 (per-Router registries) added `routerRegistriesFrom(ctx)` and the `withRouterRegistries` injection in `Router.ServeHTTP`. The `serveStream` helper got refactored along the way. v2.1's F-02 fix should build on those primitives — don't reintroduce a parallel pre-flight path. If the existing structure makes the pre-flight phase awkward, surface the friction in the PR description rather than working around it.

### 5. Validator Adapter Is Tiny — Resist Scope Creep

Task 3's `validator.AsDefaultValidator()` is documented as ~10 LOC. It's that small because it has exactly one job: wrap `validator.Struct` in the `func(any) error` shape with the `FieldErrors → ValidationErrors` adapter the user-facing example already shows. Resist the urge to add knobs (custom error mappers, configurable code, etc.) — those belong in a future PR if real users ask for them.

## Carried Over From Earlier Versions

Re-read the prior guidelines if you haven't recently. The following remain unchanged and not repeated here:

- Read before writing (handler.go, core.go, state.go, router.go, extractor patterns).
- Coffee metaphor for new public surface.
- Type-safety over `any`.
- `context.Context` mandatory on I/O.
- `sync.Pool` for hot paths.
- Conventional Commits, small focused commits.
- `cmd/example/` updates when user-facing APIs change.
- Race detector mandatory.

## Common Mistakes to Avoid (v2.1-specific)

1. **Removing the `// Deprecated:` markers without deleting the symbol.** If you're touching a deprecated SSE type, the only valid action in v2.1 is delete-it-entirely. Half-states (rename, refactor-but-keep) are net negative — they break callers without buying us anything.
2. **Adding new public symbols to `response.go` while removing the SSE legacy bits.** Task 1 is purely subtractive. Net code is lower after; net public surface is smaller.
3. **Reintroducing a parallel pre-flight path in `Stream`.** v2.0 already restructured `serveStream`; F-02's fix extends it, doesn't fork it.
4. **Writing the validator adapter without using the existing `FieldErrors.ToValidationErrors()` method.** That conversion exists; the adapter just wires it up.
5. **Bumping the version chip / `package.json` before task-06.** Same atomic-commit discipline as v2.0 task-07.
