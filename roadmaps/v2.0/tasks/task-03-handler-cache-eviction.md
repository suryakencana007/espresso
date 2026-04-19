# Task 3: Handler-Cache Eviction + Metrics Hook

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 2 days
**Dependencies:** None

## Context

`handler.go` keeps a package-level reflection cache:

```go
var handlerCache sync.Map // map[reflect.Type]*handlerInfo
```

This cache is a correctness-neutral optimization: it memoizes the reflection parse of each distinct handler function type so subsequent registrations of the same type skip the parse. For a typical app that registers its routes at startup and never at runtime, the cache is bounded by the number of distinct handler signatures — small and constant.

The footgun is **dynamic handler registration**:

- Plugin hosts that load handlers from `.so` files at runtime
- Per-tenant code-generated handlers
- Anything using `reflect.MakeFunc` to synthesize distinct handler types

In these scenarios the cache grows forever. v1.3 documented the constraint (see `handler.go` doc comment + `docs/performance.md`), but did not provide a mitigation.

v2.0 delivers a bounded cache with an eviction hook so operators can see what's being evicted and tune accordingly.

## Acceptance Criteria

- [ ] The cache has a configurable upper bound (default e.g. 1024 entries).
- [ ] When the bound is hit, an entry is evicted (LRU or similar).
- [ ] Evicting an entry does not affect in-flight requests (the `*handlerInfo` stays alive while referenced).
- [ ] A metrics hook `OnCacheEvict(fn func(reflect.Type))` exists and fires per eviction.
- [ ] `go test -race` passes under concurrent registration + eviction.
- [ ] Bench shows eviction overhead is not measurable on the static-route hot path (sub-percent).

## Technical Approach

### Step 3.1 — Pick the Eviction Strategy

Options:

- **LRU** — tracks recency; coldest entry dropped when over bound. Standard choice. Use a small dep like `hashicorp/golang-lru/v2` or implement a minimal in-repo LRU (doubly linked list + map).
- **Random** — cheapest; evicts an arbitrary entry. Works surprisingly well when the access pattern is near-uniform.
- **Size-only, no replacement** — once full, new registrations skip the cache entirely. Safest (no hot data can ever be evicted) but misses the whole point for dynamic workloads.

**Recommended:** LRU, minimal in-repo implementation to avoid adding a dependency. ~80 LOC.

### Step 3.2 — Bounded Cache Implementation

Replace `sync.Map` with an `*lruCache` wrapper behind the same `Load` / `Store` method set:

```go
type handlerCacheT struct {
    mu       sync.Mutex
    maxSize  int
    ll       *list.List                      // doubly linked list for LRU order
    entries  map[reflect.Type]*list.Element
    onEvict  atomic.Pointer[func(reflect.Type)]
}

func (c *handlerCacheT) Load(k reflect.Type) (*handlerInfo, bool) { ... }
func (c *handlerCacheT) LoadOrStore(k reflect.Type, v *handlerInfo) (*handlerInfo, bool) { ... }
```

Key design points:

- `*handlerInfo` values are immutable after creation — safe to keep reading evicted entries if someone still has a pointer.
- Lock only on the cache mutation path (Load hits + Store + Evict). Readers on the hot path (already-registered handlers) do not touch this cache at all — they hold a closure over `*handlerInfo` captured at registration.
- Atomic pointer for the `onEvict` hook so it can be set concurrently without locking.

### Step 3.3 — Configuration Surface

Router-level configuration (via Task 1's per-Router refactor fits here naturally):

```go
espresso.Portafilter(
    espresso.WithHandlerCacheSize(2048),
    espresso.WithHandlerCacheOnEvict(func(t reflect.Type) {
        metrics.Inc("handler_cache.evict", "type", t.String())
    }),
)
```

Default size: 1024. Default OnEvict: nil (no-op).

### Step 3.4 — Tests

- Register 2000 distinct handler types against a 1024-bound cache; assert 976 evictions observed.
- Concurrent registration + handler invocation under `-race`.
- OnEvict hook receives exactly the evicted type.
- After eviction, re-registering the evicted type re-parses (not served from stale memory).

## Tests Required

See Step 3.4 above. Plus:

- Benchmark `BenchmarkHandlerRegistration_SteadyState` to verify the static-route case has no new overhead.
- Benchmark `BenchmarkHandlerRegistration_Overflow` to characterize eviction cost.

## Breaking Changes

- The cache is still package-global unless Task 1's per-Router move is extended here. This task does **not** move the cache; the behavior change is the bound, not the location.
- No public API for `handlerCache` existed before, so the new `WithHandlerCacheSize` / `WithHandlerCacheOnEvict` options are purely additive for the options surface.

## Definition of Done

- Cache is bounded and tested under pressure
- OnEvict hook works
- Bench shows < 1% overhead on static route registration
- `go test -race` passes
- `golangci-lint run ./...` clean
- Migration guide entry (even if just "new options available; existing behavior preserved for static apps")
- CHANGELOG `[Unreleased]` entry under `Changed`
