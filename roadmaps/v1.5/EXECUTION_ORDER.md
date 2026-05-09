# Execution Order for v1.5.0

This document provides the recommended execution order for the v1.5 roadmap tasks. The schedule assumes one developer (human or AI agent) working full-time for approximately 1 week.

v1.5 is deliberately scoped tighter than v1.3 (3 weeks) or v1.4 (2 weeks): three additive items, no hardening, no breaking changes.

## Overview

```
Week 1 only:
  Day 1-2: Task 1 (cookies on JSON[T]) + Task 3 (ErrPreconditionFailed)
  Day 3-4: Task 2 (RawBodyWithHeaders[H])
  Day 5:   Task 4 (CHANGELOG & release)
```

## Day 1 (Monday) — Easy Wins

### Task 3 — `ErrPreconditionFailed`

Smallest task on the roadmap. Do it first to clear it off the board.

- Step 3.1: Add the constructor next to `ErrConflict` / `ErrUnprocessableEntity` in `error.go`.
- Step 3.2: One unit test asserting status code 412 and the standard JSON envelope.
- Step 3.3: Update README "Error Handling" constructor list and `docs/error-handling.md`.

Open Task 3 PR by end of day.

### Task 1 — Cookies on `JSON[T]` (start)

- Read `response.go`'s existing `JSON[T].WriteResponse` carefully. Note the buffer-pool path.
- Step 1.1: Add `Cookies []*http.Cookie` field to `JSON[T]`.
- Step 1.2: Modify `WriteResponse` to call `http.SetCookie(w, c)` for each cookie before writing headers via `WriteHeader`. Order matters — `Set-Cookie` must be in the response head.

## Day 2 (Tuesday) — Finish Task 1

- Step 1.3: Backward-compat regression test — zero-value `Cookies` produces byte-identical output to v1.4.
- Step 1.4: Cookie-set test — one cookie, multiple cookies, edge cases (empty path, expired).
- Step 1.5: Update `docs/api/response.md` with the cookie pattern.
- Step 1.6: Update `cmd/example/` (refresh-token-style example) if it doesn't already demonstrate cookies.

Open Task 1 PR.

## Day 3 (Wednesday) — Task 2 (start)

`RawBodyWithHeaders[H]` is the largest task by code volume but still self-contained.

- Read `extractor/extractor.go`'s `Path[T]`, `Header[T]`, `Cookie[T]` implementations end-to-end. They share a tag-extraction helper; the new extractor must use the same one.
- Step 2.1: Add `RawBodyWithHeaders[H any]` struct with `Body []byte` and `Headers H` fields.
- Step 2.2: Implement `Extract(r *http.Request) error`:
  1. Read body via `io.ReadAll(r.Body)` into `Body`.
  2. Populate `Headers` via the existing `extractStructTagsFromHeaders` helper.
  3. Both errors propagate as `*espresso.Error` (`ErrBadRequest`).
- Step 2.3: Implement `Reset()` for `sync.Pool` reuse — zero `Body`, zero `Headers`.

## Day 4 (Thursday) — Finish Task 2

- Step 2.4: Tests: happy path (raw body + one header + multiple headers), error path (oversized body if a limit is added; missing required header per `header:"X-Foo,required"`).
- Step 2.5: Webhook-style example in `cmd/example/` — verifies the W-08 / F-06 "raw body + provider header" pattern works in <30 lines from the user's side.
- Step 2.6: Update `docs/api/extractor.md` with the `RawBodyWithHeaders[H]` section. Cross-link from the F-06 entry in `roadmaps/USAGE_ESPRESSO.md` once it's marked closed.

Open Task 2 PR.

## Day 5 (Friday) — Release

### Task 4 — CHANGELOG & v1.5.0 release

- Step 4.1: Promote `[Unreleased]` content to `[1.5.0] - 2026-MM-DD`.
- Step 4.2: Final quality gates:
  - `go test ./... -race`
  - `golangci-lint run ./...`
  - `cd bench && go test -bench . -benchmem -benchtime=3s -count=1` — verify no regression in JSON-response benchmarks (cookie field with `nil` zero value should be free).
- Step 4.3: Bump `package.json` version to `1.5.0`.
- Step 4.4: Tag `v1.5.0`, push, create GitHub release.
- Step 4.5: Update `roadmaps/USAGE_ESPRESSO.md` — mark F-05, F-06, F-07 as `(closed)` with a pointer to the v1.5 task that closed each.
- Step 4.6: Ping Barista downstream so they can plan their pin bump and retire the chart-internal workarounds (`httpx.JSONWithCookies[T]`, `webhookRequest`, `NewError(412, …)`).

**Deliverable:** v1.5.0 released. Three Barista workarounds retired upstream.

---

## Contingency Planning

### Must Not Slip (hard requirements for v1.5 release)
- Task 1 (cookies on `JSON[T]`) — F-05 has been open longest (since v0.3, 2026-05-02) and is the most-cited Barista friction item.
- Task 4 (release).

### Can Slip to v1.5.1 (nice to have, not blocking)
- Task 2 (`RawBodyWithHeaders[H]`) — Barista's existing 35-line custom extractor still works; the framework helper is a convenience.
- Task 3 (`ErrPreconditionFailed`) — `NewError(http.StatusPreconditionFailed, ...)` still works; the helper is for symmetry.

### Must Not Compromise (quality gates)
- `JSON[T]` zero-value behavior is byte-identical to v1.4 (regression test).
- `go test ./... -race` clean.
- `golangci-lint run ./...` clean.
- No regressions in `bench/` framework-comparison numbers.

## Parallel Work Opportunities

All three feature tasks (1, 2, 3) touch disjoint files:

- Task 1 → `response.go`, `response_test.go`, `docs/api/response.md`.
- Task 2 → `extractor/extractor.go`, `extractor/extractor_test.go`, `docs/api/extractor.md`.
- Task 3 → `error.go`, `error_test.go`, `docs/error-handling.md`, README.

If multiple agents work in parallel, they can run concurrently with zero merge conflicts. A single agent should still do them sequentially (per the daily schedule above) for review-cycle reasons.

## Notes

- v1.5 is deliberately small. If a task balloons in scope, that's the sign you've drifted into v2.0 territory — stop and re-scope.
- All three friction items have lived through v0.4, v0.5, v0.6 of Barista without becoming more painful. They are not emergencies. The release is justified by closing accumulated chart-internal workarounds, not by user-blocking bugs. Optimize accordingly: ship clean and small, no heroics.
- Time estimates assume the developer has read `roadmaps/USAGE_ESPRESSO.md` end-to-end before starting. Add 0.5 day for first-time readers.
