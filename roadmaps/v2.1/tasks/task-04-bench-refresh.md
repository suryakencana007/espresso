# Task 4: Bench Refresh vs Gin/Echo/Fiber on v2.x

**Priority:** 🔵 Verification
**Estimated Effort:** 0.5 day
**Dependencies:** Tasks 1-3 substantially complete (so the numbers reflect post-cleanup v2.1, not mid-cycle)


> **Status: ✅ Shipped 2026-05-12.** Delivered via #32.

## Context

The README's "Framework Comparison" tables were captured against v1.4 (when the `bench/` module landed). v2.0 added a bounded handler cache and the auto-validate hook, but neither is on the per-request hot path when unset (which is how the bench module runs). Numbers shouldn't have moved meaningfully — but "shouldn't have moved" is a hypothesis that's worth a measurement before we let users assume it.

This is purely informational. No code is changed in Espresso itself.

## Acceptance Criteria

- [x] Bench module re-run from a clean checkout against the current `main` (post-Task 1/2/3).
- [x] `README.md` "Framework Comparison" tables refreshed with the new ns/op, B/op, allocs/op values.
- [x] Caption notes the run was on v2.1.x (no behavioral changes from v1.4 on the bench paths, just a refresh of the published numbers).
- [x] If any framework's number moved by >10% in either direction, investigate and document the reason in `bench/README.md`.

## Technical Approach

### Step 4.1 — Run

From a clean checkout:

```bash
cd bench
go test -bench . -benchmem -benchtime=3s -count=3 -cpu=1 > /tmp/bench-v2.1.txt
```

`-cpu=1` pins to a single core for less variance. `-count=3` lets us spot outliers.

### Step 4.2 — Compare to Published Numbers

The README currently shows (Intel Core Ultra 7 155H, Windows 11, Go 1.23):

```
Static text — Espresso 595 ns/op, 1016 B/op, 10 allocs/op
JSON echo   — Espresso 979 ns/op, 1522 B/op, 19 allocs/op
Path param  — Espresso 1000 ns/op, 1273 B/op, 16 allocs/op
```

Capture the new numbers. Differences to look for:
- **±~5%**: noise; just refresh the table.
- **>10% slower**: regression — check whether v2.0's per-Router registry `withRouterRegistries` ctx wrap is the culprit (it's a single `context.WithValue` per request).
- **>10% faster**: improvement — note the cause (likely sonic/Go-version updates rather than Espresso changes).

### Step 4.3 — Update README

Replace the three tables with the new numbers. Keep the caption format:

```
**Static text `GET /ping → "pong"`**

| Framework | ns/op | B/op  | allocs/op |
|-----------|-------|-------|-----------|
| ...
```

Update the caption to note the run conditions:
- Hardware (whatever the runner is)
- OS, Go version, Espresso commit
- `-bench . -benchmem -benchtime=3s -cpu=1 -count=3`

### Step 4.4 — Methodology Doc

`bench/README.md` should not need changes unless the harness changed. Sanity-check it.

## Tests Required

This task IS measurement. The "tests" are:

- The bench module compiles and runs from a clean checkout.
- The numbers are reproducible to within run-to-run variance (~few ns/op).

## Breaking Changes

None.

## Definition of Done

- [x] Three updated tables in README with v2.1.x numbers.
- [x] Caption updated with run conditions.
- [x] If any number moved >10%, the reason is captured in `bench/README.md` or the PR description.
- [x] PR description includes the raw `go test -bench` output for traceability.
