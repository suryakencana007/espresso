# Execution Order — v2.5.0

> **Retrospective.** This document records the **actual** order in which the
> v2.5.0 work landed, not a plan. v2.5 was issue-driven and shipped on
> 2026-11-15 in a single working session, so the "week/day" shape below is a
> phase breakdown rather than a calendar.

## Overview

```
v2.5 "Headers & Recovery" — actual sequence

  Phase 0  issue triage (#76, #77) + reproduction
      │
  Phase 1  #76  sonic v1.15.3 + Go 1.27 build fix            → PR #78 (merged)
      │
  Phase 2  #77  Error.WithHeader (set) + WithHeaderAdd (add) → PR #79 (opened)
      │
  Phase 3  CI was red on every branch (govulncheck, Go 1.25 stdlib)
      │      bump test + vulncheck jobs to Go 1.27            → PR #81 (merged first)
      │
  Phase 4  #82  circuit-breaker half-open probe race
      │      (surfaced by #79's CI failure)                   → PR #83 (merged)
      │
  Phase 5  rebase #79 onto the fixed main, resolve CHANGELOG  → PR #79 (merged)
      │
  Phase 6  .gitignore snip-* hygiene                          → PR #80 (merged)
      │
  Phase 7  atomic release cut + tag + GitHub release          → PR #84, tag v2.5.0
      │
  Phase 8  retrospective back-fill (this directory)           → PR (docs/roadmap-v2.5)
```

## Phase 0 — Triage and reproduction

Started from `gh issue list`: exactly two open issues, #76 (sonic/Go 1.27)
and #77 (`Error.WithHeader`). Both were read in full before any code. The
reproduction and design decisions recorded there (the tamper#163 case, the
any-header-vs-allowlist question) became the task specs.

## Phase 1 — Task 3: sonic v1.15.3 (issue #76)

`go get github.com/bytedance/sonic@v1.15.3` in both modules, `go mod tidy`,
full test/build. Independent of everything else, so it landed first. Confirmed
the fallback warning appears with v1.15.0 and disappears with v1.15.3 under a
real `go run` on Go 1.27.4 (the warning is emitted at package init, so it does
not surface under `go test`).

## Phase 2 — Task 1: `Error.WithHeader` (issue #77)

The larger change. Added the `headers` field, `WithHeader`, the reserved-header
denylist, and `applyErrorHeaders` wired into `writeErrorResponse`. Tests:
builder, `writeHandlerError`, an end-to-end router dispatch, Set semantics,
reserved-header rejection, and a byte-identity guard for the zero-header path.
Docs updated in `docs/api/espresso.md` and `docs/error-handling.md`.

## Phase 3 — Task 4: un-red CI (prerequisite for everything else)

PR #79's CI went red — but so did every other branch's, including a PR that
only touched `.gitignore`. Root cause was environmental: CI pinned Go 1.25
and `govulncheck` flagged nine Go 1.25 standard-library advisories fixed only
in Go 1.26.9+. Bumped the test and vulncheck jobs to Go 1.27 (the toolchain
issue #76 is about), kept lint on Go 1.25 for its pinned golangci-lint. PR #81
merged **first** so the remaining PRs could re-run green.

## Phase 4 — Task 2: circuit-breaker probe race (issue #82)

While reviewing #79's red CI, the failing test was
`TestCircuitBreaker_HalfOpenLimitsConcurrentProbes`. It was **not** a flaky
test — it reproduced the real defect that concurrent `Open → HalfOpen`
transitions each reset the in-flight probe counter. Filed #82, fixed the
transition to re-check state under the write lock, and hardened the test
(repeated bursts behind a start barrier) so the pre-fix code fails
deterministically. Landed as #83.

## Phase 5 — Task 1 (cont.): land #79

With the circuit-breaker fix on `main`, #79 was merged-up. The `CHANGELOG.md`
`[Unreleased]` sections conflicted each time a sibling release PR merged (both
insert at the top of the file); each conflict was resolved by keeping both the
`Added` and `Fixed` blocks. #79 then went green and merged.

## Phase 6 — Task 5: `.gitignore` hygiene

The local toolchain emitted random-suffixed test binaries (`snip-<digits>`)
into the working directory on every run. Ignored with the `snip-*` pattern;
trivial, landed as #80.

## Phase 7 — Task 7: release cut

Single atomic commit: `[Unreleased]` → `[2.5.0] - 2026-11-15`, new empty
`[Unreleased]`, `package.json` 2.4.0 → 2.5.0, the `docs/.vitepress/config.ts`
version chip and migration links, and the new
`docs/migration-v2.4-to-v2.5.md`. Merged as #84, tagged `v2.5.0` from the merge
commit `bb50ab5`, GitHub release published, and a throwaway project smoke-built
`go get …/v2@v2.5.0`.

## Phase 8 — Task 6 + retrospective

Verification ran throughout; this directory is the post-release back-fill.

---

## Contingency Planning

### Must Not Slip

- The additive contract: no existing symbol changes signature.
- The zero-header error response stays byte-identical.
- The circuit-breaker fix must be locked by a test that fails on the pre-fix
  revision.

### Could Slip to v2.5.1

- `WithHeaderAdd` — the review addition to #77. `WithHeader` alone satisfies
  the issue; the append variant could have been a follow-up patch.

### Could Slip to v2.6 / v3

- A broader error-path `IntoResponse` escape hatch (letting a custom error
  type implementing `WriteResponse` be honored by `writeHandlerError`), which
  #77 noted as a limitation but deliberately did not implement.
- Per-Router handler cache configuration (still deferred from the v2.x line).

### Must Not Compromise

- `go test -race -shuffle=on -count=1 ./...` green.
- `go vet` clean; `golangci-lint` no new findings; `govulncheck` clean.
- Docs snippet-compile and drift-guard tests green.
- Single atomic release commit; tag cut from the merge commit.

## Parallel Work Opportunities

Tasks 1, 2, and 3 touch disjoint files (`error.go`/`error_test.go`,
`middleware/service/layer.go`/`layer_test.go`, `go.mod`/`go.sum`) and were
worked in sequence only because one author was context-switching. Tasks 5 and
7 are trivial and conflict-free. Task 4 (CI) is a soft prerequisite for CI
visibility, not for the code.

## Notes

- The most expensive surprise was Task 4: an unrelated red check blocked every
  PR and had to be diagnosed (toolchain drift, not a code regression) before
  review could proceed. Budget time for green-field CI diagnosis when a repo
  has time-sensitive checks like `govulncheck`.
- The second surprise was Task 2: a "flaky" test turned out to be a real
  concurrency bug. The lesson is in the AGENT_GUIDELINES — verify a failing
  test against the pre-fix code before calling it flaky.
- `CHANGELOG.md`'s single `[Unreleased]` section conflicts across concurrent
  release PRs. Expect to resolve it on whichever PR merges last.
