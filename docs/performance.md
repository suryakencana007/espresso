# Performance Characteristics

## Overview

This document describes Espresso's behavior under long-lived connection and concurrent-load scenarios, based on integration tests in `tests/integration/`.

## Long-Lived Connections

### SSE Streams

Single SSE streams have been verified to run continuously for 30+ seconds (1-hour tests available via integration tag) with:
- Stable event delivery rate
- No memory leaks (growth < 50 MB)
- Goroutine count returns to baseline after disconnect

### WebSocket Connections

Idle WebSocket connections have been verified to maintain connectivity for 30+ seconds (1-hour tests available via integration tag) with:
- Automatic ping/pong keepalive (configurable interval)
- No memory leaks during idle periods
- Goroutine count returns to baseline after disconnect

## Concurrent Load

### 100 Concurrent Connections

Both SSE and WebSocket have been tested with 100 concurrent connections for 10+ seconds:
- No resource exhaustion
- Error rate < 10% of connections
- Goroutine count returns to near baseline after all disconnects

## Context Cancellation

When a client disconnects:
- **SSE**: `ctx.Done()` fires within 1 second (typically < 100ms)
- **WebSocket**: `ctx.Done()` fires within 1 second after client close
- **Goroutines**: 50 connect/disconnect cycles show no goroutine leaks

## Running Integration Tests

Integration tests are tagged with `//go:build integration` and don't run in default `go test`:

```bash
# Run integration tests (may take several minutes)
go test -tags=integration -timeout=2h ./tests/integration/...

# Run specific test
go test -tags=integration -run TestLongLived_SSE_StableConnection -v ./tests/integration/...
```

## Context Cancellation Tests

Context cancellation tests are in the main test suite (no build tag required):

```bash
go test -race -run TestContextCancellation -v .
```

## Tuning

### GOMAXPROCS

Espresso benefits from multiple cores for concurrent handler execution. The default `GOMAXPROCS` (number of CPUs) is recommended.

### HTTP Server Settings

```go
router.Brew(
    espresso.WithAddr(":8080"),
    espresso.WithReadTimeout(10*time.Second),
    espresso.WithWriteTimeout(10*time.Second),
    espresso.WithIdleTimeout(60*time.Second),
    espresso.WithReadHeaderTimeout(5*time.Second),
    espresso.WithShutdownTimeout(30*time.Second),
)
```

### sync.Pool Usage

Espresso uses `sync.Pool` for request object pooling in handler wrappers (`Ristretto`, `Solo`, `Doppio`, `Lungo`), reducing allocations in hot paths.

## Known Limitations

- **Multipart upload buffering**: `MultipartExtractor` buffers up to 32 MB in memory. For very large uploads (>1 GB), consider using `io.Reader` directly or implementing a streaming extractor.
- **SSE keepalive**: Keepalive comments are sent on a timer; there may be a brief window between a client disconnect and the server detecting it.

## See Also

- [WebSocket](websocket.md) - WebSocket performance characteristics
- [Streaming (SSE)](streaming.md) - SSE performance characteristics
- [Error Handling](error-handling.md) - Error response overhead