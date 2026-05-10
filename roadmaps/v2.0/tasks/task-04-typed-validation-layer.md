# Task 4: Typed `Validation[T]` Layer

**Priority:** 🟡 P1 — Should Have
**Estimated Effort:** 2 days
**Dependencies:** None


> **Status: ✅ Shipped 2026-05-10.** Delivered via #23.

## Context

`layerconfig.go` currently exposes:

```go
func Validation(validator any) LayerConfig {
    return &validationConfig{validator: validator}
}
```

The `any` there predates the typed `servicemiddleware.Validator[Req]` interface that `middleware/service/layer.go:319` now defines. Every real caller passes a `Validator[Req]` anyway — the `any` is only there because v1.x could not introduce a generic parameter on `Validation` without breaking existing callers that used a type inference pattern.

v2.0 tightens this:

```go
func Validation[Req any](validator servicemiddleware.Validator[Req]) LayerConfig
```

Benefits:

- Compile-time check that the validator's Req type matches the handler's request type.
- No more runtime type assertion inside `buildLayer` for the validation case.
- Documents intent: the reader sees `Validation[*JSON[CreateUserReq]](...)` and immediately knows the scope.

## Acceptance Criteria

- [x] `Validation` becomes generic: `Validation[Req any](servicemiddleware.Validator[Req]) LayerConfig`.
- [x] `validationConfig` is updated to carry the generic or preserves enough info to dispatch typed at layer-build time.
- [x] All internal callers and tests updated to pass the type parameter explicitly (Go infers it in most call sites).
- [x] A test demonstrates the compile-time error when a validator is mismatched against a handler's request type.
- [x] Existing `servicemiddleware.ValidationLayer` keeps working — we only change the `espresso.Validation` sugar, not the underlying typed primitive.

## Technical Approach

### Step 4.1 — Generic `Validation`

Replace the v1 function:

```go
func Validation[Req any](validator servicemiddleware.Validator[Req]) LayerConfig {
    return &validationConfig[Req]{validator: validator}
}

type validationConfig[Req any] struct {
    validator servicemiddleware.Validator[Req]
}

func (c *validationConfig[Req]) layerConfig() {}
```

### Step 4.2 — Update `buildLayer`

`buildLayer` in `withlayers.go` switch-dispatches on `LayerConfig` type. Add a case:

```go
func buildLayer[Req, Res any](cfg LayerConfig) func(Service[Req, Res]) Service[Req, Res] {
    switch c := cfg.(type) {
    // existing cases...
    case *validationConfig[Req]:
        return servicemiddleware.ValidationLayer[Req, Res](c.validator)
    case LayerConfig:
        // existing any-case fallback — may be removable once migration is done
    }
}
```

Edge case: what if a user builds a `validationConfig[Req1]` but passes it into a `WithLayers[Req2, Res]` pipeline? The type-switch will fail to match, and the pipeline should panic at registration time with a clear error message (**not** silently skip validation).

### Step 4.3 — Update Tests

- `withlayers_test.go` already has `mockValidator` — type it properly.
- Add a new test that constructs a mismatched validator and asserts the registration-time panic.

## Tests Required

- Happy path: typed validator runs, rejects bad input with `ErrValidation`.
- Mismatch: validator for `Req1` attached to a handler with `Req2` panics at registration with a descriptive message.
- Existing `TestLayerConfig_Validation` etc stay passing.

## Breaking Changes

- Call sites of `espresso.Validation(validator)` need to become `espresso.Validation[Req](validator)` — in most cases Go infers the type argument from the validator, so literal call-site rewrites are rare.
- `validationConfig` is now generic; anyone who reached into it (unlikely — it's unexported) breaks.

## Definition of Done

- Generic signature in place, internal callers typed
- Mismatch test demonstrating registration-time panic
- `go test ./... -race` passes
- `golangci-lint run ./...` clean
- Migration guide entry (mostly: "Go infers the type; usually no change needed, but explicit form is recommended for readability")
- CHANGELOG `[Unreleased]` entry under `Changed`
