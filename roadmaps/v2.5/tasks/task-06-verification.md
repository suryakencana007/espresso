# Task 6: Full-gate verification

**Priority:** 🔵 Verification
**Estimated Effort:** 0.5 day
**Dependencies:** Tasks 1–5

> **Status: ✅ Executed 2026-11-15 (v2.5.0).** Ran across the cycle; the
> release commit was cut only after every gate was green.

## Context

v2.5 has no product-scope verification matrix akin to v2.4's — the surface is
one new error-path feature and two fixes. The verification task here is the
release gate itself: prove the whole tree is green on the toolchain the
release targets, on both modules, before the tag is cut.

## Acceptance Criteria

- [x] `go test -race -shuffle=on -count=1 ./...` green (root).
- [x] `go vet ./...` clean.
- [x] `golangci-lint run ./...` — no new findings on touched files.
- [x] `govulncheck ./...` — 0 affecting (under Go 1.27.4).
- [x] `bench/` builds; `cmd/...` builds.
- [x] Docs snippet-compile + drift-guard tests green.
- [x] Integration tests compile under `-tags=integration`.
- [x] Go 1.27.4 run of the full suite green.

## Technical Approach

### Step 6.1 — Local gates

```sh
go test -race -shuffle=on -count=1 ./...
go vet ./...
golangci-lint run ./...
go test -run 'TestDocs|TestDocsSnippetsCompile' .
go test -tags=integration -run xxx ./tests/integration/...
(cd bench && go build ./...)
```

### Step 6.2 — Targeted Go 1.27.4 run

```sh
GOTOOLCHAIN=go1.27.4 go vet ./...
GOTOOLCHAIN=go1.27.4 go test -race -shuffle=on -count=1 ./...
GOTOOLCHAIN=go1.27.4 go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

### Step 6.3 — The two fix-specific locks

- Task 1: `TestError_WithHeader_NoHeaderUnchanged` (byte identity) and the
  reserved-header table stay green.
- Task 2: the hardened probe-bound test fails on the pre-fix `layer.go` and
  passes 30/30 on the fixed one.

## Tests Required

No new tests — the release gate is the deliverable.

## Definition of Done

- [x] Every gate green; recorded in the release PR body (#84).
