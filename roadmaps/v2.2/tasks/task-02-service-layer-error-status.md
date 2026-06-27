# Task 2: Service-Layer Error → HTTP Status Mapping

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 2 days
**Dependencies:** None

## Context

The 2026-06-27 post-v2.1 analysis (finding F-2) confirmed that errors surfaced through **service layers** (`WithLayers` / `WithLayersTyped`) collapse to HTTP 500 even when a more specific status is the contract. `writeHandlerError` (`error.go:221-229`) special-cases only `*espresso.Error` via `errors.As`; anything else is wrapped as `ErrInternal` → 500:

```go
func writeHandlerError(w http.ResponseWriter, r *http.Request, err error) {
	var espErr *Error
	if errors.As(err, &espErr) {
		writeErrorResponse(w, r, espErr)
		return
	}
	wrapped := ErrInternal("internal server error").Wrap(err)
	writeErrorResponse(w, r, wrapped)
}
```

Service-layer errors are **not** `*espresso.Error`, so all three of the built-in layers return 500 with `{"error":{"code":"INTERNAL",...}}`:

- `ValidationLayer` returns `ErrValidation{Err: err}` — a plain struct with no status (`middleware/service/layer.go:331-355`).
- `CircuitBreakerLayer`, when open, returns `*servicemiddleware.CircuitBreakerError` (`middleware/service/layer.go:166-197`, `222-274`).
- `TimeoutLayer` returns `ctx.Err()` = `context.DeadlineExceeded` on the timeout branch (`middleware/service/layer.go:67-73`).

This was proven with a reproduction: each of the three returns 500. The mapping infrastructure already exists and is simply unwired — `ValidationErrors()` builds a 400 / `VALIDATION_ERROR` `*espresso.Error` (`error.go:312-317`), `IsCircuitBreakerError` exists (`error.go:54-58`), and `ErrServiceUnavailable` produces a 503 (`error.go:297-300`). v2.2 wires them together so a layer error maps to the status its contract implies, while preserving the unknown-error → 500 fallback.

All typed and reflection layer call sites funnel their service error through `writeHandlerError` (`withlayers.go:326` and `withlayers.go:437`), so a single translation point covers every layer.

Auto-validate-on-**extract** (which already returns 400 via `ErrBadRequest`/`ValidationErrors` from `SetDefaultValidator`) is correct today and is **out of scope** for this task — do not touch the extract path.

## Acceptance Criteria

- [ ] A `ValidationLayer` failure (`ErrValidation`) maps to a non-500 status with code `VALIDATION_ERROR` and the underlying field detail preserved (not flattened into a bare message).
- [ ] A `CircuitBreakerLayer` open-circuit rejection (`*CircuitBreakerError`) maps to HTTP 503 (`SERVICE_UNAVAILABLE`).
- [ ] A `TimeoutLayer` deadline (`context.DeadlineExceeded`) maps to a non-500 status (503 or 504 — see decision below).
- [ ] An unknown / unrecognized error still maps to HTTP 500 (`INTERNAL`) — the fallback is unchanged.
- [ ] Every mapped error emits the canonical structured-JSON envelope (`{"error":{"code","message","details","request_id"}}`) — no text/plain, no bespoke shape.
- [ ] The existing regression-lock test `TestWithLayersTyped_WithTimeout` (`withlayers_test.go:144-167`), which currently asserts `http.StatusInternalServerError` for a timeout, is updated to the new status, and the change is called out explicitly in the PR description.
- [ ] CHANGELOG `[Unreleased]` → `Changed` with a before/after for each mapped status, plus a short upgrade note (status codes for layer errors change; clients keying off 500 for these must adjust).

## Technical Approach

### Step 2.1 — Pin the Current (Wrong) Behavior

Before changing anything, add characterization tests that assert the *current* 500 behavior for each of the three layer errors. This both documents the starting point and gives a clean diff when the assertions flip in Step 2.4. (These can live alongside the new matrix tests and be edited in place, mirroring how `TestWithLayersTyped_WithTimeout` is updated rather than deleted.)

### Step 2.2 — Decisions to Make (document the choice in the PR)

1. **`ErrValidation` → 400 vs 422.** Map to **400 / `VALIDATION_ERROR`** for consistency with auto-validate-on-extract, which uses `ErrBadRequest`/`ValidationErrors` (a client sees the same 400 shape whether validation runs at extract time or as a layer). The alternative — 422 Unprocessable Entity (`ErrUnprocessableEntity`, `error.go:281-285`) — is more semantically precise but would make the two validation paths disagree. **Recommendation: 400**, to keep the two validation entry points indistinguishable to clients. Whichever is chosen, preserve the field detail.

2. **`context.DeadlineExceeded` → 503 vs 504.** 503 Service Unavailable (`ErrServiceUnavailable`) treats the timeout as "this service couldn't answer in time"; 504 Gateway Timeout is the literal HTTP semantic but Espresso is the origin, not a gateway, so 504 is arguably a misnomer. **Recommendation: 503**, reusing the existing `ErrServiceUnavailable` constructor and aligning timeout with circuit-breaker-open (both are "service can't serve right now"). If 504 is preferred, add an `ErrGatewayTimeout` constructor + `defaultCodeForStatus` entry rather than constructing inline.

3. **Where the translation lives.** Centralize it in `writeHandlerError` (`error.go`). Both layer call sites (`withlayers.go:326`, `withlayers.go:437`) already route through it, so one translation point covers reflection and typed paths with no per-call-site duplication. (The alternative — translating inside `applyLayersAndConvert`/`createTypedHandler` before the `writeHandlerError` call — would have to be applied at every call site and is rejected.)

### Step 2.3 — Add the Translation Step

In `error.go`, introduce a translation helper and call it at the top of `writeHandlerError`, before the existing `*Error` fast path:

```go
// translateLayerError converts known service-layer error types into a
// structured *Error with the correct HTTP status. Returns (nil, false) when
// err is not a recognized layer error, leaving the caller's existing
// *Error / 500-fallback logic intact.
func translateLayerError(err error) (*Error, bool) {
	// Circuit breaker open → 503.
	if IsCircuitBreakerError(err) {
		return ErrServiceUnavailable("service temporarily unavailable").
			WithCode("SERVICE_UNAVAILABLE").Wrap(err), true
	}

	// Timeout layer deadline → 503 (see decision 2).
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrServiceUnavailable("request timed out").Wrap(err), true
	}

	// Validation layer → 400 with preserved field detail (see decision 1).
	var ve servicemiddleware.ErrValidation
	if errors.As(err, &ve) {
		if fe, ok := ve.Err.(FieldErrors); ok {
			return ValidationErrors(fe.ToValidationErrors()).Wrap(err), true
		}
		return ErrBadRequest(ve.Err.Error()).WithCode("VALIDATION_ERROR").Wrap(err), true
	}

	return nil, false
}
```

Then in `writeHandlerError`:

```go
func writeHandlerError(w http.ResponseWriter, r *http.Request, err error) {
	var espErr *Error
	if errors.As(err, &espErr) {
		writeErrorResponse(w, r, espErr)
		return
	}
	if mapped, ok := translateLayerError(err); ok {
		writeErrorResponse(w, r, mapped)
		return
	}
	wrapped := ErrInternal("internal server error").Wrap(err)
	writeErrorResponse(w, r, wrapped)
}
```

Notes on the sketch:

- `error.go` already imports `errors` and `servicemiddleware`; add `context` for `errors.Is(err, context.DeadlineExceeded)`.
- `ErrValidation` is a **value** type (`func (e ErrValidation) Error()`), so `errors.As` targets `servicemiddleware.ErrValidation` (not a pointer). Confirm against `middleware/service/layer.go:331-342`.
- Preserve field detail: when `ErrValidation.Err` is `espresso.FieldErrors`, route it through `ValidationErrors(fe.ToValidationErrors())` so the `details.errors` array survives. When it is some other error, fall back to a message-only 400.
- Order matters: check circuit-breaker and timeout before the generic validation/`*Error` paths so a circuit-breaker error that happens to wrap a deadline is classified once, deterministically.
- The `*Error` fast path stays first so any handler-returned `*espresso.Error` (e.g. from `WithPreFlight` or a normal handler) keeps its explicit status untouched.

### Step 2.4 — Update the Regression-Lock Test

`TestWithLayersTyped_WithTimeout` (`withlayers_test.go:144-167`) currently asserts:

```go
if rec.Code != http.StatusInternalServerError {
	t.Errorf("expected status 500 (timeout), got %d", rec.Code)
}
```

Change the assertion to the chosen timeout status (503 per decision 2) and update the message. Add a body assertion for the structured envelope. This is the one intentional behavior change to a previously-locked test — **call it out explicitly in the PR description** so reviewers see it is deliberate, not a regression.

## Tests Required

A per-error-type status + envelope matrix, each asserting both `rec.Code` and the JSON `error.code`:

- `TestLayerError_Validation_400`: a `ValidationLayer` whose validator returns `espresso.FieldErrors` → status 400, code `VALIDATION_ERROR`, `details.errors` present and matching the field errors.
- `TestLayerError_Validation_NonFieldErrors_400`: `ValidationLayer` whose validator returns a plain `error` → status 400, code `VALIDATION_ERROR`, message preserved.
- `TestLayerError_CircuitBreakerOpen_503`: force the breaker open (failures ≥ threshold, then call) → status 503, code `SERVICE_UNAVAILABLE`.
- `TestLayerError_Timeout_503`: `TimeoutLayer` shorter than the handler sleep → status 503 (the updated `TestWithLayersTyped_WithTimeout` covers this; this is the explicit envelope variant).
- `TestLayerError_Unknown_500`: a layer/handler returning `errors.New("boom")` → status 500, code `INTERNAL` (fallback unchanged).
- `TestLayerError_ExplicitEspressoError_Passthrough`: a handler returning `ErrConflict(...)` through layers → status 409 unchanged (the `*Error` fast path is not shadowed by the new translation).
- Every assertion verifies the canonical envelope shape (`error.{code,message,details,request_id}`), not just the status.
- Run with `-race -count=2`.

## Breaking Changes

Behavior change (allowed under the v2.0 backward-compat flip, documented): service-layer validation / circuit-breaker / timeout errors now return 400 / 503 / 503 respectively instead of 500. Handler-returned `*espresso.Error` values are unaffected, and the unknown-error → 500 fallback is unchanged. The status-code change goes under CHANGELOG `Changed` with a short upgrade note for clients that key off the old 500.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -race ./... -count=2` clean.
- [ ] `golangci-lint run ./...` clean (mind `gocyclo` min 15 on `writeHandlerError` / the new helper — keep the translation in its own function).
- [ ] CHANGELOG `[Unreleased]` → `Changed` with per-status before/after and the upgrade note.
- [ ] PR description states the two decisions made (validation 400-vs-422, timeout 503-vs-504) and explicitly flags the `TestWithLayersTyped_WithTimeout` assertion change.
- [ ] PR description confirms auto-validate-on-extract was left untouched.
