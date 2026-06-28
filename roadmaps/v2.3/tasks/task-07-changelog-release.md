# Task 7: CHANGELOG & v2.3.0 Release

**Priority:** 📦 Meta
**Estimated Effort:** 0.5 day
**Dependencies:** Tasks 1, 2, 3, 4, 5, 6 must be merged

## Context

Final gate before tagging v2.3.0. Promotes `[Unreleased]` to `[2.3.0] - <date>`, bumps the version chip + `package.json`, runs the full quality-gate set, tags, and publishes the GitHub release. Single atomic commit (mirrors v2.0 task-07 / v2.1 task-06 / v2.2 task-05).

v2.3 is the "Backflush" release — a correctness/quality cleanup that clears the OpenAPI debt the v2.2 analysis surfaced and deferred. So the CHANGELOG leans on **Fixed** more than **Added**: Tasks 1, 3, 4, and 5 fix observable wrongness (specs that lie, docs that don't compile, an integration test that hangs), Task 2 introduces the one tiny additive API (`AddSecurityScheme`), and Task 3 removes the misleading `AutoRegister` no-op stub. Because secured specs now *require* registering a security scheme, and because callers may have a dead `AutoRegister` call to delete, this release ships a short **upgrade note**.

## Acceptance Criteria

- [ ] `CHANGELOG.md` has a complete `[2.3.0] - 2026-MM-DD` section covering Tasks 1, 2, 3, 4, 5 (with Task 6 noted as a verification/testing entry).
- [ ] The OpenAPI generation/serving correctness fixes (Tasks 1, 3) appear under `Fixed` with concrete before/after.
- [ ] The `AddSecurityScheme` API (Task 2) appears under `Added`.
- [ ] The `AutoRegister` no-op stub removal (Task 3) appears under `Removed`.
- [ ] The docs type-reference sweep (Task 4) and the WebSocket integration-test fix (Task 5) appear under `Fixed`.
- [ ] A short "Upgrade from v2.2" note documents the `AddSecurityScheme` requirement for secured specs and the `AutoRegister` removal (callers of the no-op simply delete the call).
- [ ] `package.json` version bumped to `2.3.0`.
- [ ] `docs/.vitepress/config.ts` version chip updated to `v2.3.0`.
- [ ] `[Unreleased]` retained empty for v2.3.x / v2.4 work.
- [ ] `go test ./... -race` clean.
- [ ] `golangci-lint run ./...` clean.
- [ ] `go test -tags=integration ./tests/integration/...` clean (Task 5 fix landed).
- [ ] `bench/` module compiles cleanly.
- [ ] Git tag `v2.3.0` created and pushed.
- [ ] GitHub release published with the `[2.3.0]` body.

## Technical Approach

### Step 7.1 — Verify All Tasks Complete

Walk the v2.3 roadmap. Every task file's Acceptance Criteria boxes are ticked.

- [ ] Task 1: OpenAPI generation correctness
- [ ] Task 2: OpenAPI security schemes
- [ ] Task 3: OpenAPI serving hardening + remove `AutoRegister` stub
- [ ] Task 4: Docs type-reference sweep + snippet-compile check
- [ ] Task 5: Fix WebSocket long-lived integration test
- [ ] Task 6: OpenAPI spec-correctness matrix + suites green

### Step 7.2 — CHANGELOG

Promote `[Unreleased]` content into `[2.3.0] - 2026-MM-DD`. Expected shape:

```markdown
## [Unreleased]

## [2.3.0] - 2026-MM-DD

A correctness/quality release ("Backflush"): clears the accumulated OpenAPI
debt the v2.2 analysis surfaced, so the generated spec now accurately reflects
the registered routes. One tiny additive API (`AddSecurityScheme`) for security
schemes; otherwise no new feature surface. Plus a docs type-reference sweep and
a WebSocket integration-test fix. See the upgrade note below before bumping.

### Added

- **`AddSecurityScheme(name, scheme)` for OpenAPI security** (#NN, Task 2).
  `components.securitySchemes` was allocated empty and never populated, so
  `Security("bearerAuth")` produced a **dangling** reference that failed strict
  OpenAPI validation and broke the Scalar/Swagger "Authorize" button. Register
  a scheme by name and the operation's `security` reference now resolves.

### Fixed

- **OpenAPI generation now reflects reality** (#NN, Task 1). Several
  generator defects made emitted specs quietly wrong:
  - Custom `FromRequest` extractors are introspected again — the introspection
    interface looked for `Extract(r any) error`, but every real extractor
    implements `Extract(*http.Request) error`, so the branch was dead (no
    params, no `requestBody`).
  - Response status codes are derived from the response type instead of always
    being `200` — a `201` POST documents as `201`, not `200`.
  - Extractor classification matches the actual extractor types rather than a
    type-name **prefix** string-match, so renaming an extractor (or a user type
    named e.g. `Files…`/`Format…`) no longer silently mis-classifies.
  - `registerPath` no longer silently swallows introspection errors and drops
    the route from the spec; the error is surfaced/logged.
  - `registerPath` and `RegisterHandler` are unified onto one shared helper, so
    both paths attach the response-body schema (previously only `registerPath`
    did; `RegisterHandler` emitted a bare `200: {description: Success}`).
- **OpenAPI serving hardening** (#NN, Task 3). The spec-generation failure path
  emits the canonical JSON error envelope (via the stdlib-only
  `internal/errorenvelope` leaf) instead of `text/plain` `http.Error`. The
  immutable marshaled spec is cached once after registration instead of being
  re-marshaled on every request. The Scalar UI CDN URL is pinned to a specific
  `@version` (offline/air-gapped users should self-host).
- **Docs type references corrected** (#NN, Task 4). `espresso.Path/Query/Form/
  Header/XML[T]` were documented under the root package, but those extractor
  types live in the `extractor` package and are not re-exported by root. All
  occurrences are rewritten to `extractor.X`. A docs-snippet-compile check now
  builds the self-contained `go` example fences in `docs/`.
- **WebSocket long-lived integration test no longer hangs** (#NN, Task 5).
  `TestLongLived_WS_StableConnection` dialed and pinged without ever reading;
  `coder/websocket` only processes pong frames inside the read path, so `Ping`
  timed out at 8s. Adding `conn.CloseRead(ctx)` after the dial (the library
  idiom for read-less clients) lets pongs flow. No framework code changed; the
  espresso WS server was always healthy.

### Removed

- **`AutoRegister` no-op stub removed** (#NN, Task 3). `Router.AutoRegister`
  was an empty no-op despite godoc promising it registered all routes. The
  method and its misleading godoc are deleted so the API stops lying; real
  auto-registration is possible future work.

### Internal

- **OpenAPI spec-correctness test matrix** (#NN, Task 6) generates real specs
  and inspects them — locking status codes, security-scheme resolution,
  response-body schemas, and custom-extractor introspection against regression.
```

### Step 7.3 — Bumps

```
package.json:                        "2.2.0" → "2.3.0"
docs/.vitepress/config.ts version:   "v2.2.0" → "v2.3.0"
```

(Grep the repo for other places the version string is referenced and bump them in the same commit.)

### Step 7.4 — Final Quality Gates

```bash
go test ./... -race
golangci-lint run ./...
go test -tags=integration ./tests/integration/...
cd bench && go test -bench . -benchmem -benchtime=3s -count=1 && cd ..
```

All pass before tagging. The integration suite is a hard gate this release (Task 5 made it green on this machine).

### Step 7.5 — Tag and Release

Single atomic release commit (CHANGELOG `[Unreleased]` → `[2.3.0]` + version chips in `package.json` and `docs/.vitepress/config.ts`), merged via PR (CLI merge is blocked by branch protection — the user merges via the UI). Tag from the merge commit.

```bash
git tag v2.3.0           # from the merge commit
git push origin v2.3.0

gh release create v2.3.0 \
    --title "v2.3.0 — Backflush" \
    --notes-file <(...)   # paste the [2.3.0] CHANGELOG body
```

GitHub release body: the `[2.3.0]` CHANGELOG section (including the upgrade note) + a "Full Changelog" compare link from `v2.2.0`.

### Step 7.6 — Post-Release Cleanup

- Mark the v2.3 roadmap retrospective (pre-check boxes, add the ship date, past-tense `EXECUTION_ORDER.md`) — mirror what was done for v2.0 (#28), v2.1 (#36), and v2.2 (#45).
- Update `SESSION_STATE.md`: move the confirmed OpenAPI/docs/WS findings out of "v2.3 candidates" into the shipped log; note any items that slipped.
- Smoke test: `go get github.com/suryakencana007/espresso/v2@v2.3.0` in a throwaway project; generate an OpenAPI spec for a `201` POST with a secured route and confirm the spec reports `201`, resolves the security scheme, and includes the response-body schema.
- Notify Barista — the OpenAPI correctness fixes are the headline; if they consumed the (wrong) generated spec or relied on `AutoRegister`, they need the upgrade note before bumping.

## Tests Required

The full test suite (Tasks 1-6) plus the integration suite, plus the `bench/` module spot-check. No new tests are authored here.

## Definition of Done

- [ ] `CHANGELOG.md` `[2.3.0]` section finalized with the ship date.
- [ ] `package.json` version bumped to `2.3.0`.
- [ ] Docs version chip updated to `v2.3.0`.
- [ ] `[Unreleased]` left empty.
- [ ] Git tag `v2.3.0` pushed from the merge commit.
- [ ] GitHub release published with the `[2.3.0]` body + upgrade note.
- [ ] `SESSION_STATE.md` updated to reflect the v2.3.0 ship.
- [ ] Barista pinged with the OpenAPI correctness / `AddSecurityScheme` / `AutoRegister`-removal upgrade note.
