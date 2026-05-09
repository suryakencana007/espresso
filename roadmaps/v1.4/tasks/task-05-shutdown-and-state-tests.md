# Task 5: Shutdown and State-Injection Coverage Tests

**Priority:** 🔵 Verification
**Estimated Effort:** 1 day
**Dependencies:** Task 3 (concurrency hardening must land first so tests assert the fixed behavior)

## Context

v1.3 shipped graceful shutdown (`OnShutdown` hooks, registry close-all on SIGTERM) and SSE state injection through both `StreamSimple` (covered by tests) and `Stream[Req]` (uncovered). Task 3's atomic / idempotent-Close / readLoop-guard work needs locked behavior, not just code review.

This task adds four targeted tests that lock specific behaviors. None of them exists to chase coverage percentage; each one closes a specific regression vector.

## Acceptance Criteria

- [x] `TestWebSocket_GracefulShutdown` — connect a WebSocket, trigger `router.gracefulShutdown`, assert client receives close code 1001 (going away).
- [x] `TestShutdown_WebSocketsClosed` — mirrors the existing SSE shutdown test; covers the registry close-all path.
- [x] `TestSSE_Stream_StateInjection` — extract state via `MustGetState[T]` inside a `Stream[Req]` handler. Previously only `StreamSimple` had a state test, leaving the typed variant uncovered.
- [x] `TestWithLayers_ExtractorErrorReturnsStructuredJSON` — covered in Task 4. Listed here for completeness.
- [x] All four tests run under `-race` and pass `-count=10`.

## Technical Approach

### TestWebSocket_GracefulShutdown

```go
func TestWebSocket_GracefulShutdown(t *testing.T) {
    router := espresso.Portafilter().
        Get("/ws", espresso.WebSocketSimple(func(ctx context.Context, ws *espresso.WS) error {
            <-ctx.Done()  // hold open
            return nil
        }))
    srv := httptest.NewServer(router)
    defer srv.Close()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"/ws", nil)
    if err != nil { t.Fatal(err) }

    // trigger graceful shutdown
    go router.gracefulShutdown(context.Background(), srv.Config, time.Second)

    _, _, err = conn.Read(ctx)
    var ce websocket.CloseError
    if !errors.As(err, &ce) || ce.Code != websocket.StatusGoingAway {
        t.Fatalf("expected close code 1001, got %v", err)
    }
}
```

### TestShutdown_WebSocketsClosed

Parallel to the existing `TestShutdown_SSEStreamsClosed`. Open N WS connections, kick off shutdown, assert all N receive a close frame within the timeout.

### TestSSE_Stream_StateInjection

```go
type appState struct{ Greeting string }

router := espresso.Portafilter().WithState(appState{Greeting: "hello"}).
    Get("/stream", espresso.Stream[QueryParams](func(ctx context.Context, q *QueryParams, s *espresso.SSEStream) error {
        st := espresso.MustGetState[appState](ctx)
        return s.SendText("greet", st.Greeting)
    }))
```

Assert the stream emits the expected `data: hello` frame.

### TestWithLayers_ExtractorErrorReturnsStructuredJSON

See Task 4.

## Tests Required

This task **is** tests. No additional tests required.

## Definition of Done

- [x] All four tests in the appropriate `_test.go` file (websocket_test.go, sse_test.go, withlayers_test.go).
- [x] `go test -race -count=10 ./... -run 'TestWebSocket_GracefulShutdown|TestShutdown_WebSocketsClosed|TestSSE_Stream_StateInjection|TestWithLayers_ExtractorErrorReturnsStructuredJSON'` clean.
- [x] CHANGELOG entries under `[Unreleased]` → `Added` for each new test.
