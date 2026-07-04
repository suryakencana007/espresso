# Task 5: OpenAPI Generator — Mutate-While-Serve Race + Recursive-Type Overflow

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 1.5 days
**Dependencies:** None

## Context

Two defects in `openapi/openapi.go` share the file and land together:

- **D1 — the doc claim of race-safety is false.** The `Generator` doc comment (`openapi/openapi.go:141-147`) explicitly states: "Reads and the mutation/invalidation path are guarded by mu, so `Handler` is safe to serve concurrently while registration is still in flight (verified under -race)." Only the cache fields are guarded — every mutation method writes `g.spec` **unguarded**: `AddPath` writes the `Paths` map at `openapi.go:269`, `AddSchema`/`AddSecurityScheme` write component maps (281, 304), `Server` appends (230), all **before** calling `invalidateCache()`. Meanwhile `cachedJSON` (172-180) marshals `g.spec` under `mu`, reading those same maps. A concurrent test running `AddPath` while `Handler().ServeHTTP` under `-race` produces `WARNING: DATA RACE` immediately (`reflect.maplen` in `json.Marshal` at `openapi.go:361/176` vs the `AddPath` write). The existing `TestOpenAPIHandler_ConcurrentServe` (`openapi/serving_test.go:126-146`) only serves concurrently **after** registration completes, so the "-race verified" claim is not actually covered by any test. Unsynchronized map read/write can also fatal-panic without `-race` under adverse scheduling.

- **D2 — recursive types stack-overflow at registration time.** `GenerateSchemaFromType`/`generateSchemaFromStruct` (`openapi/openapi.go:370-447`) have no cycle detection and never emit `$ref` for nested structs — they inline everything. Calling `GenerateSchemaFromType(reflect.TypeOf(node{}))` where `type node struct { Children []*node }` dies with `fatal error: stack overflow` (goroutine stack exceeds 1 GB). This fires at route-registration time via `BuildPathOperation → attachResponse` (`introspect.go:372`) and `GenerateRequestBody` (`introspect.go:512`), so documenting any self-referential request/response type (trees, comment threads, linked categories) kills the process. Stack exhaustion is not a recoverable panic. The `Schema` struct already has a `Ref` field (`openapi.go:106`) and `components/schemas` exists, so the infrastructure for the fix is present.

Secondary: `Spec()` (`openapi.go:365-367`) hands out the internal `*Spec`, so external mutation bypasses cache invalidation. Not the P0, but capture it in the godoc while touching the file.

## Acceptance Criteria

- [ ] `AddPath` (and every mutation method: `AddSchema`, `AddSecurityScheme`, `Server`, `Description`, `SetDescription`) writes `g.spec` inside `g.mu.Lock()` critical sections, folding the existing `invalidateCache()` call into the same section.
- [ ] A `-race` regression test running `AddPath` concurrently with `Handler().ServeHTTP` is clean (was: `WARNING: DATA RACE`).
- [ ] The `Generator` godoc at `openapi.go:141-147` is corrected — either the race-safety claim is true (holds after the fix) or the doc is reworded. Given the fix makes it true, keep the claim and add the new regression test as the "-race verified" reference.
- [ ] `GenerateSchemaFromType` detects self-referential types and emits `$ref: "#/components/schemas/<Name>"` on revisit, registering the type under `components/schemas` on first visit. A recursive `type node struct{ Children []*node }` produces a spec, not a stack overflow.
- [ ] `Spec()` godoc explicitly warns callers that returned `*Spec` mutation bypasses cache invalidation (the finding recommends adding a `SpecClone()` for external readers; land the warning, defer the clone).

## Technical Approach

### Step 5.1 — Reproduce the mutate-while-serve race

```go
func TestGenerator_MutateWhileServeRaceFree(t *testing.T) {
    gen := openapi.NewGenerator("t", "1")
    handler := gen.Handler("/openapi.json")
    // Serve loop in background.
    stop := make(chan struct{}); defer close(stop)
    go func() {
        for {
            select { case <-stop: return; default: }
            rr := httptest.NewRecorder()
            handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
        }
    }()
    // Mutate concurrently — this is what pre-fix races.
    for i := range 100 {
        gen.AddPath(fmt.Sprintf("/p%d", i), &openapi.PathItem{})
    }
}
```

Confirm pre-fix `go test -race` prints `WARNING: DATA RACE`.

### Step 5.2 — Lock every mutation method

For each of `AddPath`, `AddSchema`, `AddSecurityScheme`, `Server`, `Description`, `SetDescription`, wrap the spec mutation and the `invalidateCache()` call in a single `g.mu.Lock()` critical section:

```go
func (g *Generator) AddPath(path string, item *PathItem) *Generator {
    g.mu.Lock()
    defer g.mu.Unlock()
    if g.spec.Paths == nil { g.spec.Paths = map[string]*PathItem{} }
    g.spec.Paths[path] = item
    g.invalidateCacheLocked()
    return g
}
```

Where `invalidateCacheLocked` is the existing invalidate logic factored to assume the lock is already held (rename the current `invalidateCache` and add a locked variant, or just inline). `cachedJSON` (172-180) already reads under `g.mu.RLock()`; keep it as read-lock — mutation methods take write-lock.

### Step 5.3 — Reproduce the recursive-type overflow

```go
func TestGenerateSchemaFromType_RecursiveDoesNotOverflow(t *testing.T) {
    type node struct {
        Value string
        Children []*node `json:"children,omitempty"`
    }
    // Pre-fix: dies with fatal error: stack overflow.
    // Post-fix: returns a Schema with $ref to itself.
    s := openapi.GenerateSchemaFromType(reflect.TypeOf(node{}))
    // Assert s.Properties["children"] resolves to a $ref pointing at "#/components/schemas/node"
}
```

Run this test in a separate process (or with an explicit `runtime.GOMAXPROCS`/stack guard) to avoid killing the test binary on pre-fix — a goroutine's initial stack is 8 KB but grows; the overflow happens well beyond that. Guard the pre-fix run behind a `t.Skip` marker until the fix lands, then flip.

### Step 5.4 — Thread a visited-types set

Change the internal generator signature:

```go
func generateSchemaFromStruct(t reflect.Type, visited map[reflect.Type]bool, gen *Generator) *Schema
```

On entry: `if visited[t] { register-in-components; return &Schema{Ref: "#/components/schemas/" + name(t)} }`; `visited[t] = true`; recurse. On exit: `delete(visited, t)` is not necessary because we're operating per-call — a shared `visited` map traversal is fine. `GenerateSchemaFromType` (the exported entry point) allocates the map and calls the inner. `attachResponse` and `GenerateRequestBody` (in `introspect.go`) pass through unchanged if they don't already call `GenerateSchemaFromType` directly.

Registering under `components/schemas`: `gen.AddSchema(name(t), inlineSchema)` — where `inlineSchema` is what we would have inlined on first visit. This means the recursive type's spec has one component and every reference resolves — matches how tools like `swagger-parser` expect recursive types.

### Step 5.5 — Correct or delete the godoc claim

The `Generator` godoc at `openapi.go:141-147` claims "verified under `-race`". After Step 5.2 lands the claim becomes true. Add a `// See TestGenerator_MutateWhileServeRaceFree` reference so the doc points at the test.

`Spec()` godoc: add "Callers must not mutate the returned Spec; mutations bypass cache invalidation. Use the mutation methods instead. A future release may return a defensive clone."

## Tests Required

- `TestGenerator_MutateWhileServeRaceFree` (D1): concurrent mutate + serve under `-race`, clean.
- `TestGenerateSchemaFromType_RecursiveDoesNotOverflow` (D2): recursive struct produces a `$ref`-based Schema, no overflow.
- `TestGenerateSchemaFromType_MutuallyRecursive`: `type a struct{ B *b }`, `type b struct{ A *a }` — both resolve via `$ref`.
- `TestGenerator_SpecReturnedNotDirectlyMutable`: (light) confirm `Spec()` godoc warns; do not enforce read-only.
- Run with `-race -count=2`.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -race ./openapi/... -count=2` clean.
- [ ] `go test -race ./... -count=2` clean (regression across the module).
- [ ] `golangci-lint run ./...` clean.
- [ ] CI's `Test (race)` job green on the PR.
- [ ] CHANGELOG `[Unreleased]` entry under `Fixed`: `openapi.Generator` mutation methods now hold `g.mu` across spec mutation + cache invalidation, closing the previously-latent data race the godoc claimed was guarded; `GenerateSchemaFromType` handles recursive types via `$ref` to `components/schemas` instead of dying with a stack overflow.
- [ ] No public API signature changed on any exported OpenAPI method.
