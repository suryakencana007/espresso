# Task 5: CHANGELOG & v2.2.0 Release

**Priority:** 📦 Meta
**Estimated Effort:** 0.5 day
**Dependencies:** Tasks 1, 2, 3, 4 must be merged

> **Status: ✅ Shipped 2026-06-28.** Delivered via #45 — tagged v2.2.0; docs synced in #46.

## Context

Final gate before tagging v2.2.0. Promotes `[Unreleased]` to `[2.2.0] - <date>`, bumps the version chip + `package.json`, runs the full quality-gate set, tags, and publishes the GitHub release. Single atomic commit (mirrors v2.0 task-07 / v2.1 task-06).

v2.2 is a correctness pass, so the CHANGELOG leans on **Changed** more than **Added**: Tasks 2 and 3 alter observable HTTP behavior (status codes, content type) and Task 1 either adds a capability (approach A) or fixes a latent defect (approach B). Because callers — Barista in particular — may key off the old behavior, this release ships a short **upgrade note** for the status-code and content-type shifts.

## Acceptance Criteria

- [x] `CHANGELOG.md` has a complete `[2.2.0] - 2026-MM-DD` section covering Tasks 1, 2, 3 (with Task 4 noted as a verification/testing entry).
- [x] The status-code mapping (Task 2) and the error-envelope unification (Task 3) appear under `Changed` with explicit before/after.
- [x] The two-extractor fix (Task 1) appears under `Added` if approach A shipped, or `Fixed` if approach B shipped.
- [x] A short "Upgrade from v2.1" note documents the status-code change (service-layer validation/circuit-breaker/timeout now 400/503/503 instead of 500) and the auth/rate-limit `text/plain` → JSON change.
- [x] `package.json` version bumped to `2.2.0`.
- [x] `docs/.vitepress/config.ts` version chip updated to `v2.2.0`.
- [x] `[Unreleased]` retained empty for v2.2.x / v2.3 work.
- [x] `go test ./... -race` clean.
- [x] `golangci-lint run ./...` clean.
- [x] `bench/` module compiles cleanly.
- [x] Git tag `v2.2.0` created and pushed.
- [x] GitHub release published with the `[2.2.0]` body.

## Technical Approach

### Step 5.1 — Verify All Tasks Complete

Walk the v2.2 roadmap. Every task file's Acceptance Criteria boxes are ticked.

- [x] Task 1: Reflection-path two-extractor handlers
- [x] Task 2: Service-layer error → HTTP status mapping
- [x] Task 3: Structured-JSON envelope on every error path
- [x] Task 4: Status-code matrix + signature + doc/code consistency tests

### Step 5.2 — CHANGELOG

Promote `[Unreleased]` content into `[2.2.0] - 2026-MM-DD`. Expected shape (adjust the Task 1 entry to the chosen approach):

```markdown
## [Unreleased]

## [2.2.0] - 2026-MM-DD

A correctness release ("Dial It In"): makes Espresso's behavior match its
documented/expected contract. No new feature surface. Service-layer errors
now map to the right HTTP status, every error path emits the same structured
JSON envelope, and the reflection dispatch path no longer silently accepts a
signature it can't serve. See the upgrade note below before bumping.

### Changed

- **Service-layer errors map to their semantic HTTP status** (#NN, Task 2).
  Errors surfaced through service layers previously collapsed to `500`.
  Now: `ValidationLayer` → `400 VALIDATION_ERROR` (with field detail
  preserved), open `CircuitBreaker` → `503 SERVICE_UNAVAILABLE`, and a
  `Timeout` (`context.DeadlineExceeded`) → `503 SERVICE_UNAVAILABLE`.
  Handler-returned `*espresso.Error` values and the unknown-error → `500`
  fallback are unchanged. Auto-validate-on-extract was already correct (`400`).
- **Auth and rate-limit responses use the canonical JSON envelope** (#NN,
  Task 3). `JWT`/`BasicAuth`/`APIKey` 401s and the rate limiter's 429
  previously emitted `text/plain`; they now emit
  `{"error":{"code","message","details","request_id"}}` like every other
  error path. The panic (500) body from `RecoverMiddleware` is now
  envelope-consistent (`details` handling matches `writeHandlerError`).

### Fixed
<!-- if Task 1 shipped approach B -->
- **Two-extractor reflection handlers fail fast at registration** (#NN,
  Task 1). `func(ctx, *Req1, *Req2) (T, error)` registered via
  `router.Get/Post/Handle` used to register silently and then panic
  per-request (`"espresso: invalid handler argument - this is a bug"` →
  500). It now panics at **registration** with an actionable message
  pointing to `HandlerCtxReq1Req2Err` / `Lungo`. The request-time panic is
  unreachable.

### Added
<!-- if Task 1 shipped approach A instead -->
- **Reflection path supports two extractors** (#NN, Task 1).
  `func(ctx, *Req1, *Req2) (T, error)` registered via `router.Get/Post/Handle`
  now extracts both requests, matching the typed `HandlerCtxReq1Req2Err` /
  `Lungo` path.

### Internal
- **Status/signature/doc-consistency test matrices** (#NN, Task 4) lock the
  Task 1-3 behavior and guard against re-introducing removed symbols or false
  signature claims.

### Upgrade from v2.1

- If your client keys off **HTTP 500** to detect validation / open-circuit /
  timeout failures behind service layers, switch to `400` / `503`
  respectively. The JSON envelope's `error.code` (`VALIDATION_ERROR`,
  `SERVICE_UNAVAILABLE`) is the stable discriminator.
- If anything parses the **`text/plain` body** of a `401` or `429` (e.g. the
  literal strings `"Unauthorized"` / `"Too Many Requests"`), switch to parsing
  the JSON envelope. Content type is now `application/json`.
- Two-extractor handlers must use `HandlerCtxReq1Req2Err` / `Lungo` (always
  the only working path); a reflection-path registration now fails loudly
  [approach B] / now works [approach A].
```

### Step 5.3 — Bumps

```
package.json:                        "2.1.0" → "2.2.0"
docs/.vitepress/config.ts version:   "v2.1.0" → "v2.2.0"
```

(Grep the repo for other places the version string is referenced and bump them in the same commit.)

### Step 5.4 — Final Quality Gates

```bash
go test ./... -race
golangci-lint run ./...
go test -tags=integration ./tests/integration/...
cd bench && go test -bench . -benchmem -benchtime=3s -count=1 && cd ..
```

All pass before tagging.

### Step 5.5 — Tag and Release

```bash
git tag v2.2.0
git push origin v2.2.0

gh release create v2.2.0 \
    --title "v2.2.0 — Dial It In" \
    --notes-file <(...)   # paste the [2.2.0] CHANGELOG body
```

GitHub release body: the `[2.2.0]` CHANGELOG section (including the upgrade note) + a "Full Changelog" compare link from `v2.1.0`.

### Step 5.6 — Post-Release Cleanup

- Mark the v2.2 roadmap retrospective (pre-check boxes, add the ship date, past-tense `EXECUTION_ORDER.md`) — mirror what was done for v2.0 (#28) and v2.1 (#36).
- Update `SESSION_STATE.md`: move the two confirmed findings out of "v2.2 candidates" into the shipped log; note any items that slipped (e.g. Task 3 to v2.3 if it did).
- Smoke test: `go get github.com/suryakencana007/espresso/v2@v2.2.0` in a throwaway project; confirm a service-layer timeout returns 503 and an auth failure returns JSON.
- Notify Barista — the status-code and content-type changes are the headline; they may need to adjust status-based handling and any text/plain parsing before upgrading.

## Tests Required

The full test suite (Tasks 1-4), plus the `bench/` module spot-check. No new tests are authored here.

## Definition of Done

- [x] `CHANGELOG.md` `[2.2.0]` section finalized with the ship date.
- [x] `package.json` version bumped to `2.2.0`.
- [x] Docs version chip updated to `v2.2.0`.
- [x] `[Unreleased]` left empty.
- [x] Git tag `v2.2.0` pushed.
- [x] GitHub release published with the `[2.2.0]` body + upgrade note.
- [x] `SESSION_STATE.md` updated to reflect the v2.2.0 ship.
- [x] Barista pinged with the status-code / content-type upgrade note.
