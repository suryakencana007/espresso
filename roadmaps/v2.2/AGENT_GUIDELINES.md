# Guidelines for AI Agents — v2.2

Read this file **in addition to** `roadmaps/v1.3/AGENT_GUIDELINES.md` (the baseline), `roadmaps/v1.4/AGENT_GUIDELINES.md`, `roadmaps/v1.5/AGENT_GUIDELINES.md`, `roadmaps/v2.0/AGENT_GUIDELINES.md`, and `roadmaps/v2.1/AGENT_GUIDELINES.md`. Most rules carry forward unchanged. This file only lists what is **different** for v2.2.

## What's Different in v2.2

### 1. This Is a Correctness Release — Behavior Changes Are the Point

v2.1 reverted to a no-breaking-changes posture. v2.2 deliberately changes observable behavior, because the behavior was *wrong* relative to the documented/expected contract. In scope and expected:

- Changing HTTP **status codes** for service-layer errors (Task 2: 500 → 400/503).
- Converting a request-time panic into either real support or a **registration-time** fail-fast (Task 1).
- Converting `text/plain` error responses into the canonical JSON envelope (Task 3).

These are not "breaking for breaking's sake" — each closes a gap where code contradicted its contract. But they *are* observable, so every one must be documented under CHANGELOG `Changed` with a before/after, and surfaced in the Task 5 upgrade note. No new feature surface beyond what a fix strictly requires.

### 2. Never Quietly Flip a Regression-Locked Test

Task 2 changes `TestWithLayersTyped_WithTimeout` (`withlayers_test.go`), which currently asserts `500` for a timeout. When you change the asserted value of a test that *locks* behavior, you **must** call it out explicitly in the PR description as a deliberate change — show the old assertion and the new one. A reviewer must never have to wonder whether a flipped lock was intentional. This is the single most important discipline of this release.

### 3. Confirm Before You Fix (Task 3 especially)

F-1 and F-2 shipped with runnable reproductions; F-3 (error-envelope consistency) was surfaced by analysis but **not yet independently locked**. So Task 3's *first* acceptance criterion is to characterize the current behavior with a test (panic omits `details`; auth/rate-limit emit `text/plain`), and only then unify. Don't assume the finding — pin it, then flip it.

### 4. Respect the `root → httpmiddleware` Import Direction

`error.go` imports `middleware/http` (for `GetRequestID`); `middleware/http` does **not** import the root package, and must not — root's test files import `httpmiddleware`, so a back-edge is an import cycle. This is *why* `RecoverMiddleware` hand-rolls its JSON today. Task 3 must break the cycle the way `internal/validatehook` already does: a stdlib-only **leaf** package that both sides import. Do not "fix" the envelope by importing root into middleware.

### 5. Prefer Correcting Behavior Over Adding Surface

When docs/godoc and code disagree, the default is to make the code match the documented/expected contract and lock it with a test — not to add a new knob or type. The only new package this release should introduce is the cycle-break leaf for Task 3 (plumbing, not feature surface), and it should be named to read as such.

## Carried Over From Earlier Versions

Re-read the prior guidelines if you haven't recently. The following remain unchanged and not repeated here:

- Read before writing (`handler.go`, `core.go`, `error.go`, `router.go`, `middleware/service/layer.go`, extractor patterns).
- Coffee metaphor for any new public surface.
- Type-safety over `any`.
- `context.Context` mandatory on I/O.
- `sync.Pool` for hot paths; atomic over mutex on hot paths.
- Conventional Commits, small focused commits, feature-branch + PR (UI merge — branch protection blocks CLI merge).
- `cmd/example/` updates when user-facing behavior changes.
- Race detector mandatory; `golangci-lint` (gocyclo min 15) clean.

## Common Mistakes to Avoid (v2.2-specific)

1. **Flipping `TestWithLayersTyped_WithTimeout` without flagging it.** The assertion change is correct (Task 2), but a silent edit reads as a regression. Call it out.
2. **Importing the root package into `middleware/http`.** That is the import cycle the whole framework has avoided. Task 3 goes through a shared leaf, not a back-edge.
3. **Touching the per-request hot path harder than the fix needs.** Task 1 approach B (fail-fast) leaves `createHandlerFromInfo` untouched; approach A (multi-slot) changes it and must be re-verified with `-race` and a gocyclo check. Pick deliberately and record the choice.
4. **Re-introducing auto-validate-on-extract into Task 2.** The extract path is already correct (400). Task 2 is only about the service-**layer** path.
5. **Bumping the version chip / `package.json` before Task 5.** Same atomic-release discipline as v2.0 task-07 / v2.1 task-06.
6. **Shipping a status-code or content-type change without an upgrade note.** Barista (and any caller keying on 500 / `text/plain`) needs to adjust before upgrading; Task 5's note is mandatory, not optional.
