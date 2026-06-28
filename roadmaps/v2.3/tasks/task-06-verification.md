# Task 6: OpenAPI Spec-Correctness Matrix + Suites Green

**Priority:** 🔵 Verification — tests/coverage, no new features
**Estimated Effort:** 1 day
**Dependencies:** Tasks 1, 2, 3, 4, 5

## Context

Tasks 1-5 are five independent correctness fixes spread across the OpenAPI
generator (`openapi/`, `router_openapi.go`), the docs (`docs/`), and the
long-lived integration harness (`tests/integration/`). They share the v2.3
property: each replaces a quietly-wrong artifact with a correct one — a spec
that introspects custom extractors, documents real status codes, resolves its
security references, attaches response schemas on both registration paths,
never silently drops a route, fails through the JSON envelope, and serves from
cache. A correct artifact is only worth as much as the test that locks it.
This task does not change behavior — it builds the regression net that keeps
Tasks 1-5 from silently regressing, and it codifies the v2.3 emphasis
("trustworthy generated artifacts") as an executable check rather than prose.

Today the relevant assertions are scattered or absent: there is no single place
that generates a spec from a realistic router and asserts "for every route
shape, the emitted OpenAPI document matches reality." The defects in Tasks 1-3
were each found by generating a spec by hand and inspecting it — exactly the
manual step this matrix replaces with a permanent, table-driven guard. There is
also no executable confirmation that the `-tags=integration` suite is green
after the Task 5 WebSocket harness fix. This task consolidates all of that into
one spec-correctness matrix plus the two suite-green gates.

This is verification work: it adds no new feature surface, only tests. It runs
last because it asserts the *combined* post-fix behavior of Tasks 1-5.

## Acceptance Criteria

- [ ] A table-driven `TestOpenAPISpecCorrectness` builds one `*Router` that exercises, on distinct routes, the full extractor/handler matrix and then generates the spec **once**, asserting per row against the generated document:
  - [ ] a `extractor.Path[T]` route — the path parameter appears under the operation's `parameters` with `in: path`.
  - [ ] a `extractor.Query[T]` route — the query parameter appears under `parameters` with `in: query`.
  - [ ] a `JSON[T]` body route — the operation has a `requestBody` whose schema references the request type.
  - [ ] a **custom** `FromRequest` extractor route (Task 1, D2) — it is introspected: its params and/or requestBody are present, NOT silently empty.
  - [ ] a route classified by a user type whose name shares a prefix with a built-in (e.g. a type named `Files...` / `Format...`, Task 1, D8) — classified correctly, NOT mis-bucketed by name prefix.
- [ ] `TestOpenAPISpecCorrectness_StatusCodes` asserts real status codes are documented (Task 1, D3): a `201`-returning POST documents `"201"` (not `"200"`); a `Status`/`Text` route documents its real code; the default `JSON[T]` route still documents `"200"`. No row asserts a spurious `"200"`-only responses map for a non-200 handler.
- [ ] `TestOpenAPISecurityRefsResolve` asserts every `security` reference on every operation resolves to a key present in `components.securitySchemes` (Task 2, D4): a route secured via `Security("bearerAuth")` produces a non-dangling reference, and `components.securitySchemes["bearerAuth"]` is a defined scheme. Walk all operations; fail on any reference with no matching component.
- [ ] `TestOpenAPIResponseSchemaBothPaths` asserts response schemas are present for routes registered via **both** `registerPath` and `RegisterHandler` (Task 1, D9): neither path emits a bare `200:{description:"Success"}` with no schema; both attach the response-body schema.
- [ ] `TestOpenAPINoRouteSilentlyDropped` asserts every registered route appears as a path in the generated spec (Task 1, D6): registering a route whose introspection would previously have been swallowed does NOT make the route vanish; the count of documented paths equals the count of registered routes.
- [ ] `TestOpenAPIFailurePathEnvelope` asserts the spec-generation failure path returns the canonical JSON envelope (Task 3, D1): `Content-Type: application/json`, body decodes into `{"error":{"code","message","details","request_id"}}` with a non-empty `code`; no row asserts a `text/plain` body.
- [ ] `TestOpenAPIServedFromCache` asserts the spec is marshaled once and served from cached bytes (Task 3, D7): two successive requests to the spec handler return byte-identical bodies and the marshal cost is incurred once (e.g. via a marshal-count probe or by asserting the cached bytes are reused).
- [ ] `TestAutoRegisterRemoved` (or equivalent) confirms the no-op `AutoRegister` stub is gone (Task 3, D5): a source/godoc guard fails if the symbol or its misleading "registers all routes" godoc reappears.
- [ ] The full unit suite passes under `-race`.
- [ ] The `-tags=integration` suite passes (green after the Task 5 WebSocket harness fix).

## Technical Approach

### Step 6.1 — Spec-Correctness Matrix Test

Add `openapi_spec_correctness_test.go` (root or `openapi/` package, wherever
the generator is reachable without an import cycle). Build a single `*Router`,
register one route per shape, register the security scheme (Task 2), then call
the generator **once** and decode the document into a navigable structure:

```go
var doc map[string]any
if err := json.Unmarshal(specBytes, &doc); err != nil { t.Fatal(err) }
paths := doc["paths"].(map[string]any)
comps := doc["components"].(map[string]any)
schemes, _ := comps["securitySchemes"].(map[string]any)
```

Drive the assertions table-style — one row per extractor/handler shape — so each
failure names the route and what was expected. Reuse the same generated `doc`
across the related sub-tests (generate once, assert many) to keep the matrix
fast and to mirror the real "spec is immutable after registration" contract that
Task 3's cache relies on. Decode into `map[string]json.RawMessage` where the
test must distinguish "absent" from "null" (e.g. a missing `requestBody` vs. an
empty one).

### Step 6.2 — Security, Response-Schema, and No-Drop Guards

- **Security refs** — walk every operation under `paths`, collect each `security`
  entry's scheme names, and assert each name is a key of
  `components.securitySchemes`. This is the structural inverse of the D4 dangling
  reference: the test fails if any reference has no defining component.
- **Response schema on both paths** — register one route via the
  `registerPath` code path and one via `RegisterHandler`, then assert both
  operations carry a response schema (not a bare `description: Success`). This
  locks the D9 drift fix onto the shared helper.
- **No silent drop** — count registered routes vs. documented paths; assert
  equality. Seed at least one route whose introspection exercises the
  previously-swallowed branch (D6) so the test would fail if the error were
  re-swallowed.

### Step 6.3 — Failure-Path, Cache, and AutoRegister Guards

- **Failure path** — drive the spec handler into its error branch and assert the
  canonical envelope (Task 3 / D1), mirroring the root package's
  `TestWithLayers_ExtractorErrorReturnsStructuredJSON` shape but for the OpenAPI
  serving path. Assert `Content-Type: application/json` and a decodable
  envelope with a non-empty `code`; assert no `text/plain`.
- **Cache** — request the spec twice and assert byte-identical bodies; assert the
  marshal happens once (a marshal-count hook on the generator, or comparing the
  cached `[]byte` identity, whichever Task 3 exposed).
- **AutoRegister removed** — a stdlib-only source/godoc guard (grep over the
  package source, no subprocess) that fails if `AutoRegister` or its
  "registers all routes" godoc reappears. Keep it fast so it runs inside the
  normal `go test ./...` pass.

### Step 6.4 — Suite-Green Gates

No new code — these are the run gates this task certifies:

```bash
go test -race ./...
go test -tags=integration ./tests/integration/... -timeout=2h
```

The integration run must be green on this machine after the Task 5
`conn.CloseRead(ctx)` fix; `TestLongLived_WS_StableConnection` and
`TestLongLived_WS_100Concurrent` no longer time out. Record both runs in the PR.

## Tests Required

- `TestOpenAPISpecCorrectness` — one sub-test per extractor/handler shape (path / query / json / custom `FromRequest` / prefix-collision type); asserts params, requestBody, and correct classification.
- `TestOpenAPISpecCorrectness_StatusCodes` — `201` POST documents `"201"`; `Status`/`Text` document their real codes; default `JSON[T]` documents `"200"`.
- `TestOpenAPISecurityRefsResolve` — every operation `security` reference resolves to a defined `components.securitySchemes` entry; no dangling references.
- `TestOpenAPIResponseSchemaBothPaths` — `registerPath`- and `RegisterHandler`-registered routes both carry a response schema.
- `TestOpenAPINoRouteSilentlyDropped` — documented-path count equals registered-route count.
- `TestOpenAPIFailurePathEnvelope` — failure path returns the canonical JSON envelope, not `text/plain`.
- `TestOpenAPIServedFromCache` — two requests return byte-identical bodies; spec marshaled once.
- `TestAutoRegisterRemoved` — source/godoc guard fails if the no-op `AutoRegister` stub or its godoc reappears.
- All run under `go test -race ./... -count=2`, plus the `-tags=integration` suite green.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -race ./... -count=2` clean.
- [ ] `go test -tags=integration ./tests/integration/... -timeout=2h` clean on this machine (green after Task 5).
- [ ] `golangci-lint run ./...` clean (gocyclo min 15; keep the matrix drivers table-driven so per-function cyclomatic complexity stays under the threshold).
- [ ] Project coverage does not regress below the established minimum; the new matrix measurably raises coverage of the `openapi/` introspection, status-code derivation, security-scheme population, the unified registration helper, and the cached-bytes serving path.
- [ ] PR description lists the spec-correctness matrix and the two suite-green gates, and notes that this task adds tests only — no production code changes.
