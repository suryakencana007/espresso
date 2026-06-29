# Task 2: OpenAPI Security Schemes

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 1.5 days
**Dependencies:** None

> **Status: ✅ Shipped 2026-06-29 (v2.3.0).** Delivered via #49 — AddSecurityScheme + Bearer/APIKey constructors + UnresolvedSecurityRefs.

## Context

The 2026-06-28 verify-and-scope pass (finding D4) confirmed against a real generated spec that the OpenAPI generator advertises operation-level security it never defines. `Security("bearerAuth")` (`openapi/options.go:45`) sets `op.Security` to reference a scheme **by name**, but `components.securitySchemes` is allocated **empty** at `openapi/openapi.go:129` and is **never populated**. The result is a **dangling reference**: the operation points at `bearerAuth`, but `bearerAuth` is not defined anywhere in `components`.

The consequences, both observed when the generated spec is fed to a strict validator and to Scalar:

- The spec **fails strict OpenAPI 3.0 validation** — every `security` requirement must name a key present in `components.securitySchemes`, and there are none.
- The **Scalar / Swagger "Authorize" button breaks** — there is no scheme for the UI to render an auth control against, so secured routes cannot be exercised from the docs.

The fix is the **one small additive API** in v2.3: a way to register a security scheme so that `components.securitySchemes` is populated and every `op.Security` reference resolves to a defined scheme. Keep it minimal — this is a correctness/quality release, not a feature release. No new package is added; this task edits `openapi/openapi.go` and `openapi/options.go` only.

This finding was confirmed by generating a real spec with a `Security(...)`-decorated route and inspecting the emitted JSON: `components.securitySchemes` was `{}` while the operation carried a `security` array referencing `bearerAuth`.

## Acceptance Criteria

- [x] A security-scheme registration API exists on the `Generator` (and/or an `OpenAPIRouter` option) — minimal surface, e.g. `AddSecurityScheme(name string, scheme SecurityScheme)`.
- [x] Registering a scheme populates `components.securitySchemes[name]` in the generated spec.
- [x] A generated spec in which **every** `op.Security` reference resolves to a defined scheme — strict-validation clean, no dangling references.
- [x] At least the two common schemes are documented and expressible: HTTP **bearer** (JWT) and **apiKey** in header.
- [x] The empty-allocation site (`openapi/openapi.go:129`) is wired to the registration API rather than left as a never-populated map.
- [x] `op.Security` set via `Security("name")` (`openapi/options.go:45`) names a scheme that the spec actually defines; if a referenced name has no registered scheme, the mismatch is surfaced (logged or flagged), not emitted silently as a dangling reference.
- [x] CHANGELOG `[Unreleased]` → `Added` for the new `AddSecurityScheme` API, with a short example of registering bearer JWT and an apiKey header scheme.

## Technical Approach

### Step 2.1 — Pin the Current (Wrong) Behavior

Before changing anything, add a characterization test that generates a spec with a `Security("bearerAuth")`-decorated route and asserts the **current** broken state: `components.securitySchemes` is empty while an operation references `bearerAuth`. This documents the dangling-reference starting point and gives a clean diff when the assertion flips in Step 2.4.

### Step 2.2 — Decisions to Make (document the choice in the PR)

1. **Registration surface.** Add the API on the `Generator` (`AddSecurityScheme(name, scheme)`) as the primitive, and optionally expose an `OpenAPIRouter` option that forwards to it so schemes can be registered at router-wiring time alongside `Security(...)` usage. **Recommendation:** ship the `Generator` method as the load-bearing primitive (smallest surface, directly populates `components.securitySchemes`); add the router option only if it composes naturally with how routes are already registered. Whichever is chosen, keep it to a single additive entry point — do not grow the API beyond what D4 requires.

2. **Scheme representation.** Model the scheme as a small struct mirroring the OpenAPI 3.0 Security Scheme Object (`type`, `scheme`, `bearerFormat`, `in`, `name`, `description`) rather than inventing a bespoke shape. Provide the two common forms as documented constructions, not a sprawling helper set:
   - HTTP bearer / JWT — `{type: "http", scheme: "bearer", bearerFormat: "JWT"}`.
   - apiKey in header — `{type: "apiKey", in: "header", name: "X-API-Key"}`.

3. **Mismatch handling.** When `Security("name")` references a name with no registered scheme, decide between (a) logging a warning at generation time, or (b) flagging it. **Recommendation:** surface it (log) consistent with the v2.3 "trustworthy generated artifacts" emphasis — never emit a dangling reference silently. (This dovetails with D6's "do not silently drop" stance in Task 1; keep the behavior consistent.)

### Step 2.3 — Add the Registration API and Populate `components.securitySchemes`

In `openapi/openapi.go`, give the `Generator` a registry for security schemes and an `AddSecurityScheme` method, then populate the components block at generation time instead of leaving the map empty at `openapi.go:129`:

```go
// SecurityScheme mirrors the OpenAPI 3.0 Security Scheme Object (minimal subset).
type SecurityScheme struct {
	Type         string `json:"type"`                   // "http", "apiKey", ...
	Scheme       string `json:"scheme,omitempty"`       // "bearer" for http
	BearerFormat string `json:"bearerFormat,omitempty"` // "JWT"
	In           string `json:"in,omitempty"`           // "header" for apiKey
	Name         string `json:"name,omitempty"`         // header name for apiKey
	Description  string `json:"description,omitempty"`
}

// AddSecurityScheme registers a named security scheme that operations may
// reference via Security("name"). The name is the key emitted under
// components.securitySchemes.
func (g *Generator) AddSecurityScheme(name string, scheme SecurityScheme) {
	if g.securitySchemes == nil {
		g.securitySchemes = map[string]SecurityScheme{}
	}
	g.securitySchemes[name] = scheme
}
```

At the components-assembly site (currently the empty allocation at `openapi.go:129`), copy the registered schemes into `components.securitySchemes` so the generated spec carries them. Common-scheme construction is documented, not special-cased:

```go
// Bearer JWT.
g.AddSecurityScheme("bearerAuth", SecurityScheme{
	Type: "http", Scheme: "bearer", BearerFormat: "JWT",
})
// apiKey in header.
g.AddSecurityScheme("apiKeyAuth", SecurityScheme{
	Type: "apiKey", In: "header", Name: "X-API-Key",
})
```

Notes on the sketch:

- Keep the struct minimal — only the fields the two documented schemes need plus `description`. Do not model OAuth2 flows or OpenID Connect in v2.3 (out of scope for a one-small-API release).
- The `Security("name")` decorator in `openapi/options.go:45` is unchanged in shape; it still sets `op.Security`. The fix is that the named scheme now **exists** in `components`.
- If the spec is cached after route registration (see Task 3, D7), security-scheme registration must happen **before** the marshaled bytes are cached, or the cache must be invalidated on registration. Coordinate with Task 3's caching change in verification.

### Step 2.4 — Flip the Characterization Test

Update the Step 2.1 characterization test so that, after registering `bearerAuth`, the generated spec's `components.securitySchemes` **contains** the referenced scheme and the operation's `security` reference resolves. Add a strict-resolution assertion: for every name appearing in any `op.Security`, that name is present in `components.securitySchemes`.

## Tests Required

- `TestOpenAPISecurityScheme_Registered`: register a bearer-JWT scheme, generate the spec, assert `components.securitySchemes["bearerAuth"]` is present with `type: "http"`, `scheme: "bearer"`, `bearerFormat: "JWT"`.
- `TestOpenAPISecurityScheme_ApiKeyHeader`: register an apiKey header scheme, assert `components.securitySchemes["apiKeyAuth"]` with `type: "apiKey"`, `in: "header"`, `name` set.
- `TestOpenAPISecuredRoute_ReferenceResolves`: register a scheme **and** decorate a route with `Security("bearerAuth")`; assert the operation's `security` reference resolves to the defined scheme (no dangling reference).
- `TestOpenAPISecurity_StrictResolution`: across a multi-route spec, assert every name in any `op.Security` array exists in `components.securitySchemes`.
- `TestOpenAPISecurity_MissingScheme_Surfaced`: decorate with `Security("ghost")` without registering it; assert the mismatch is surfaced (logged/flagged) rather than emitted as a silent dangling reference.
- Run with `-race -count=2`.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `go test -race ./... -count=2` clean.
- [x] `golangci-lint run ./...` clean.
- [x] A generated spec passes strict OpenAPI 3.0 validation for the `security` / `securitySchemes` relationship — no dangling references (asserted in the verification matrix, Task 6).
- [x] CHANGELOG `[Unreleased]` → `Added` for `AddSecurityScheme`, with bearer-JWT and apiKey-header examples.
- [x] PR description records the registration-surface decision (Generator method vs. router option) and the mismatch-handling decision.
- [x] godoc on `AddSecurityScheme` / `SecurityScheme` documents the two common schemes (bearer JWT, apiKey header) and notes OAuth2/OIDC are out of scope for v2.3.
