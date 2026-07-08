# Task 11: Verification Matrix + Snippet-Compile Hardening

**Priority:** 🔵 Verification
**Estimated Effort:** 1 day
**Dependencies:** Tasks 1-10

> **Status: ✅ Shipped 2026-07-05 (v2.4.0).** Delivered via #73 — testDocsCorrectedClaims drift guard locking 11 audit-corrected phrases; caught docs/api/index.md:89 GetState signature regression PR #63 missed.

## Context

The v2.4 release is a correctness/quality pass; the whole point is that each P0/P1 fix lands **locked by a regression test shaped like the audit's repro**. This task is the safety net that catches: (a) a fix whose regression test does not actually reproduce the bug on pre-fix code, (b) a fix that broke an adjacent invariant, (c) a doc that drifted between Task 10's landing and the release, and (d) audit findings not owned by any Task 1-10 that were meant to be picked up.

The audit noted a real blind spot in the v2.3 snippet-compile guard (`docs_snippet_compile_test.go`): it skips fences with non-hermetic third-party imports (e.g. `golang-jwt/jwt` for the auth Complete Example). That is a known escape hatch — this task closes it enough that Task 10's rewrites are actually CI-covered.

## Acceptance Criteria

- [x] Every P0 task's regression test reliably **fails on the pre-fix commit** (spot-check via a temporary revert-branch; document the check in the PR body).
- [x] `go test -race -shuffle=on ./... -count=1` clean.
- [x] `go test -race -shuffle=on ./... -count=1` on Linux CI (Ubuntu-latest) clean — no OS-specific timing that only reproduces one way.
- [x] `go test -tags=integration ./tests/integration/...` clean.
- [x] `golangci-lint run ./...` clean.
- [x] `govulncheck ./...` clean on root **and** `bench/` (CI already covers this per #58; verify green on this PR).
- [x] The snippet-compile guard is extended to compile more fences. Two viable shapes below (Step 11.2) — pick one, document.
- [x] The v2.3-deferred non-hermetic fences (auth Complete Example with `golang-jwt/jwt`) are now CI-covered under the extended guard.
- [x] No unclosed audit finding in the P0/P1 tier remains. Cross-reference the audit report and confirm.

## Technical Approach

### Step 11.1 — Bisect each P0 regression test

For each Task 1-6 PR (the P0 tier):

1. `git checkout <PR-commit>~1` (parent of the fix commit).
2. Cherry-pick the regression test alone (skipping the code change).
3. Run the test; it must fail.
4. Return to `HEAD`; run the full test again; it must pass.

If any test does not fail on `~1`, the test does not lock the fix — write a follow-up commit to fix the test shape (not the code).

Same spot-check for Task 7's P1 defects (they are less severe but the same discipline applies).

### Step 11.2 — Harden the snippet-compile guard

Two shapes:

**(a) Whitelist third-party deps** — extend the guard to `go mod init` a temporary module, `go get` from an allowlist (`golang-jwt/jwt`, `bytedance/sonic`, `rs/zerolog`, `coder/websocket`), then `go build` the fence. Slower per-fence but keeps `docs/` self-documenting; the fence stays as it is written for readers.

**(b) Extract Complete Examples to `docs/examples/testdata/`** — move the `golang-jwt/jwt`-using Complete Example (and any other non-hermetic full-program example) into `docs/examples/testdata/<name>/main.go`, a real compilable subdirectory with a `go.mod` (or the root's). The docs page uses a `<<< @/examples/testdata/<name>/main.go` VitePress include directive. Faster, keeps snippet-compile guard simple, but decouples code from prose.

Prefer (b): matches how many mature docs sites handle "runnable" examples. The Complete Example becomes a small hidden subpackage the tree already knows how to build, and the docs page renders the file verbatim. VitePress supports `@/` file includes natively.

### Step 11.3 — Grep-based drift assertions

Add a small test file `docs_grep_asserts_test.go` (or extend `docs_consistency_test.go`) with cheap grep-based assertions:

```go
// t.Run("MiddlewareOrderNotBackwardsInDocs", func(t *testing.T) {
//     forbidden := []string{
//         "last added = first executed",       // guide/middleware/index.md
//         "Middleware runs in reverse order",   // examples/middleware-stack.md
//         "last added is executed first",      // service.go
//         "last added = outermost",             // withlayers.go
//     }
//     for _, phrase := range forbidden { assertAbsentFromDocs(t, phrase) }
// })
```

Same for: `"zero-allocation handlers"` (from Task 10.1), `"WithServer"` from `docs/api/*.md`, `"espresso.GetRequestID"` from guides, `"espresso.Path["` from docs (except migration guides which describe the fix).

These are cheap; they lock Task 10's work against future drift.

### Step 11.4 — Verify final CI matrix

Once Tasks 1-10 have all merged:

- Cut a preparation branch off `main`.
- Run locally: `go test -race -shuffle=on ./... -count=1` (Windows).
- Push and confirm the CI's three jobs (test, lint, govulncheck) are green.
- Run `go test -tags=integration ./tests/integration/... -timeout=10m` locally.

### Step 11.5 — Reconcile the audit findings list

Take the audit report (or the memory `full-audit-2026-07.md`) and mark each P0/P1 finding as either "closed by Task N" or "explicitly deferred (see README `Related, Deferred`)". No finding in the P0 tier is unaccounted for.

## Tests Required

This task adds tests; it does not add production code. New tests are all under existing packages or docs:

- Grep-based drift assertions in `docs_consistency_test.go` (Step 11.3).
- Snippet-compile guard extension (Step 11.2) as an internal change to `docs_snippet_compile_test.go`.
- The `docs/examples/testdata/` compilable examples become their own micro-programs the module builds under `go build ./docs/examples/testdata/...`.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] Each P0 regression test verified to fail on pre-fix commit (documented in the PR body).
- [x] `go test -race -shuffle=on ./... -count=1` clean on Windows and CI Linux.
- [x] `go test -tags=integration ./tests/integration/...` clean.
- [x] `golangci-lint run ./...` clean; `govulncheck ./...` clean.
- [x] Snippet-compile guard extended and the auth Complete Example (or its `testdata/` extraction) is now CI-covered.
- [x] Audit reconciliation table (Task 11.5) posted as a comment on this PR or added to `roadmaps/v2.4/README.md` `Related, Deferred`.
- [x] CHANGELOG `[Unreleased]` entry under `Changed` (tests/docs): snippet-compile guard extended to cover previously-skipped fences; regression tests added for every P0 finding.
