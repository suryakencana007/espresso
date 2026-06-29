# Task 1: OpenAPI Generation Correctness

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 3 days
**Dependencies:** None

> **Status: ✅ Shipped 2026-06-29 (v2.3.0).** Delivered via #52 — five generation-path defects fixed.

## Context

The OpenAPI generator emits specs that are quietly wrong. A 2026-06-28 verify-and-scope pass generated a real spec from registered routes and inspected it; five correctness defects in the generation path were confirmed against live code. None throws — every one degrades the spec silently, so the generated artifact disagrees with the routes it claims to describe. v2.3's thesis is **trustworthy generated artifacts**: when generator output disagrees with reality, fix the generator and lock it with a spec-inspection test. This task fixes the five generation-path defects; serving-path issues (D1, D5, D7, D10) belong to Task 3.

The five defects, all confirmed by reading `openapi/introspect.go`, `router_openapi.go`, `extractor/extractor.go`, `response.go`, and `core.go` on 2026-06-28:

- **D2 — the custom-`FromRequest` introspection branch is dead.** `introspect.go:50` declares the probe interface as `interface{ Extract(r any) error }`, but every real extractor implements `Extract(*http.Request) error` (`extractor/extractor.go`, `response.go:96`, `core.go`). No type in the framework satisfies the `Extract(r any) error` shape, so the `paramType.Implements(fromRequestIf)` branch at `introspect.go:124-128` is **never** taken. Custom extractors that are not one of the name-prefix-matched built-ins are never introspected — they contribute no parameters and no request body to the spec.

- **D8 — extractor classification is by type-name prefix string-match.** `getExtractorKind` (`introspect.go:135-167`) classifies via `strings.HasPrefix(getTypeName(t), "PathExtractor"/"Path"/"JSON"/"File"/"Files"/…)`. This is fragile in two confirmed ways: (1) renaming an extractor type silently changes its classification with **zero compile signal**; (2) the prefix table lists `{"FileExtractor", "File"}` before `{"FilesExtractor", "Files"}`, and `"File"` is a prefix of `"Files"`, so a `FilesExtractor[T]` (or any user type named `Files…`, `Format…`, `Pathological…`, etc.) mis-classifies. The classification must key off the **actual extractor types**, not name prefixes.

- **D3 — `extractStatusCode` always returns 0.** `extractStatusCode` (`introspect.go:240-259`) returns `0` on every path — the `//nolint:unparam` comment at `introspect.go:239` literally admits "Always returns 0 for now." `Introspect` stores this into `HandlerInfo.StatusCode` (`introspect.go:105`), and both register paths only ever seed the `"200"` response key. A handler returning `JSON[T]{StatusCode: 201}` (or any non-200 response) is documented as `200`. The status must be derived from the response type — `JSON[T]` defaults to 200 unless its `StatusCode` field is set, `Status`/`Text` carry their own — so a 201 POST documents as 201.

- **D9 — `registerPath` and `RegisterHandler` are drifted copy-paste twins.** `registerPath` (`router_openapi.go:108-168`) and `RegisterHandler` (`router_openapi.go:264-306`) are near-identical, but only `registerPath` wires the response-body schema (the `if info.ResponseType != nil` block at `router_openapi.go:148-165`). `RegisterHandler` stops after seeding `200:{description:"Success"}` with **no** response schema. The two paths must be unified onto one shared helper so both attach the response schema (and both surface the introspect error — see D6).

- **D6 — `registerPath` swallows introspection errors silently.** `registerPath` (`router_openapi.go:109-112`) does `if err != nil { return }` — a route whose handler fails introspection simply **vanishes** from the spec with no diagnostic. Its twin `RegisterHandler` (`router_openapi.go:265-268`) correctly returns the error. `registerPath` cannot change its signature (it is called from the `Get/Post/Put/…` chain, which returns `*OpenAPIRouter`), so the error must be surfaced another way — logged, or recorded on the router for later inspection — but never silently dropped.

These compound: D2 and D8 mean the parameter/body side of the spec is incomplete or mislabeled; D3 means response status codes are wrong; D9 means the `RegisterHandler` entry point omits response schemas entirely; D6 means failures disappear without a trace. After this task the generated spec must accurately reflect the registered routes, locked by a spec-inspection test.

## Acceptance Criteria

- [x] The custom-`FromRequest` probe interface at `introspect.go:50` is `interface{ Extract(*http.Request) error }`, matching the real extractor contract; a custom extractor that is not a built-in is introspected and contributes to the spec (D2).
- [x] Extractor classification keys off the actual extractor types (reflecting against `extractor.PathExtractor[…]`/`QueryExtractor[…]`/… or a kind-reporting interface), not `strings.HasPrefix` on the type name; a `Files…`-named type classifies as files (or its true kind), not files-as-file, and renaming an extractor type produces a compile signal rather than a silent mis-classification (D8).
- [x] `extractStatusCode` derives the real status from the response type: `JSON[T]` → its `StatusCode` field when set, else `200`; `Status`/`Text` → their carried status; unknown/dynamic → a documented default. A handler returning a 201 response documents as `201`, not `200` (D3); the `//nolint:unparam` at `introspect.go:239` is removed because the function now returns meaningful values.
- [x] `registerPath` and `RegisterHandler` share one helper so **both** attach the response-body schema; `RegisterHandler` no longer emits a bare `200:{description:"Success"}` for a handler with a known response type (D9).
- [x] `registerPath` no longer silently drops routes on introspection failure — the error is surfaced (logged and/or recorded on the router) (D6); `RegisterHandler` keeps returning the error.
- [x] The typed extraction/handler dispatch paths and existing public OpenAPI API signatures are unchanged (no breaking changes; v2.0 compat flip still stands — additive only here).

## Technical Approach

### Step 1.1 — Reproduce and lock the wrong spec

Before changing anything, generate a real spec from a route set that exercises all five defects and assert the **wrong** output, so the fix is provably the thing that flips it:

```go
// A custom extractor (not a built-in) — D2 makes it invisible today.
type AuditExtractor struct{ Data AuditReq }
func (a *AuditExtractor) Extract(r *http.Request) error { /* … */ return nil }

// A 201-returning POST — D3 documents it as 200 today.
r := espresso.OpenAPI(gen).
    Post("/users", func(ctx context.Context, body *espresso.JSON[CreateUser]) (espresso.JSON[User], error) {
        return espresso.JSON[User]{StatusCode: 201, Data: User{}}, nil
    })
```

Marshal `gen` to JSON and assert today's defective shape: the custom extractor contributes nothing (D2); the POST response key is `"200"` (D3); a `RegisterHandler` registration has no response schema (D9); a deliberately-uninstrospectable handler vanishes with no diagnostic (D6); a `FilesExtractor`/`Files…`-named type mis-classifies as `file` (D8). Convert each into the post-fix assertion as the steps below land.

### Step 1.2 — D2: fix the custom-`FromRequest` probe interface

In `introspect.go:47-51`, change:

```go
fromRequestIf = reflect.TypeFor[interface{ Extract(r any) error }]()
```

to match the real contract (`extractor/extractor.go`, `response.go:96`, `core.go`):

```go
fromRequestIf = reflect.TypeFor[interface{ Extract(*http.Request) error }]()
```

This requires `import "net/http"` in `introspect.go`. With the corrected interface, the `paramType.Implements(fromRequestIf)` branch at `introspect.go:124-128` fires for real custom extractors. Verify `extractInnerType` (`introspect.go:181-207`) still resolves the inner type for a custom extractor's `Data` field; if the custom type uses a different field name, the branch should still register it (as `KindUnknown`) rather than drop it.

### Step 1.3 — D8: robust type-based extractor classification

Replace the prefix table in `getExtractorKind` (`introspect.go:135-167`) with classification against the actual extractor types. Two viable shapes — pick one in the PR:

- **Type-set match** — build a lookup keyed by the generic base type (`reflect.TypeFor[extractor.PathExtractor[struct{}]]()` etc., compared by the type's `PkgPath()`+base name, not the type-parameter instantiation), so `PathExtractor[Anything]` maps to `KindPath`. This removes the `"File"`-is-a-prefix-of-`"Files"` ambiguity because `FileExtractor` and `FilesExtractor` are distinct base types.
- **Kind-reporting interface** — define an unexported interface the built-in extractors satisfy (e.g. `interface{ openAPIKind() ExtractorKind }`) and implement it on each extractor in `extractor/`. Classification becomes `if k, ok := …; ok { return k.openAPIKind() }`. This makes the kind authoritative at the extractor and gives a compile signal if an extractor is added without a kind.

Whichever is chosen, the ordering hazard (file vs files) must be gone, and renaming an extractor type must not silently change classification. Keep `IsExtractor` (`introspect.go:271-273`) and `extractInnerType` working against the new scheme.

### Step 1.4 — D3: derive the real status code

Rewrite `extractStatusCode` (`introspect.go:240-259`) to return the real status:

- `JSON[T]` (`response.go`): default `200`; if the response value/type carries a non-zero `StatusCode` that is statically knowable, document it — note that `StatusCode` is an instance field, so for the reflection path the realistic rule is "default 200 unless the handler signature encodes otherwise." Where the value is not statically knowable, return the documented default (200) rather than 0.
- `Status` / `Text` (`response.go`): carry their own status; reflect their default.
- Unknown/dynamic response types: a single documented default (200), not 0.

Remove the `//nolint:unparam` at `introspect.go:239`. Update the godoc (`introspect.go:236-239`) to state the function now returns the documented status. `Introspect` already stores the result into `HandlerInfo.StatusCode` (`introspect.go:105`); the register helper (Step 1.5) must seed the response under that status key instead of hard-coding `"200"`.

### Step 1.5 — D9 + D6: unify the two register paths

`registerPath` (`router_openapi.go:108-168`) and `RegisterHandler` (`router_openapi.go:264-306`) duplicate the same logic and have drifted. Extract a single shared helper, e.g.:

```go
func buildPathOperation(gen *openapi.Generator, info *openapi.HandlerInfo, opts ...openapi.OperationOption) *openapi.Operation
```

that does the tag-default, parameter loop (`router_openapi.go:128-146`), **and** the response-schema wiring (currently only `router_openapi.go:148-165`), seeding the response under `info.StatusCode` (from Step 1.4) rather than the literal `"200"`. Then:

- `RegisterHandler` calls the helper, attaching the response schema it currently omits, and keeps returning the introspect error (`router_openapi.go:265-268`) — D9 closed.
- `registerPath` calls the same helper. Its introspect error (`router_openapi.go:109-112`) must no longer be swallowed: surface it via the framework logger and/or record it on `OpenAPIRouter` (e.g. an `errs []error` slice the caller can inspect), since `registerPath` cannot return an error through the fluent `Get/Post/…` chain — D6 closed.

Keep both public signatures (`registerPath` private, `RegisterHandler(gen, method, path, handler, …) error`) unchanged.

## Tests Required

- `TestOpenAPI_CustomExtractorIntrospected` (D2): register a handler taking a custom `FromRequest` whose `Extract(*http.Request) error` is satisfied; marshal the spec and assert the operation reflects the custom extractor (parameter/body present), proving the probe branch now fires.
- `TestOpenAPI_RealStatusCodeDocumented` (D3): a handler returning a 201 response documents under the `"201"` response key, not `"200"`; a default `JSON[T]` handler documents `"200"`.
- `TestOpenAPI_RegisterHandlerAttachesResponseSchema` (D9): a `RegisterHandler` registration with a known `JSON[T]` response carries the response-body schema in the spec — byte-for-byte the same response shape `registerPath` produces for the same handler.
- `TestOpenAPI_IntrospectErrorNotSilentlyDropped` (D6): registering an uninstrospectable handler via the fluent path surfaces a diagnostic (logged/recorded) instead of vanishing; `RegisterHandler` still returns the error.
- `TestOpenAPI_FilesNotMisclassifiedAsFile` (D8): a `FilesExtractor[T]` (and a user type named `Files…`/`Format…`) classifies as its true kind, not file; renaming an extractor type is covered by a compile-time assertion or a test that pins the type-based mapping.
- `TestOpenAPI_SpecMatrix_Snapshot` (cross-cutting): a fixed route set (path + query + JSON body + custom extractor + 201 response) produces a spec asserted field-by-field — the regression-locking spec-inspection test for this release.
- Run with `-race -count=2`.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `go test -race ./... -count=2` clean.
- [x] `golangci-lint run ./...` clean (the removed `//nolint:unparam` no longer flagged; gocyclo on the new shared helper and `getExtractorKind` under the min-15 budget).
- [x] The chosen D8 classification approach (type-set match vs kind interface) is recorded in the PR description with its rationale.
- [x] CHANGELOG `[Unreleased]` entry under `Fixed`: custom extractors now introspected (D2); response status codes documented accurately (D3); `RegisterHandler` attaches response schemas (D9); introspection failures surfaced not dropped (D6); extractor classification no longer prefix-fragile (D8).
- [x] No public OpenAPI API signature changed; no breaking change to extraction or handler dispatch.
