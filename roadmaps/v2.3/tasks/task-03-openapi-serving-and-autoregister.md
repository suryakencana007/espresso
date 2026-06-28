# Task 3: OpenAPI Serving Hardening + Remove `AutoRegister` Stub

**Priority:** 🟡 P1 — Should Have (hardening)
**Estimated Effort:** 1.5 days
**Dependencies:** Tasks 1, 2 (shares `openapi/openapi.go` with Task 2 and `router_openapi.go` with Task 1)

## Context

Tasks 1 and 2 make the *generated spec* correct. This task makes the *serving and surface* of that spec trustworthy: the failure path, the per-request cost, the docs UI delivery, and one API symbol that openly lies about what it does. A 2026-06-28 verify-and-scope pass generated a real spec, served it, and inspected the handler/serving code; the four findings below were each confirmed against the source.

**D1 — failure path emits `text/plain`, not the canonical envelope.** When spec marshaling fails, `Handler()` (`openapi/openapi.go:320`) calls `http.Error(...)`, producing a `text/plain` body with no `code`, no `request_id`, and no `{"error":{...}}` wrapper — the opposite of every other framework failure path. The standard fix elsewhere is to route through the canonical envelope, but the `openapi` package **must not import the root `espresso` package** (no `*espresso.Error` is constructible here without an import cycle). The cycle-safe path already exists: the stdlib-only `internal/errorenvelope` leaf added in v2.2 (`internal/errorenvelope/errorenvelope.go`) is importable by `openapi` with no cycle, and emits the exact `{"error":{"code","message","details","request_id"}}` shape. Severity low, fix small.

**D7 — the spec is re-marshaled on every request.** `Handler()` (`openapi/openapi.go:316-327`) calls `ToJSON()` → `json.MarshalIndent` on **each** hit. The `Generator` carries no cached bytes. But the spec is immutable once route registration completes — every request re-does identical work and identical allocations. Severity low, fix small.

**D10 — the Scalar UI loads an unpinned, external-only CDN bundle.** `scalar.go:18` embeds `https://cdn.jsdelivr.net/npm/@scalar/api-reference` with **no `@version` pin** (resolves to `latest`, so a breaking Scalar release silently breaks the docs page) and from an external host (the page is blank offline / under a strict CSP). Severity low, fix small.

**D5 — `AutoRegister` is a no-op stub with a godoc that promises otherwise.** `router_openapi.go:248-249` is an **empty no-op**, yet its godoc (`router_openapi.go:234-247`) describes it in detail as registering every route on the router into the spec. The API lies: callers wire it up and get nothing. **Decision (locked by the maintainer): delete the no-op `AutoRegister` and its misleading godoc** so the surface stops promising behavior it does not have. Genuine auto-registration is real future work and is out of scope for v2.3 — mention it only as a possible future feature, not as a deferred v2.3 deliverable. Severity medium, fix small (deletion).

This task is **P1, not P0** because each finding is a hardening/correctness cleanup on serving and surface rather than a wrong-spec defect, and because it edits files shared with Tasks 1 and 2 — it must land **after** them to avoid churn on `openapi/openapi.go` (Task 2) and `router_openapi.go` (Task 1).

## Acceptance Criteria

- [ ] The spec-generation failure path no longer emits `text/plain`: on a marshal failure, `Handler()` writes the canonical `{"error":{"code","message","details","request_id"}}` envelope with `Content-Type: application/json` and an appropriate 5xx status, produced via the stdlib-only `internal/errorenvelope` leaf.
- [ ] The `openapi` package still does **not** import the root `espresso` package (no import cycle introduced); the envelope is built only from `internal/errorenvelope`.
- [ ] The marshaled spec is generated **once** and served from cached bytes on subsequent requests — a single marshal regardless of request count, justified by the spec being immutable after route registration.
- [ ] The Scalar UI bundle URL in `scalar.go:18` is pinned to a specific `@version` (not `latest`), and the offline / air-gapped / strict-CSP limitation is documented (self-host guidance) at the call site or in the docs.
- [ ] The `AutoRegister` symbol and its godoc (`router_openapi.go:234-249`) are **deleted**; the package compiles with no remaining reference to it.
- [ ] Real auto-registration is recorded as possible future work (CHANGELOG/note), explicitly **not** shipped in v2.3.

## Technical Approach

### Step 3.1 — Route the Failure Path Through the Cycle-Safe Envelope (D1)

`openapi/openapi.go:320` currently does:

```go
http.Error(w, err.Error(), http.StatusInternalServerError) // text/plain — wrong shape
```

Replace it with a write through the v2.2 leaf, which is stdlib-only and importable here without a cycle:

```go
import "github.com/suryakencana007/espresso/v2/internal/errorenvelope"

errorenvelope.Write(w, http.StatusInternalServerError, errorenvelope.Body{
    Code:    "INTERNAL",
    Message: "failed to generate OpenAPI specification",
    // RequestID: from the request context if a request-id is available at this layer.
})
```

Confirm the direction stays `openapi → internal/errorenvelope` only — the `openapi` package must not gain an edge to the root package. If `errorenvelope` lacks a field or helper this path needs, do not add a root dependency to obtain it; extend the leaf (stdlib-only) instead. Whether a `request_id` is reachable at this serving layer is an author's call — populate it if the request context carries one, otherwise leave it omitted (`omitempty`), matching the canonical rule.

### Step 3.2 — Cache the Marshaled Spec (D7)

`Handler()` (`openapi/openapi.go:316-327`) marshals on every request. Marshal once and serve the cached bytes. Sketch:

```go
// On the Generator (or in Handler via sync.Once): compute the marshaled
// bytes the first time, then serve the slice on every subsequent request.
var (
    specOnce  sync.Once
    specBytes []byte
    specErr   error
)
specOnce.Do(func() { specBytes, specErr = g.ToJSON() })
if specErr != nil {
    // D1 envelope path from Step 3.1
}
w.Header().Set("Content-Type", "application/json")
_, _ = w.Write(specBytes)
```

The immutability assumption is load-bearing: this caches because the spec does not change after route registration completes. If any code path can mutate the spec after the first serve, the cache must be invalidated or this optimization does not hold — record that assumption in the PR. Decide whether the cache lives on the `Generator` or is closed over inside `Handler()`; either is fine as long as the marshal-once guarantee holds and the failure path (D1) is reached when the one marshal fails.

### Step 3.3 — Pin the Scalar Bundle + Document Self-Hosting (D10)

`scalar.go:18` references the CDN with no version:

```
https://cdn.jsdelivr.net/npm/@scalar/api-reference          // resolves to latest — unpinned
```

Pin a specific published version, e.g.:

```
https://cdn.jsdelivr.net/npm/@scalar/api-reference@<pinned-version>
```

Pick a concrete, currently-published `@scalar/api-reference` version and record it in the PR. Add a short note (godoc at the call site and/or the OpenAPI docs page) that the Scalar UI loads from an external CDN, so offline / air-gapped / strict-CSP deployments should self-host the bundle and point the UI at the local copy. This is a documentation acknowledgment, not a feature — do not build a self-hosting mechanism in v2.3.

### Step 3.4 — Delete the `AutoRegister` No-Op (D5)

Delete the no-op body (`router_openapi.go:248-249`) **and** its misleading godoc (`router_openapi.go:234-247`). The decision is delete, not implement. Grep the repo for any remaining `AutoRegister` reference (tests, `cmd/example`, docs) and remove or repoint each so the package compiles. In the CHANGELOG entry (Task 7) note the removal under `Removed` and mention that genuine route auto-registration is possible future work — framed as a future feature, not a v2.3 commitment.

## Risks

- **Cycle reintroduction.** The entire reason D1 uses the leaf is to avoid `openapi → root`. A `go list` / build check that `openapi` does not import the root `espresso` package belongs in the verification task (Task 6).
- **Cache staleness.** Caching the marshaled bytes assumes the spec is frozen after registration. If a downstream registers routes lazily or mutates the spec post-first-serve, the cache is wrong. Confirm registration is complete before the first serve, or invalidate on mutation.
- **`AutoRegister` is exported.** Deleting an exported symbol is an API removal. Because the v2.0 backward-compat flip stands and the symbol is a documented-but-dead no-op (it never did anything), removal is in-scope for this minor — but it must be called out explicitly under `Removed` in the CHANGELOG with the rationale ("the symbol was a no-op; deleting it stops the API from promising behavior it never had").
- **Scalar version drift.** Pinning fixes silent breakage from `latest`, but a pinned version eventually goes stale. Note in the PR that the pin should be bumped deliberately in a future release, not left to float.
- **Shared-file conflicts.** This task edits `openapi/openapi.go` (also touched by Task 2) and `router_openapi.go` (also touched by Task 1). Land after Tasks 1 and 2 and rebase onto their merges to keep the diffs clean.

## Tests Required

- `TestOpenAPIHandler_MarshalFailure_Envelope`: drive `Handler()` down the failure path and assert the response is `Content-Type: application/json` with the canonical `{"error":{...}}` envelope (code, message present), **not** `text/plain`.
- `TestOpenAPIHandler_SpecServedFromCache`: serve the spec multiple times and assert the marshal happens exactly once (e.g. via an injected/counting marshal hook or by asserting byte-identical responses backed by a single generation), confirming D7.
- `TestOpenAPIHandler_SpecBytesStable`: two successive GETs return byte-identical bodies (the cache returns the same marshaled slice).
- `TestScalarHTML_VersionPinned`: assert the served Scalar HTML references a pinned `@scalar/api-reference@<version>` and does **not** reference the unpinned bare package path.
- `TestAutoRegisterRemoved`: a compile-level / grep guard that the `AutoRegister` symbol no longer exists (no reference compiles; repo grep for `AutoRegister` returns only CHANGELOG/roadmap mentions).
- An import-direction guard (or defer to Task 6): `openapi` does not import the root `espresso` package.
- Run with `-race`.

## Breaking Changes

**API removal (documented).** Exported `AutoRegister` is deleted. It was a no-op, so no behavior is lost — but the symbol disappears, so any code referencing it stops compiling. Goes under `Removed` in the CHANGELOG (Task 7) with the rationale and a pointer that real auto-registration is possible future work. **Behavior change (documented):** the OpenAPI spec-generation failure path changes from `text/plain` to the canonical JSON envelope; note under `Changed` with before/after body and content type.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `openapi` is confirmed to not import the root `espresso` package (no cycle introduced by the D1 fix).
- [ ] `go test -race ./...` clean.
- [ ] `golangci-lint run ./...` clean.
- [ ] CHANGELOG `[Unreleased]` entries drafted: `Changed` for the failure-path envelope and the Scalar pin; `Removed` for `AutoRegister` (with the "no-op; real auto-registration is future work" note).
- [ ] PR description records the chosen Scalar version pin, the cache-immutability assumption, and the confirmation that the failure path no longer emits `text/plain`.
