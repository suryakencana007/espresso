# Task 2: Framework-Comparison Benchmark Module

**Priority:** 🔴 P0 — Feature
**Estimated Effort:** 2 days
**Dependencies:** None

## Context

`TODOS.md` #10 ("Performance Benchmarks → benchmark suite comparing with gin, echo, fiber") had been open since v1.0. The README claimed "fast" without comparable numbers; users had no way to estimate the cost of choosing Espresso over an established framework.

The constraint is **dependency hygiene**: pulling Gin, Echo, Fiber, and fasthttp into the main module would balloon `go.sum` and force every Espresso user to download those graphs even if they never run the benchmarks.

The solution is a **separate Go module** at `bench/` with a `replace` directive back to the parent.

## Acceptance Criteria

- [x] New directory `bench/` with its own `go.mod` and `go.sum`.
- [x] `replace github.com/suryakencana007/espresso => ../` in `bench/go.mod` so the benchmarks always run against the local checkout.
- [x] Three scenarios implemented in all four frameworks (Espresso, Gin, Echo, Fiber):
  - Static text: `GET /ping` → `pong`.
  - JSON round-trip: `POST /echo {"name":"world"}` → `{"greeting":"hello world"}`.
  - Path parameter: `GET /users/{id}` → `{"id":"42"}`.
- [x] `bench/README.md` documents methodology, including Fiber's `app.Test()` harness asymmetry (it serializes to wire format because it sits on fasthttp, not `net/http`).
- [x] Results tabled in main `README.md` under a new "Framework Comparison" section.
- [x] Main module's `go.sum` contains zero entries from gin, echo, fiber, or fasthttp.
- [x] `b.ReportAllocs()` and `b.ResetTimer()` after setup so reported numbers exclude registration overhead.

## Technical Approach

### Step 2.1: Module scaffolding

```
bench/
├── go.mod           # module github.com/suryakencana007/espresso/bench
├── go.sum
├── bench_test.go    # all three scenarios x four frameworks
└── README.md
```

`bench/go.mod` declares its own module path and a `replace` back to the parent so `go test ./...` from inside `bench/` always exercises the working tree, not a tagged version.

### Step 2.2: Dispatch harness

For Espresso, Gin, Echo: use `httptest.NewRecorder` and call `http.Handler.ServeHTTP` directly. No port, no network.

For Fiber: use `app.Test(*http.Request)` because Fiber sits on fasthttp. Document that this harness includes wire-format encode/decode overhead the others don't pay.

### Step 2.3: Result publication

Run `go test -bench . -benchmem -benchtime=3s -count=1` from inside `bench/`. Capture three tables in the README: ns/op, B/op, allocs/op, columns sorted ascending by ns/op.

Annotate Fiber rows with a footnote pointing to the methodology paragraph.

### Step 2.4: Methodology doc

`bench/README.md` covers:

- What each scenario tests.
- Why Fiber's numbers are not directly comparable.
- Hardware used for the published table (Intel Core Ultra 7 155H, Windows 11, Go 1.23).
- Reproduction command.

## Tests Required

This task is itself a test suite. No additional tests required, but:

- `cd bench && go test -bench . -benchmem -benchtime=3s -count=1` must run cleanly on a fresh checkout.
- `go test ./...` from the repo root must continue to pass and **must not** descend into `bench/` (it's a separate module).

## Definition of Done

- [x] Bench module compiles and runs against the local Espresso checkout.
- [x] Main module `go.sum` unchanged after this task lands.
- [x] README "Framework Comparison" tables populated with absolute numbers and a methodology footnote.
- [x] CHANGELOG entry under `[Unreleased]` → `Added`.
- [x] CI does not run the bench module on every push (it's reproducible by hand; running it on shared CI would produce noisy results).
