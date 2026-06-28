# Task 5: Fix WebSocket Long-Lived Integration Test

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 0.5 day
**Dependencies:** None

## Context

`TestLongLived_WS_StableConnection` (`tests/integration/longlived_test.go` ~154-169)
fails on this machine, and the failure has been root-caused to the **test harness**,
not the framework. This is a test fix; no `espresso` production code changes.

The mechanics, confirmed by reproducing the failure on 2026-06-28:

- The test dials the WebSocket, then enters a `for`/`select` loop whose only
  client-side activity is a periodic `conn.Ping(pingCtx)` (`longlived_test.go`
  ~160-168). It **never** calls `conn.Read`, `conn.Reader`, or `conn.CloseRead`.
- `coder/websocket` has **no background read pump**. Incoming frames — including
  the **pong** that `Ping` blocks on — are only processed inside the read path
  (`Conn.readLoop`, reached via `Read`/`Reader`/`CloseRead`). With no reader
  running on the client, the server's pong is never drained, so `Ping` waits out
  its deadline and fails at exactly the 3s ping timeout (the loop's
  `context.WithTimeout(ctx, 3*time.Second)`), surfacing as
  `t.Errorf("ping failed at ...")`.
- The espresso WebSocket **server is healthy**: `websocket.go:150`'s `readLoop`
  drains the connection and auto-replies pong. The defect is entirely on the
  read-less client in the test.

This was proven by three experiments, each isolating the cause:

1. Running a background `conn.Read` goroutine on the client → test **PASSES**
   (the read path drains the pong).
2. Widening the ping deadline to 15s → still **fails** (the pong never arrives —
   rules out latency or platform slowness as the cause).
3. Calling `conn.CloseRead(ctx)` immediately after `Dial` → test **PASSES**.

`CloseRead` is the library-documented idiom for clients that never read
application messages: it starts a read loop in the background (so control frames
like pong are processed) while discarding any data frames. That is exactly the
shape of these two tests, whose clients only ping / hold the connection open.

The latent twin is `TestLongLived_WS_100Concurrent` (`longlived_test.go`
~261-324). It does **not** ping, so it does not manifest the failure today — but
its clients are equally read-less (`<-ctx.Done()` only). It should read for the
same correctness reason, so the fix is applied to both for consistency and to
keep the pattern from being copied back in.

## Acceptance Criteria

- [ ] `TestLongLived_WS_StableConnection` calls `conn.CloseRead(ctx)` immediately after a successful `Dial` (before the ping loop), using the dial context.
- [ ] `TestLongLived_WS_100Concurrent` applies the same `conn.CloseRead(ctx)` after each goroutine's successful `Dial`, before it blocks on `<-ctx.Done()`.
- [ ] No `espresso` production code is modified — the diff is confined to `tests/integration/longlived_test.go`.
- [ ] `go test -tags=integration ./tests/integration/...` passes on this machine (default 30s duration; not flaky across repeated runs).
- [ ] The ping loop's behavior is otherwise unchanged (same interval, same deadline) — the only change is making the client drain control frames.

## Technical Approach

### Step 5.1 — Reproduce

Confirm the baseline failure before touching anything:

```bash
go test -tags=integration ./tests/integration/... -run TestLongLived_WS_StableConnection
```

Today this fails with `ping failed at ~3s: ...` (the pong deadline). Keep this
command as the before/after check.

### Step 5.2 — Fix `TestLongLived_WS_StableConnection`

After the successful `Dial` and the existing `resp.Body` close
(`longlived_test.go` ~141-148), add the read-drain idiom before the ping loop:

```go
conn, resp, err := websocket.Dial(ctx, wsURL, nil)
if err != nil {
    t.Fatalf("dial failed: %v", err)
}
if resp != nil && resp.Body != nil {
    _ = resp.Body.Close()
}
conn.CloseRead(ctx) // start a background read loop so pong frames are processed
defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
```

`CloseRead` returns a derived context (which this test does not need); calling it
for its side effect — a background reader that processes control frames — is the
documented use here. The rest of the ping loop stays exactly as-is.

### Step 5.3 — Fix the latent twin `TestLongLived_WS_100Concurrent`

In each client goroutine (`longlived_test.go` ~294-304), add the same
`conn.CloseRead(ctx)` after the `Dial`/`resp.Body` close and before `<-ctx.Done()`:

```go
conn, resp, err := websocket.Dial(ctx, wsURL, nil)
if err != nil {
    errorCount.Add(1)
    return
}
if resp != nil && resp.Body != nil {
    _ = resp.Body.Close()
}
conn.CloseRead(ctx)
defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

<-ctx.Done()
```

This does not change the test's pass/fail today, but it removes the same
read-less footgun so the pattern is consistent across both WS tests.

### Step 5.4 — Verify

Run the full integration suite at the default duration and confirm green and
stable:

```bash
go test -tags=integration ./tests/integration/...
```

Do not change `websocket.go` or any other framework file — the server side is
already correct.

## Tests Required

- No new test functions; this task **fixes** the two existing WS integration tests.
- `TestLongLived_WS_StableConnection` passes at the default 30s duration after adding `CloseRead`.
- `TestLongLived_WS_100Concurrent` continues to pass with the `CloseRead` drain in place.
- Re-run `go test -tags=integration ./tests/integration/...` to confirm the SSE tests in the same file are unaffected.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -tags=integration ./tests/integration/...` passes on this machine, non-flaky across at least two consecutive runs.
- [ ] The diff touches **only** `tests/integration/longlived_test.go`; `websocket.go` (and the rest of the framework) is **unchanged** — the PR description states this explicitly and notes the server-side `readLoop` (`websocket.go:150`) was already correct.
- [ ] CHANGELOG `[Unreleased]` entry under `Fixed` records the integration-test correction (read-less WS client added `CloseRead`), explicitly noting it is a test-harness fix, not a behavior change.
- [ ] `golangci-lint run ./...` clean (the integration package builds under the `integration` build tag).
