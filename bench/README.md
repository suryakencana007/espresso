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

## Note on the v2.1 refresh vs the v1.4 baseline (May 2026)

The README numbers were refreshed against v2.1.x and are roughly 2-3x
higher across the board than the v1.4 publication. This is **not** a
framework-dispatch regression — it is a hardware shift. The v1.4
numbers were captured on an Intel Core Ultra 7 155H (a 2023 P-core /
E-core hybrid mobile chip). The v2.1 refresh ran on an AMD Ryzen 7
4800H, a 2020 Zen 2 mobile part with substantially lower
single-thread throughput. Every framework — Gin, Echo, Espresso, and
Fiber — slowed down by roughly the same proportion (~150-250% on
ns/op, identical or near-identical B/op and allocs/op), which is the
fingerprint of a runner change rather than a framework change. The
Go toolchain also moved from 1.23 (v1.4 baseline) to 1.25.6 (current
mise pin), but that contributes a small fraction of the delta
compared to the CPU shift. When the next refresh runs on a machine
closer to the v1.4 baseline class, expect ns/op to drop back into
the same neighborhood as the original publication.
