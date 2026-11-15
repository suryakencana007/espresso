# Task 4: CI test/vulncheck jobs on Go 1.27

**Priority:** 🟡 P1 — hardening (tooling)
**Estimated Effort:** 0.5 day
**Dependencies:** None (soft prerequisite for CI visibility on Tasks 1, 2, 5)

> **Status: ✅ Shipped 2026-11-15 (v2.5.0).** Delivered via #81.

## Context

`govulncheck` was red on **every** branch, including a PR that only touched
`.gitignore`. Root cause was toolchain drift, not a code regression:

- CI pinned `go-version: "1.25.x"` → runner used go1.25.14.
- `govulncheck@latest` (x/vuln v1.9.0) flagged nine standard-library
  advisories fixed only in Go 1.26.9–1.26.11 — `net/http` (GO-2026-6859,
  6617, 6613, 6612, 6611, 6603), `net/textproto` (6608), `crypto/tls`
  (6607), `crypto/x509` (6809).
- `main` was last green 2026-07-08, before those advisories were published.

Separately, CI never exercised the Go 1.27 toolchain that issue #76 targets —
which is how the sonic fallback slipped through.

## Acceptance Criteria

- [x] `govulncheck` reports 0 affecting vulnerabilities.
- [x] Test and vulncheck jobs run on Go 1.27.
- [x] Lint job unchanged (still passes).
- [x] The `go mod tidy is clean` gate still passes under Go 1.27.

## Technical Approach

### Step 4.1 — Bump the two jobs

`.github/workflows/ci.yml`:

- `test` job: `go-version: "1.25.x"` → `"1.27.x"`.
- `vulncheck` job: `1.25.x` → `"1.27.x"`.
- `lint` job: **stays** `1.25.x`, with a comment. golangci-lint is pinned at
  v2.11.3, which predates Go 1.27, and bumping it would pull in new analyzers
  that currently flag unrelated pre-existing findings.

### Step 4.2 — Validate under Go 1.27.4 before pushing

```sh
GOTOOLCHAIN=go1.27.4 go mod tidy            # clean (the CI gate)
GOTOOLCHAIN=go1.27.4 go vet ./...           # clean
GOTOOLCHAIN=go1.27.4 go test -race -shuffle=on -count=1 ./...
GOTOOLCHAIN=go1.27.4 go run golang.org/x/vuln/cmd/govulncheck@latest ./...  # 0 affecting
```

## Tests Required

- No new unit test. The fix is validated by the CI jobs themselves (all three
  green on #81) plus the local Go 1.27.4 run above.

## Definition of Done

- [x] All three CI jobs green on the PR.
- [x] CHANGELOG `[2.5.0]` `Changed` entry references #81.
