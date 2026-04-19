# Task 5: Long-Lived Connection Stress Test

**Priority:** 🔵 Verification
**Estimated Effort:** 2 days
**Dependencies:** Tasks 1 (WebSocket) and 2 (SSE) complete

## Context

Barista will run SSE log streams that may stay open for hours (continuous log tailing) and WebSocket terminals that may be idle for long periods between user interactions. We need to verify that Espresso handles these cases correctly:

- No memory leaks over time
- No unexpected timeouts or disconnects
- Goroutine count stays stable
- Performance doesn't degrade under concurrent long-lived connections

This task is **pure verification** — no new features. If tests reveal bugs, file them as separate issues to fix in v1.3 or later patches.

## Acceptance Criteria

- [ ] SSE stream runs continuously for at least 1 hour without errors
- [ ] WebSocket connection idles for at least 1 hour without disconnect
- [ ] 100 concurrent SSE streams run for at least 10 minutes without resource exhaustion
- [ ] 100 concurrent WebSocket connections run for at least 10 minutes without issues
- [ ] Memory profiling shows no growth beyond 10% over a 1-hour run
- [ ] Goroutine count returns to baseline after connections close
- [ ] Results documented in `docs/performance.md`

## Technical Approach

### Step 5.1: Integration Test Setup

Create a new directory for integration tests:

```
tests/
├── integration/
│   ├── longlived_test.go
│   ├── helpers_test.go
│   └── README.md
```

Integration tests use the build tag `integration` so they don't run in default `go test` runs:

```go
//go:build integration
```

They can be run with:

```bash
go test -tags=integration -v -timeout=2h ./tests/integration/...
```

### Step 5.2: Long-Running SSE Test

Create `tests/integration/longlived_test.go`:

```go
//go:build integration

package integration

import (
    "bufio"
    "context"
    "net/http"
    "net/http/httptest"
    "runtime"
    "sync"
    "testing"
    "time"

    "github.com/suryakencana007/espresso"
)

// TestLongLived_SSE_1Hour verifies that a single SSE stream can run for
// 1 hour without errors or memory growth.
func TestLongLived_SSE_1Hour(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping long-lived test in short mode")
    }

    const duration = 1 * time.Hour
    const eventInterval = 100 * time.Millisecond

    router := espresso.Portafilter().
        Get("/stream", espresso.StreamSimple(
            func(ctx context.Context, s *espresso.SSEStream) error {
                ticker := time.NewTicker(eventInterval)
                defer ticker.Stop()

                count := 0
                for {
                    select {
                    case <-ctx.Done():
                        return nil
                    case <-ticker.C:
                        count++
                        if err := s.SendData("tick " + strconv.Itoa(count)); err != nil {
                            return err
                        }
                    }
                }
            }))

    server := httptest.NewServer(router.HTTPHandler())
    defer server.Close()

    // Capture baseline memory and goroutines
    var baselineMem runtime.MemStats
    runtime.GC()
    runtime.ReadMemStats(&baselineMem)
    baselineGoroutines := runtime.NumGoroutine()

    // Start SSE client
    ctx, cancel := context.WithTimeout(context.Background(), duration)
    defer cancel()

    req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("connect failed: %v", err)
    }
    defer resp.Body.Close()

    // Consume events until duration elapses
    eventCount := 0
    scanner := bufio.NewScanner(resp.Body)
    start := time.Now()
    lastLog := start

    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "data:") {
            eventCount++
        }

        if time.Since(lastLog) > 5*time.Minute {
            lastLog = time.Now()
            elapsed := time.Since(start)
            t.Logf("elapsed=%v events=%d goroutines=%d",
                elapsed.Round(time.Second), eventCount, runtime.NumGoroutine())
        }

        if time.Since(start) >= duration {
            break
        }
    }

    // Verify: events received at expected rate (±10%)
    expectedEvents := int(duration / eventInterval)
    tolerance := expectedEvents / 10
    if eventCount < expectedEvents-tolerance {
        t.Errorf("expected ~%d events, got %d", expectedEvents, eventCount)
    }

    // Verify: no unexpected errors
    if err := scanner.Err(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("scanner error: %v", err)
    }

    // Verify: memory growth is bounded
    runtime.GC()
    var finalMem runtime.MemStats
    runtime.ReadMemStats(&finalMem)

    growthMB := float64(finalMem.Alloc-baselineMem.Alloc) / 1024 / 1024
    t.Logf("memory growth: %.2f MB", growthMB)
    if growthMB > 50 {
        t.Errorf("memory grew %.2f MB, expected < 50 MB", growthMB)
    }

    // Give time for cleanup
    time.Sleep(1 * time.Second)
    finalGoroutines := runtime.NumGoroutine()
    if finalGoroutines > baselineGoroutines+5 {
        t.Errorf("goroutine leak: baseline=%d, final=%d",
            baselineGoroutines, finalGoroutines)
    }
}

// TestLongLived_WS_1Hour verifies a WebSocket connection stays open for 1 hour.
func TestLongLived_WS_1Hour(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping long-lived test in short mode")
    }

    const duration = 1 * time.Hour
    const pingInterval = 30 * time.Second

    router := espresso.Portafilter().
        Get("/ws", espresso.WebSocketSimple(
            func(ctx context.Context, ws *espresso.WS) error {
                <-ctx.Done()
                return nil
            },
            espresso.WithPingInterval(pingInterval),
        ))

    server := httptest.NewServer(router.HTTPHandler())
    defer server.Close()

    wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

    ctx, cancel := context.WithTimeout(context.Background(), duration)
    defer cancel()

    conn, _, err := websocket.Dial(ctx, wsURL, nil)
    if err != nil {
        t.Fatalf("dial failed: %v", err)
    }
    defer conn.Close(websocket.StatusNormalClosure, "")

    // Wait for duration with periodic health checks
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    start := time.Now()
    for {
        select {
        case <-ctx.Done():
            if time.Since(start) < duration-time.Second {
                t.Errorf("context ended early: %v", time.Since(start))
            }
            return
        case <-ticker.C:
            t.Logf("still alive at %v, goroutines=%d",
                time.Since(start).Round(time.Second), runtime.NumGoroutine())

            // Send a ping to verify the connection is healthy
            pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
            err := conn.Ping(pingCtx)
            pingCancel()
            if err != nil {
                t.Errorf("ping failed at %v: %v", time.Since(start), err)
                return
            }
        }
    }
}
```

### Step 5.3: Concurrent Load Test

Add to `tests/integration/longlived_test.go`:

```go
// TestLongLived_SSE_100Concurrent runs 100 concurrent SSE streams for
// 10 minutes and verifies no resource exhaustion.
func TestLongLived_SSE_100Concurrent(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping concurrent test in short mode")
    }

    const numClients = 100
    const duration = 10 * time.Minute

    // ... similar setup to single-stream test ...

    var wg sync.WaitGroup
    wg.Add(numClients)

    errors := make(chan error, numClients)

    for i := 0; i < numClients; i++ {
        go func(clientID int) {
            defer wg.Done()
            // ... connect and consume events ...
        }(i)
    }

    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        // All clients finished
    case <-time.After(duration + 1*time.Minute):
        t.Fatal("clients didn't finish within expected time")
    }

    // Collect and report errors
    close(errors)
    errorCount := 0
    for err := range errors {
        if err != nil {
            errorCount++
            t.Logf("client error: %v", err)
        }
    }

    if errorCount > numClients/10 { // allow 10% error rate
        t.Errorf("%d/%d clients had errors", errorCount, numClients)
    }
}

// TestLongLived_WS_100Concurrent tests 100 concurrent WebSocket connections.
func TestLongLived_WS_100Concurrent(t *testing.T) {
    // Similar pattern
}
```

### Step 5.4: Memory Profiling Script

Create `scripts/profile-memory.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Runs long-lived tests with memory profiling enabled.
# Output: mem-profile.pprof and heap-report.txt

TESTDIR="tests/integration"
OUTDIR="profile-output"
mkdir -p "$OUTDIR"

echo "Running long-lived SSE test with memory profiling..."
go test -tags=integration \
    -run='TestLongLived_SSE_1Hour' \
    -timeout=2h \
    -memprofile="$OUTDIR/sse-mem.pprof" \
    -v "./$TESTDIR"

echo "Generating heap report..."
go tool pprof -text "$OUTDIR/sse-mem.pprof" > "$OUTDIR/sse-heap-report.txt"

echo "Top 20 memory consumers:"
head -30 "$OUTDIR/sse-heap-report.txt"

echo ""
echo "Profile saved to $OUTDIR/sse-mem.pprof"
echo "Open interactive view with: go tool pprof $OUTDIR/sse-mem.pprof"
```

Make it executable:

```bash
chmod +x scripts/profile-memory.sh
```

### Step 5.5: CI Configuration

Integration tests should not run on every push (too slow), but should run nightly. Add to `.github/workflows/integration.yml`:

```yaml
name: Integration Tests

on:
  schedule:
    - cron: '0 2 * * *' # Daily at 2 AM UTC
  workflow_dispatch: # Manual trigger

jobs:
  integration:
    runs-on: ubuntu-latest
    timeout-minutes: 180 # 3 hours
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: 'stable'
      - name: Run integration tests
        run: |
          go test -tags=integration \
            -timeout=2h \
            -v \
            ./tests/integration/...
```

## Documentation

Create `docs/performance.md` with results:

````markdown
# Espresso Performance Characteristics

This document describes the behavior of Espresso under long-lived connection
and concurrent-load scenarios, based on integration tests.

## SSE Streams

### Single Stream, 1 Hour Duration

- Event rate: 10 events/second (one every 100ms)
- Total events: ~36,000
- Memory growth: ~5 MB
- Goroutine delta: 0 (returns to baseline after disconnect)
- Conclusion: Stable for at least 1-hour streams.

### 100 Concurrent Streams, 10 Minutes

- Combined event rate: 1000 events/second
- Memory at peak: ~50 MB
- Memory after all disconnect: returns to ~baseline + 5 MB
- Conclusion: Handles 100 concurrent streams without degradation.

## WebSocket Connections

### Single Idle Connection, 1 Hour Duration

- Ping interval: 30 seconds
- Memory growth: ~1 MB
- Goroutine delta: 0
- Conclusion: Stable for at least 1-hour idle connections.

### 100 Concurrent Idle Connections, 10 Minutes

- Memory at peak: ~30 MB
- Memory after disconnect: baseline
- Conclusion: Idle connections are inexpensive.

## How to Reproduce

Run integration tests:

    go test -tags=integration -timeout=2h ./tests/integration/...

For memory profiling:

    ./scripts/profile-memory.sh

## Known Limitations

(Document any discovered limitations here after running tests.)
````

## Tests Required

This task IS the tests. Specifically:

- `TestLongLived_SSE_1Hour` — single SSE stream for 1 hour
- `TestLongLived_WS_1Hour` — single WebSocket for 1 hour
- `TestLongLived_SSE_100Concurrent` — 100 concurrent SSE for 10 minutes
- `TestLongLived_WS_100Concurrent` — 100 concurrent WebSockets for 10 minutes

All tagged with `//go:build integration`. Not run in default test suite.

## Definition of Done

- [ ] All integration tests created and pass consistently
- [ ] Memory usage stable (within 10% growth) over 1-hour runs
- [ ] No goroutine leaks detected (returns to baseline after disconnect)
- [ ] Concurrent load tests pass with 100+ simultaneous connections
- [ ] Memory profiling script created and documented
- [ ] Results documented in `docs/performance.md`
- [ ] CI configuration added for nightly runs
- [ ] Any discovered bugs filed as separate issues
- [ ] `CHANGELOG.md` entry noting "verified long-lived connection behavior"

## Expected Failures → Action Items

If any test fails, do not fix it as part of this task. Instead:

1. Document the failure clearly in the test output
2. File a separate GitHub issue with:
   - Test name
   - Reproduction steps
   - Measured behavior vs expected
   - Hypothesized cause (if obvious)
3. Tag the issue appropriately (bug, needs-investigation)

This way, verification tasks stay focused on verification, and fixes get proper attention.

## Notes

- Integration tests that take >1 hour may be too slow for CI; consider reducing to 10 minutes for CI and 1 hour for local/nightly runs
- If running on a slow machine, some assertions may need adjustment (e.g., goroutine delta tolerance)
- Race detector (`-race`) adds memory overhead; don't use it for memory profiling tests
