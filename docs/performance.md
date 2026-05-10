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

## Handler-Reflection Cache

`espresso.Handler()` caches per-handler reflection metadata in a
process-global cache keyed by handler function type, so subsequent
registrations of the same handler shape skip the reflection parse.

Since v2.0 the cache is **bounded with LRU eviction**. Default upper bound
is `DefaultHandlerCacheSize` (1024 entries). Tuning surface:

```go
// Resize the bound. Pass 0 or negative to reset to the default.
espresso.SetHandlerCacheSize(2048)

// Observe evictions for telemetry.
espresso.OnHandlerCacheEvict(func(t reflect.Type) {
    metrics.Inc("handler_cache.evict", "type", t.String())
})
```

Static apps (routes wired at startup) stay well under the default bound
and never evict — the LRU bookkeeping adds about 24 ns per registration
(measured: `BenchmarkHandlerCache_SteadyState`) but does not touch the
per-request hot path.

Dynamic-registration scenarios (plugin hosts, per-tenant codegen,
`reflect.MakeFunc`) now have an upper bound on cache memory regardless
of churn rate. For those, set `OnHandlerCacheEvict` to a metric increment
to alert on unexpected churn. Eviction overhead is bounded too:
`BenchmarkHandlerCache_Overflow` measures roughly 224 ns/op for the
insert + LRU update + evict + hook fire path.

In-flight requests are unaffected by eviction: `*handlerInfo` values are
immutable, and request-side handlers hold a pointer captured at
registration time. Eviction only drops the cache's reference; existing
pointers continue to work until garbage-collected.

If reflection itself is unwanted (any handler-cache pressure is too much),
use the typed handler variants — `Ristretto`, `Solo`, `Doppio`, `Lungo`,
`HandlerCtx*` — which skip this cache entirely.

## Framework Comparison

A Gin / Echo / Fiber comparison lives in [`bench/`](https://github.com/suryakencana007/espresso/v2/tree/main/bench) as a separate Go module (so the comparison deps don't pollute the main module). It covers three scenarios (static text, JSON round-trip, path param) and reports ns/op + B/op + allocs/op.

Summary on an Intel Core Ultra 7 155H, Go 1.23 (`go test -bench . -benchmem -benchtime=3s`):

- Espresso is within ~12% of Gin on static text and within ~35% on path parameters.
- On JSON round-trip, Espresso (979 ns/op) beats Gin (1412 ns/op) but trails Echo (774 ns/op).
- Fiber's numbers in that harness include fasthttp wire-format overhead and are directional only.

See the full table and methodology in [the main README](../README.md#framework-comparison) and [`bench/README.md`](https://github.com/suryakencana007/espresso/v2/tree/main/bench/README.md).

## See Also

- [WebSocket](websocket.md) - WebSocket performance characteristics
- [Streaming (SSE)](streaming.md) - SSE performance characteristics
- [Error Handling](error-handling.md) - Error response overhead