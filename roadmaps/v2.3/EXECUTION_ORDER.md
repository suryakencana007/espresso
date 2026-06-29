# Execution Order for v2.3.0

> **Shipped 2026-06-29** — tag [`v2.3.0`](https://github.com/suryakencana007/espresso/releases/tag/v2.3.0). This records the planned order; actual execution matched it (task-01 ‖ 02 ‖ 04 ‖ 05 parallel, then 03, then 06, then the 07 release).

This document provides the recommended execution order for the v2.3 roadmap tasks. The schedule is sized for approximately one-and-a-half focused weeks. v2.3 is a correctness/quality cleanup pass — the headline is making the OpenAPI generator trustworthy — with one tiny additive API (security schemes) and two small cleanups (a docs type-reference sweep and a one-line WebSocket integration-test fix). The work is narrow but cross-file-sensitive in one spot (Task 3, which lands on files shared with Tasks 1 and 2).

## Overview (planned)

```
Day 1-3:  Task 1 (OpenAPI generation correctness)        ─┐
          Task 2 (OpenAPI security schemes)               │ PARALLEL — disjoint files
          Task 4 (docs type-reference sweep + compile)    │ (introspect.go/router_openapi.go ‖
          Task 5 (WS integration-test fix)               ─┘  openapi.go/options.go ‖ docs/ ‖ tests/integration/)
Day 4-5:  Task 3 (serving hardening + remove AutoRegister) — starts AFTER Tasks 1+2 land
                                                            (shares openapi.go with Task 2, router_openapi.go with Task 1)
Day 6:    Task 6 (verification) — once Tasks 1-5 are in
Day 7:    Task 7 (CHANGELOG & v2.3.0 release)
```

The shape is deliberately front-loaded: the two OpenAPI P0 tasks plus the two independent cleanups touch disjoint files and can be dispatched together, leaving the file-sharing task (the serving hardening + `AutoRegister` removal, which lands on both `openapi.go` and `router_openapi.go`) for after those files have settled, the verification sweep for after every fix is in, and the release for last.

## Week 1 — Backflush

The theme is the release tagline: clear the accumulated correctness/quality residue the v2.2 verify-and-scope pass surfaced and deferred. The emphasis is trustworthy generated artifacts — the OpenAPI spec must accurately reflect the routes; when generator output disagrees with reality, fix the generator and lock it with a spec-inspection test. Every day below ends at a PR; every PR rebases against the running `[Unreleased]` CHANGELOG section.

### Day 1-3 — `OpenAPI generation correctness` (Task 1) ‖ `OpenAPI security schemes` (Task 2) ‖ `Docs type-reference sweep` (Task 4) ‖ `WS integration-test fix` (Task 5)

These four run in **parallel** — their files are disjoint. Task 1 lives in `openapi/introspect.go` + `router_openapi.go`; Task 2 lives in `openapi/openapi.go` + `openapi/options.go`; Task 4 lives entirely under `docs/`; Task 5 is a one-line change in `tests/integration/`. No file overlap, so dispatch all four at once (isolated worktrees, as in v2.1's tasks 02/03/04 round).

**Task 1 — OpenAPI generation correctness (`openapi/introspect.go`, `router_openapi.go`)**

- Step 1.1: Regenerate a real spec and inspect it to re-confirm the five generation defects this task owns before changing anything — D2 (dead custom-extractor branch), D3 (status code always 0), D6 (`registerPath` swallows introspection errors), D8 (prefix-string extractor classification), D9 (`registerPath`/`RegisterHandler` drift on response schema). These were confirmed 2026-06-28; lock each with a spec-inspection assertion first.
- Step 1.2: D2 — fix the custom-`FromRequest` introspection interface at `introspect.go:50` from `interface{ Extract(r any) error }` to `Extract(*http.Request) error` so real extractors (`extractor/extractor.go`, `response.go:96`, `core.go`) are actually introspected.
- Step 1.3: D3 — make `extractStatusCode` (`introspect.go:240-259`) derive the real status from the response type (`JSON[T].StatusCode` default 200, `Status`, `Text`, …) instead of always returning 0, so a 201 POST documents as 201.
- Step 1.4: D8 — classify extractors robustly by reflecting against the actual extractor types (`extractor.PathExtractor[...]` etc. / a kind interface), not `strings.HasPrefix` on the type name (`introspect.go:135-167` `getExtractorKind`); the current scheme mis-classifies `Files...`/`Format...` user types with zero compile signal.
- Step 1.5: D9 + D6 — unify `registerPath` (`router_openapi.go:108-168`) and `RegisterHandler` (`264-306`) onto one shared helper so both attach the response-body schema (only `registerPath` does today, at `148-165`) and both surface/log the introspection error instead of silently dropping the route (`registerPath` swallows it at `109-112`; `RegisterHandler` already returns it).
- Step 1.6: Open the Task 1 PR. Note that every fix is locked by a spec-inspection test (generate → inspect → assert).

**Task 2 — OpenAPI security schemes (`openapi/openapi.go`, `openapi/options.go`)**

- Step 2.1: Re-confirm D4 — `components.securitySchemes` is allocated empty at `openapi.go:129` and never populated, while `options.go:45` `Security("bearerAuth")` sets `op.Security` referencing a scheme by name, producing a dangling reference that fails strict OpenAPI validation and breaks the Scalar/Swagger auth button. Lock it with a test asserting a `Security(...)` op currently dangles.
- Step 2.2: Add the security-scheme registration API (e.g. `AddSecurityScheme(name, scheme)`) and populate `components.securitySchemes`. This is the **one** small additive API in v2.3 — keep it minimal and additive only.
- Step 2.3: Add the regression test: register a scheme, attach `Security(name)` to an op, generate, and assert the `op.Security` reference resolves against a populated `components.securitySchemes`.
- Step 2.4: Run `go test ./... -race` clean. Open the Task 2 PR.

**Task 4 — docs type-reference sweep + snippet-compile check (`docs/`)**

- Step 4.1: Re-confirm the inventory — `espresso.Path/Query/Form/Header/XML[T]` are wrong in docs; those types live in the `extractor` package (`extractor.go:417-426` aliases) and are not re-exported by root. Replace `espresso.X` → `extractor.X` across the confirmed occurrences (`espresso.Path[` 7 files/10 occ; `espresso.Query[` 4 files/6 occ; `espresso.Form[` 1; `espresso.Header[` 1; `espresso.XML[` 2).
- Step 4.2: Do **not** rewrite correct root symbols — `JSON`, `Text`, `Status`, `Error`, `Err*`, `WS`, `SSEStream`, `Stream`, `StreamSimple`, `Event`, `Ristretto`/`Solo`/`Doppio`/`Lungo`, `Handler*`, `MustGetState`/`GetState`/`State`/`FromState`, `Validation`, `SetDefaultValidator`.
- Step 4.3: Add a docs-snippet-compile check — extract whole-program ```` ```go ```` fences from `docs/` and `go build` them (pragmatic subset: target self-contained `package main` example fences; fragment fences that reference undefined helpers cannot compile-all).
- Step 4.4: Open the Task 4 PR.

**Task 5 — fix WebSocket long-lived integration test (`tests/integration/`)**

- Step 5.1: Re-confirm the root cause is the test harness, not the framework — `TestLongLived_WS_StableConnection` (`longlived_test.go` ~154-169) dials then loops on `conn.Ping` and never calls `conn.Read`; `coder/websocket` has no background read pump, so the pong `Ping` waits on is never processed and `Ping` times out at exactly 8s. The espresso WS server is healthy (`websocket.go:150` `readLoop` drains and auto-replies pong).
- Step 5.2: Add `conn.CloseRead(ctx)` right after the `Dial` in `TestLongLived_WS_StableConnection` (the library-documented idiom for read-less clients). Also fix the latent same-issue in `TestLongLived_WS_100Concurrent` (it does not manifest today because it never pings, but it should read too for correctness). **No framework code change.**
- Step 5.3: After the fix, `go test -tags=integration ./tests/integration/...` must pass on this machine. Open the Task 5 PR.

### Day 4-5 — `OpenAPI serving hardening + remove AutoRegister stub` (Task 3)

Starts **after Tasks 1 and 2 land**. Task 3 touches `openapi/openapi.go` (shared with Task 2) and `router_openapi.go` (shared with Task 1), plus `openapi/scalar.go`. Serializing it behind the two P0 tasks avoids guaranteed conflicts on both shared files and lets it build on the just-unified register helper. This is why it is P1 and sequenced after the P0 work rather than parallel to it.

- Step 3.1: D1 — the spec-generation failure path emits `text/plain` via `http.Error` at `openapi.go:320` instead of the canonical JSON envelope. The `openapi` package does not depend on root espresso (cannot build an `*espresso.Error`), but it can import the stdlib-only `internal/errorenvelope` leaf (added in v2.2) to emit the canonical shape with no import cycle. Reuse that leaf — do not introduce a new package.
- Step 3.2: D5 — delete the no-op `AutoRegister` stub (`router_openapi.go:248-249`) and its misleading godoc (`234-247`) so the API stops promising what it does not do. Real auto-registration is future work; mention it as a possible future feature, not v2.3.
- Step 3.3: D7 — cache the marshaled spec bytes once. `Handler()` (`openapi.go:316-327`) re-marshals via `ToJSON()` → `json.MarshalIndent` on every request; the spec is immutable after route registration, so marshal once and serve the cached bytes.
- Step 3.4: D10 — pin the Scalar UI CDN script to a specific `@version` (`scalar.go:18` currently loads `@scalar/api-reference` unpinned from an external host) and document that offline/air-gapped users should self-host.
- Step 3.5: Run `go test ./... -race` clean. Open the Task 3 PR.

### Day 6 — `Verification` (Task 6)

Depends on Tasks 1, 2, 3, 4, and 5 being in. Lands once the substantive fixes have merged so it tests reality, not intent.

- Step 6.1: OpenAPI spec-correctness matrix — a table-driven test that generates a spec across the extractor/response/status permutations and asserts each output (status codes per D3, extractor classification per D8, custom-extractor params/body per D2, response schema on both register paths per D9, populated `securitySchemes` with resolving references per D4, JSON envelope on the failure path per D1).
- Step 6.2: Suites green — `go test ./... -race` clean, and `go test -tags=integration ./tests/integration/...` passes on this machine (Task 5).
- Step 6.3: Confirm the docs-snippet-compile check from Task 4 runs clean.
- Step 6.4: Open the Task 6 PR.

### Day 7 — `CHANGELOG & v2.3.0 release` (Task 7)

Last task. Depends on Tasks 1-6.

- Step 7.1: Promote `[Unreleased]` → `[2.3.0]` in `CHANGELOG.md` in a single atomic commit. The OpenAPI generation fixes (D2/D3/D6/D8/D9) and serving fixes (D1/D7/D10) go under **Fixed**; the `AutoRegister` removal (D5) under **Removed**; the security-scheme API (D4) under **Added**; the docs sweep (Task 4) and WS test fix (Task 5) under **Fixed**.
- Step 7.2: Bump version chips — `package.json` to `2.3.0` and `docs/.vitepress/config.ts` to `v2.3.0` — in the same atomic commit.
- Step 7.3: Final quality gates: `go test -race ./...`, `golangci-lint run ./...`, `go test -tags=integration ./tests/integration/...`, bench module spot-check.
- Step 7.4: Tag `v2.3.0` from the merge commit, push, `gh release create v2.3.0` with the `[2.3.0]` body.

## Contingency Planning

### Must Not Slip (hard requirements for the v2.3 release)

- Task 1 (OpenAPI generation correctness) — the generator emits quietly-wrong specs today (wrong status codes, mis-classified extractors, dropped routes, missing response schemas); making the generated artifact trustworthy is v2.3's whole thesis.
- Task 2 (OpenAPI security schemes) — the dangling `securitySchemes` reference fails strict validation and breaks the docs auth button; the additive registration API is the headline capability gap.
- Task 7 (release).

### Can Slip to v2.3.1 (next patch)

- Task 6 (verification) — the spec-correctness matrix hardens the fixes but does not change behavior. If the cycle runs hot, a thinner version can land with v2.3.0 and the full matrix follow in the patch. Prefer not to slip it, but it is the safest cut.
- Task 4 (docs type-reference sweep) — docs-only; wrong type references mislead readers but ship no code defect. The snippet-compile check, once added, will keep them honest going forward.

### Can Slip to v2.4 (next minor)

- Task 3 (serving hardening + `AutoRegister` removal) — it is P1 precisely because it depends on Tasks 1 and 2 settling the shared files first. If the `errorenvelope` reuse (D1) or the marshal-cache (D7) proves thornier than budgeted, the serving paths can stay as-is for one more minor without blocking the P0 generation correctness. The `text/plain` failure path and unpinned Scalar CDN have shipped since the OpenAPI feature landed; one more cycle is tolerable. Real auto-registration (the future-work replacement for the deleted `AutoRegister` stub) is explicitly out of scope for v2.3.

### Must Not Compromise (quality gates)

- `go test -race ./...` clean.
- `golangci-lint run ./...` clean.
- `go test -tags=integration ./tests/integration/...` passes on this machine (Task 5's whole point).
- Test coverage holds at or above the project minimum; every generator fix lands locked by a spec-inspection test (the v2.3 emphasis: when a generator output disagrees with reality, fix the generator and lock it).

## Parallel Work Opportunities

**Parallel batch 1** (no cross-file conflicts):
- Task 1 (`openapi/introspect.go`, `router_openapi.go`), Task 2 (`openapi/openapi.go`, `openapi/options.go`), Task 4 (`docs/`), and Task 5 (`tests/integration/`) touch disjoint files — dispatch all four together on Day 1 (isolated worktrees).

**Strictly serialized** (shared files):
- Task 3 shares `openapi.go` with Task 2 and `router_openapi.go` with Task 1, so it must land **after** both. It is not a parallel candidate — sequencing it avoids guaranteed conflicts on both files and lets it build on the unified register helper from Task 1.

**Downstream** (after the fixes are in):
- Task 6 (verification) is downstream of Tasks 1-5 by definition; Task 7 (release) is downstream of everything.

## Notes

- All v2.3 work happens on the `/v2` module path; no module-bump considerations.
- v2.3 adds **no** new packages. It edits `openapi/`, `router_openapi.go`, `docs/`, and `tests/integration/`, and **reuses** the existing stdlib-only `internal/errorenvelope` leaf (added in v2.2) for the OpenAPI failure path — that is the deliberate cycle-safe path, not new feature surface.
- Resist scope creep. v2.3 is a cleanup pass: fix the ten confirmed OpenAPI defects, add the one small security-scheme API, sweep the docs, fix the one WS test, lock everything with tests, ship. If `openapi/`, `router_openapi.go`, or the docs reveal adjacent cleanup, capture it in a follow-up issue rather than bundling.
- The CHANGELOG `[Unreleased]` section will take conflicts on each PR after the first, as in v2.2 — these are one-file, one-section rebases and resolve in seconds.
- Every change lands via a feature branch + PR. CLI merge is blocked by branch protection; the maintainer merges via the GitHub UI. Use Conventional Commits throughout.
- Task 2 changes the generated spec shape (populated `securitySchemes`) and Task 3 changes the failure-path content type (`text/plain` → JSON). Neither is a runtime-behavior change for application handlers, but the release note in Task 7 should mention them so downstream consumers — Barista in particular — regenerate clients against the corrected spec after upgrading.
