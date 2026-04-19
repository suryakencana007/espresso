# Framework Benchmarks

Head-to-head benchmarks comparing Espresso against Gin, Echo, and Fiber on three
representative scenarios. Lives in its own Go module so the comparison
dependencies (`gin`, `echo`, `fiber` and their transitive graph) don't leak
into the main `espresso` module.

## Scenarios

| # | Scenario      | Request                         | Response      |
|---|---------------|---------------------------------|---------------|
| 1 | Static text   | `GET /ping`                     | `pong` (text) |
| 2 | JSON round-trip | `POST /echo {"name":"world"}` | `{"greeting":"hello world"}` |
| 3 | Path param    | `GET /users/42`                 | `{"id":"42"}` |

Each scenario runs the same logical handler across all four frameworks using
each framework's idiomatic extractor / binding API.

## Methodology

- `httptest.NewRecorder` dispatches requests directly to `http.Handler.ServeHTTP`
  for Espresso, Gin, and Echo — no network, no ports, no flakiness.
- Fiber uses its own `app.Test(*http.Request)` harness because it sits on
  `fasthttp` rather than `net/http`. That harness serializes the request to
  wire format and parses the response out, so Fiber's numbers include
  encode/decode overhead the others don't pay here. Treat Fiber as a
  directional signal, not a fair head-to-head in this test bed.
- Benchmarks call `b.ReportAllocs()` and use `b.ResetTimer()` after setup,
  so the reported allocations are per-request only.

## Running

```bash
cd bench
go test -bench . -benchmem -benchtime=3s -count=1
```

Pin a single CPU core for less variance:

```bash
go test -bench . -benchmem -benchtime=5s -count=3 -cpu=1
```

## Interpretation

Espresso is designed around typed extractors, per-request pooling, and a
structured error pipeline — not for lowest possible ns/op. In practice it is
within a handful of nanoseconds per request of Gin and Echo on net/http
scenarios. The tradeoff buys you compile-time-typed handlers, bidirectional
`FromRequest` / `IntoResponse`, and first-class OpenAPI introspection.

If raw throughput is the only thing that matters, measure your workload —
differences here are dominated by JSON encoding, not by framework dispatch.
