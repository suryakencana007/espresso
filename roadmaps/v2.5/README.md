# Espresso v2.5.0 — Headers & Recovery

> **Status: shipped 2026-11-15.** Tag [`v2.5.0`](https://github.com/suryakencana007/espresso/releases/tag/v2.5.0).
> An additive minor release: handler-returned errors can set response headers, and the half-open circuit breaker respects its probe bound again.
> Upgrade notes: [`docs/migration-v2.4-to-v2.5.md`](../../docs/migration-v2.4-to-v2.5.md) and CHANGELOG `[2.5.0]`.
> This directory is a retrospective back-fill; v2.5 was issue-driven, not scheduled.

This directory contains the roadmap and task specifications for Espresso
v2.5.0, a **minor** release driven by the two issues left open at the end of
the v2.4 cycle plus one concurrency defect that CI surfaced while those were
being closed. Unlike v2.0–v2.4, v2.5 had no pre-written roadmap: the work
started from filed issues and is documented here after the fact, following
the repo's retrospective back-fill convention.

## Why v2.5?

At the end of the v2.4 cycle the tracker held exactly two open issues, and
the release line had a quiet correctness gap that only reproduced under CI
timing. v2.5 closes all three:

- **A handler-returned error cannot set a response header** (issue #77).
  Handlers receive `(context.Context, Req) (Res, error)` and never the
  `http.ResponseWriter`; `writeHandlerError` (`error.go`) only `errors.As`-es
  a `*espresso.Error` and writes the canonical envelope. A `429` therefore
  cannot carry `Retry-After` (RFC 9110 §10.2.3), and a custom error type
  implementing `WriteResponse` is not consulted on the error path. Found
  downstream in **tamper**: its identity rate limiter answered a 429 with no
  header and had to fall back to a `retry_after_seconds` body detail
  (suryakencana007/tamper#163).
- **`CircuitBreakerLayer` did not enforce `HalfOpenMaxProbes`** (issue #82,
  found by CI on #79). Every request that observed `StateOpen` with the open
  timeout elapsed re-ran the `Open → HalfOpen` transition and `Store(0)`-ed
  the in-flight probe counter, wiping the slot a peer had just acquired. More
  than `HalfOpenMaxProbes` probes could reach a just-recovering upstream —
  the exact stampede the v2.4 probe limit (task-07, D4) was added to prevent.
- **The build was not clean on Go 1.27** (issue #76). The pinned
  `bytedance/sonic v1.15.0` does not support Go 1.27: every process start
  printed a fallback warning and `JSON[T]` encoding silently dropped to
  `encoding/json`. Found while scaffolding a new downstream app on Go 1.27.0.

Two supporting changes made the release shippable: CI moved its test and
vulncheck jobs to Go 1.27 (`govulncheck` had gone red on every branch against
the Go 1.25 stdlib), and the local toolchain's random-suffixed test binaries
were ignored.

The headline is **the error path gains a contract it was missing, and the
recovery path honors the contract it already claimed**.

## Design Principles

Carried forward unchanged from v2.4:

1. **Coffee metaphor** for any public surface.
2. **Type-safety first** — generics over `any`.
3. **Zero-allocation hot paths** — performance-conscious; atomic over mutex on hot paths.
4. **State injection via `MustGetState[T]`** stays the canonical pattern.
5. **Context-first** on I/O.
6. **Structured errors** — `*espresso.Error` and the canonical JSON envelope are the contract.
7. **Trustworthy generated artifacts** — spec-inspection tests lock generator output.
8. **Truth-in-docs.** Docs and code are fixed together; never paraphrased apart.
9. **Regression tests written from the repro, not paraphrased from the fix.**

New emphasis for v2.5:

10. **Additive by default; the public surface only grows.** A minor release
    may add API, but must not move or re-sign an existing symbol. Every v2.5
    change is either a new method (`WithHeader`, `WithHeaderAdd`) or a fix
    that removes no surface. The migration guide's job is to say so.
11. **A fix that only holds on one schedule is not a fix.** The
    circuit-breaker probe bound was "tested" in v2.4 but the test tripped the
    race only once per few hundred runs. v2.5's regression test repeats the
    burst behind a start barrier so the pre-fix code fails deterministically.

## Task Index

### 🔴 P0 — Must Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 1 | `Error.WithHeader` / `WithHeaderAdd` on the handler error path | [task-01-error-with-header.md](./tasks/task-01-error-with-header.md) | 1.5 days |

### 🟡 P1 — Should Have

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 2 | Circuit breaker half-open probe-counter race | [task-02-circuitbreaker-halfopen-probe-race.md](./tasks/task-02-circuitbreaker-halfopen-probe-race.md) | 0.5 day |
| 3 | sonic v1.15.3 — Go 1.27 support | [task-03-sonic-go1.27.md](./tasks/task-03-sonic-go1.27.md) | 0.5 day |
| 4 | CI test/vulncheck jobs on Go 1.27 | [task-04-ci-go1.27.md](./tasks/task-04-ci-go1.27.md) | 0.5 day |

### ⚪ Cleanup

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 5 | Ignore toolchain `snip-*` test binaries | [task-05-hygiene-gitignore.md](./tasks/task-05-hygiene-gitignore.md) | 0.1 day |

### 🔵 Verification

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 6 | Full-gate verification (race, lint, vet, docs, integration) | [task-06-verification.md](./tasks/task-06-verification.md) | 0.5 day |

### 📦 Meta

| # | Task | File | Estimated Effort |
|---|------|------|------------------|
| 7 | CHANGELOG + v2.5.0 release + migration guide | [task-07-changelog-release.md](./tasks/task-07-changelog-release.md) | 0.5 day |

## Recommended Execution Order

See [EXECUTION_ORDER.md](./EXECUTION_ORDER.md). Task 1 is independent; Tasks 2
and 3 touch disjoint files and could land in parallel; Task 4 (CI) is a
prerequisite for the others to show green because `govulncheck` was red on
every branch; Task 5 is trivial; Task 6 gates the whole set; Task 7 cuts the
release.

## For AI Agents

See [AGENT_GUIDELINES.md](./AGENT_GUIDELINES.md). v1.3–v2.4 rules carry over;
this file lists only what changed for v2.5.

## Project Information

- **Repository:** `github.com/suryakencana007/espresso/v2`
- **Previous Version:** v2.4.0 (2026-07-05, "Ground Truth")
- **Target Version:** v2.5.0 (2026-11-15, "Headers & Recovery")
- **Module path:** unchanged (`/v2`; v2.5 is a minor bump)
- **Downstream Projects:** Barista, tamper, resi
- **Origin:** issues #76, #77 and #82; the CI failure on #79 surfaced #82.

## Existing Package Layout (post-v2.4)

v2.5 adds **no new packages or directories** in the module — it edits
`error.go` and `middleware/service/layer.go`, and adds one docs file. It does
add the `roadmaps/v2.5/` directory itself.

```
espresso/
├── error.go                    ← Task 1 adds WithHeader/WithHeaderAdd + applyErrorHeaders
├── error_test.go               ← Task 1 adds the error-header tests (incl. E2E via ServeHTTP)
├── .github/workflows/ci.yml    ← Task 4 bumps test + vulncheck jobs to Go 1.27
├── .gitignore                  ← Task 5 ignores snip-*
├── bench/                      ← Task 3 bumps sonic/loader in the second module
├── cmd/example/
├── docs/
│   ├── api/espresso.md         ← Task 1 documents the new builder methods
│   ├── error-handling.md       ← Task 1 documents error response headers
│   ├── migration-v1-to-v2.md
│   ├── migration-v2-to-v2.1.md
│   ├── migration-v2.3-to-v2.4.md
│   └── migration-v2.4-to-v2.5.md   ← new in v2.5 (Task 7)
├── extractor/
├── internal/
│   ├── errorenvelope/          ← unchanged; Content-Type stays writer-owned
│   └── validatehook/
├── middleware/
│   ├── http/
│   └── service/
│       ├── layer.go            ← Task 2 fixes the Open→HalfOpen transition re-check
│       └── layer_test.go       ← Task 2 hardens the probe-bound regression test
├── openapi/
├── pool/
├── tests/integration/
├── validator/
└── roadmaps/
    └── v2.5/                   ← new in v2.5 (retrospective back-fill)
```

## Global Conventions

Same as v2.4:

- Unit tests with minimum 80% coverage on touched lines.
- `cmd/example/` updated when user-facing behavior changes (no example change
  was needed in v2.5 — the error path is library-level).
- All public APIs have godoc comments with examples; behavior changes correct
  godoc in the same PR.
- `go test ./... -race` clean before a task is considered done.
- `golangci-lint run ./...` clean before a task is considered done.
- `govulncheck ./...` clean.
- Behavior changes are locked with a regression test shaped like the repro,
  not a paraphrase of the fix, and called out in the CHANGELOG
  `Changed`/`Fixed` section with before/after.
- The release cut is a single atomic commit (CHANGELOG + version chips +
  migration guide), tagged `v2.5.0` from the merge commit.
