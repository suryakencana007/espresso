# Execution Order for v2.2.0

This document provides the recommended execution order for the v2.2 roadmap tasks. The schedule is sized for approximately one focused week. v2.2 is a correctness pass — no new feature surface — so the work is narrow but cross-package-sensitive in one spot (Task 3).

## Overview (planned)

```
Day 1-2:  Task 1 (reflection two-extractor) ‖ Task 2 (service-layer error→status) — PARALLEL, disjoint files
Day 3:    Task 3 (error-envelope consistency) — starts AFTER Task 2 lands (shares error.go)
Day 4:    Task 3 finishes (cycle-safe leaf pkg + unify) + Task 4 (verification) once 1/2/3 are in
Day 5:    Task 5 (CHANGELOG & v2.2.0 release)
```

The shape is deliberately front-loaded: the two P0 defects touch disjoint files and can be dispatched together, leaving the riskiest task (the cross-package envelope unification) for after `error.go` has settled, the verification sweep for after all three fixes are in, and the release for last.

## Week 1 — Dial It In

The theme is the release tagline: tune behavior until it matches contract. Every day below ends at a PR; every PR rebases against the running `[Unreleased]` CHANGELOG section.

### Day 1-2 — `Reflection two-extractor` (Task 1) ‖ `Service-layer error→status` (Task 2)

These two run in **parallel**. Task 1 lives in `handler.go`; Task 2 lives in `error.go` + `middleware/service/`. No file overlap, so dispatch both at once (isolated worktrees, as in v2.1's tasks 02/03/04 round).

**Task 1 — reflection-path two-extractor handlers (`handler.go`)**

- Step 1.1: Re-read the reflection registration path — `handlerInfo`'s single request slot (`reqPool`/`reqType`/`reqIndex`) and the arg loop that overwrites it on the second `FromRequest` arg. Confirm the request-time `panic("espresso: invalid handler argument - this is a bug")` at `handler.go` ~line 744 with a reproduction (reflection path panics → 500 under `RecoverMiddleware`; the typed `HandlerCtxReq1Req2Err`/`Lungo` path populates both extractors).
- Step 1.2: Pick **one** of the two documented approaches and record it in the PR:
  - **(A)** extend the reflection path to carry multiple request slots so `func(ctx, *Req1, *Req2) (T, error)` works, mirroring `HandlerCtxReq1Req2Err`; or
  - **(B)** fail-fast — detect `>1 FromRequest` arg in `handlerFunc` and `panic` at **registration** with an actionable message (`"two-extractor handlers require HandlerCtxReq1Req2Err / Lungo"`).
- Step 1.3: Implement. Either way, the request-time `"this is a bug"` panic must become unreachable.
- Step 1.4: Add the regression test (per approach: a working two-extractor reflection handler for A, or a registration-time panic assertion for B).
- Step 1.5: Open the Task 1 PR. Note the chosen approach in the description; this determines whether the CHANGELOG entry lands under "Fixed" (B) or "Added" (A).

**Task 2 — service-layer error → HTTP status mapping (`error.go`, `middleware/service/`)**

- Step 2.1: Re-read `writeHandlerError` (`error.go` ~221-229) — today it only `errors.As`-matches `*espresso.Error`; everything else collapses to `ErrInternal` → HTTP 500. Confirm the three service-layer cases (proven to all return 500 `INTERNAL`): `ValidationLayer` → `ErrValidation{}`, open `CircuitBreaker` → `*servicemiddleware.CircuitBreakerError`, `TimeoutLayer` → `context.DeadlineExceeded`.
- Step 2.2: Wire the already-existing mapping infra into the response path: `ValidationErrors()` (400/`VALIDATION_ERROR`), `IsCircuitBreakerError`, `ErrServiceUnavailable` (503). Decide and record: validation → 400 (matches auto-validate-on-extract's `ErrBadRequest`) vs 422; circuit-breaker-open → 503; timeout → 503 vs 504. Preserve validation field detail. Keep the unknown-error → 500 fallback intact.
- Step 2.3: ⚠ **Update** the existing regression-lock `TestWithLayersTyped_WithTimeout` (`withlayers_test.go` ~144-167) — it currently asserts `http.StatusInternalServerError` for a timeout. That assertion changes here; the change must be called out explicitly in the PR. Auto-validate-on-extract (already correct at 400) stays out of scope.
- Step 2.4: Run `go test ./... -race` clean. Open the Task 2 PR.

### Day 3-4 — `Structured-JSON envelope on every error path` (Task 3)

Starts **after Task 2 lands** — both touch `error.go`, and serializing the order avoids a guaranteed conflict on that file. This is the riskiest task (cross-package, import-cycle-sensitive), which is why it is P1 and sequenced behind the P0 work rather than parallel to it.

- Step 3.1 (**first acceptance criterion**): CONFIRM/characterize current behavior with a test before changing anything. This finding was surfaced by analysis, not yet independently locked: (a) `RecoverMiddleware` emits a hand-rolled anonymous-struct JSON that omits `"details"` with code `"PANIC"`; (b) auth middleware (JWT/BasicAuth/APIKey) and the rate limiter emit `http.Error` `text/plain` (401/429).
- Step 3.2: Respect the dependency direction (`root → httpmiddleware`; `middleware/http` does **not** import root). `httpmiddleware` cannot import the root package to reach `writeErrorResponse`/`*espresso.Error` without an import cycle — this is exactly why `RecoverMiddleware` hand-rolls its JSON today. Resolve it cycle-safely, e.g. extract the envelope writer/shape into a shared stdlib-only **leaf** package that both root and `httpmiddleware` import (mirroring the `internal/validatehook` cycle-break), or another cycle-safe approach. Document the chosen mechanism in the PR.
- Step 3.3: Unify the bypassing paths onto the canonical `{"error":{"code","message","details","request_id"}}` envelope — `RecoverMiddleware` (code `PANIC`, now with `"details"`), auth middleware (401), and the rate limiter (429).
- Step 3.4: Confirm `TestWithLayers_ExtractorErrorReturnsStructuredJSON` still holds (it locks the extractor/handler/SSE/WS paths) and that the newly-unified paths assert the same shape.
- Step 3.5: Run `go test ./... -race` clean. Open the Task 3 PR.

### Day 4 — `Verification` (Task 4)

Depends on Tasks 1, 2, and 3 being in. Lands once the substantive fixes have merged so it tests reality, not intent.

- Step 4.1: Status-code matrix — table-driven test asserting each service-layer error surfaces its mapped status (validation, circuit-breaker-open, timeout) per the decisions recorded in Task 2.
- Step 4.2: Signature coverage — assert the two-extractor reflection outcome chosen in Task 1 (works end-to-end for A, or panics at registration for B), alongside the typed `Lungo` path that already works.
- Step 4.3: Doc/code consistency — assert the envelope shape is uniform across every error path (extractor, handler, service-layer, SSE, WS, panic, auth, rate-limit).
- Step 4.4: Open the Task 4 PR.

### Day 5 — `CHANGELOG & v2.2.0 release` (Task 5)

Last task. Depends on 1, 2, 3, 4.

- Step 5.1: Promote `[Unreleased]` → `[2.2.0]` in `CHANGELOG.md` in a single atomic commit. The status-code change (F-2) and any envelope change (F-3) go under **Changed** with before/after; the two-extractor fix goes under **Fixed** (or **Added** if approach A was chosen). Include a short upgrade note for the status-code change.
- Step 5.2: Bump version chips — `package.json` to `2.2.0` and `docs/.vitepress/config.ts` to `v2.2.0`.
- Step 5.3: Final quality gates: `go test -race ./...`, `golangci-lint run ./...`, bench module spot-check.
- Step 5.4: Tag `v2.2.0` from the merge commit, push, `gh release create v2.2.0` with the `[2.2.0]` body.

## Contingency Planning

### Must Not Slip (hard requirements for the v2.2 release)

- Task 1 (two-extractor reflection defect) — registered handlers that panic per-request are a correctness defect; v2.2's whole thesis is making behavior match contract.
- Task 2 (service-layer error→status) — service-layer errors collapsing to 500 mislead every caller doing status-based handling; this is the headline correctness fix.
- Task 5 (release).

### Can Slip to v2.2.1 (next patch)

- Task 4 (verification) — the matrix/signature/consistency tests harden the fixes but don't change behavior. If the cycle runs hot, a thinner version can land with v2.2.0 and the full matrix follow in the patch. Prefer not to slip it, but it is the safest cut.

### Can Slip to v2.3 (next minor)

- Task 3 (error-envelope consistency) — it is P1 precisely because it is the riskiest (cross-package, cycle-sensitive). If the leaf-package extraction proves thornier than budgeted, the two error paths can stay as-is for one more minor without blocking the P0 correctness fixes. The `RecoverMiddleware` hand-rolled JSON has shipped since v2.0; one more cycle is tolerable.

### Must Not Compromise (quality gates)

- `go test -race ./...` clean.
- `golangci-lint run ./...` clean.
- Test coverage holds at or above the project minimum; every behavior change lands locked by a test (the v2.2 emphasis: when docs and code disagree, fix whichever is wrong and lock it).

## Parallel Work Opportunities

**Parallel batch 1** (no cross-file conflicts):
- Task 1 (`handler.go`) and Task 2 (`error.go`, `middleware/service/`) touch disjoint files — dispatch together on Day 1.

**Strictly serialized** (shared file):
- Task 3 shares `error.go` with Task 2 and so must land **after** Task 2. It is not a parallel candidate — sequencing it avoids a guaranteed `error.go` conflict and lets Task 3 build on the just-wired mapping.

**Downstream** (after the fixes are in):
- Task 4 (verification) is downstream of 1/2/3 by definition; Task 5 (release) is downstream of everything.

## Notes

- All v2.2 work happens on the `/v2` module path; no module-bump considerations.
- v2.2 adds **no** new packages except possibly the shared stdlib-only leaf package required by Task 3's cycle-break — that is plumbing for a correctness fix, not new feature surface, and should be named to read as such.
- Resist scope creep. v2.2 is a correctness pass: fix the three findings, lock them with tests, ship. If `handler.go`, `error.go`, or the middleware packages reveal adjacent cleanup, capture it in a follow-up issue rather than bundling.
- The CHANGELOG `[Unreleased]` section will take conflicts on each PR after the first, as in v2.1 — these are one-file, one-section rebases and resolve in seconds.
- Task 2 and Task 3 both change observable HTTP behavior. The release note in Task 5 must make the status-code shift (and any envelope-shape shift) explicit so downstream callers — Barista in particular — can adjust status-based handling before upgrading.
