# Task 1: Reflection-Path Two-Extractor Handlers

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 2 days
**Dependencies:** None

## Context

The reflection dispatch path (`Handler(any)` via `router.Get/Post/Handle`) does **not** support `func(ctx, *Req1, *Req2) (T, error)`, even though `CLAUDE.md` and the `Handler` godoc used to claim it. PR #39 already removed the false doc claim — this task closes the gap between the doc and the code by making the behavior coherent.

The mechanics, confirmed by reading `handler.go` on 2026-06-27:

- `handlerInfo` (`handler.go:19-27`) carries a **single** request slot — `reqPool *sync.Pool`, `reqType reflect.Type`, `reqIndex int`. There is no room for a second extractor.
- The registration-time argument loop (`handler.go:658-682`) walks every input. When it sees a second `FromRequest` argument it re-enters the same branch (`handler.go:668-676`) and **overwrites** `reqPool`/`reqType`/`reqIndex`. Only the last extractor's metadata survives, and registration succeeds **silently** — no panic, no warning.
- At **request** time, `createHandlerFromInfo` (`handler.go:704-748`) reconstructs args. For the first extractor's index, neither the `hasContext`/`ctxIndex` branch (`handler.go:735`) nor the `reqIndex` branch (`handler.go:737`) matches — that slot was clobbered — so control falls to the `else` at `handler.go:744-747`, which executes `panic("espresso: invalid handler argument - this is a bug")`.

Empirically: a two-extractor handler registered via the reflection path panics **per request** (surfacing as HTTP 500 under `RecoverMiddleware`). The typed `HandlerCtxReq1Req2Err` constructor — and its `Lungo` alias (`handler.go:189-234`, `handler.go:560-564`) — works correctly today and populates **both** extractors via independent pools (`pool1`/`pool2`).

The comment at `handler.go:744-745` literally calls the fall-through a "bug." It is reachable. v2.2 makes behavior match contract: either the documented signature actually works on the reflection path, or it is rejected loudly at registration. The request-time "this is a bug" panic must become unreachable.

## Acceptance Criteria

- [ ] The request-time `panic("espresso: invalid handler argument - this is a bug")` at `handler.go:744-747` is **unreachable** for any registrable signature (proven by either approach A or B below).
- [ ] A two-extractor handler — `func(ctx, *Req1, *Req2) (T, error)` — registered via `router.Get/Post/Handle` either: (A) works end-to-end, populating **both** extractors and returning the handler's response; **or** (B) panics at **registration** time with an actionable message naming the typed alternative.
- [ ] The typed `HandlerCtxReq1Req2Err` / `Lungo` path is **unchanged** — same behavior, same per-pool extraction, no new overhead.
- [ ] Single-extractor and zero-extractor reflection handlers (`func() T`, `func(*Req) T`, `func(ctx, *Req) (T, error)`, etc.) are unaffected.
- [ ] `CLAUDE.md` and the `Handler` godoc (`handler.go:55-110`) describe the chosen behavior accurately.

## Technical Approach

### Step 1.1 — Reproduce and Characterize

Lock the current failure before changing anything:

```go
// Reflection path: two extractors registered via router.Get.
r := espresso.Portafilter()
r.Use(httpmiddleware.RecoverMiddleware) // turns the panic into a 500
r.Get("/u/{id}", func(ctx context.Context, p *extractor.Path[UserPath], q *extractor.Query[Filter]) (espresso.JSON[Out], error) {
    return espresso.JSON[Out]{Data: Out{ID: p.Data.ID, F: q.Data.F}}, nil
})
// Today: request panics in createHandlerFromInfo → 500.
```

Assert the per-request panic / 500 today, then assert the chosen post-fix behavior. Pair it with a typed `Lungo` registration of the same handler to show the typed path already works and stays working.

### Step 1.2 — Choose the Approach

This is the decision to make in this task. Both options make the "this is a bug" panic unreachable; they differ in whether the reflection path gains a feature or sheds a false promise.

**Option A — Multi-slot reflection support (make the documented signature work).**

Generalize `handlerInfo`'s single request slot into a slice, mirroring the typed `HandlerCtxReq1Req2Err`:

```go
type reqSlot struct {
    pool  *sync.Pool
    typ   reflect.Type
    index int
}

type handlerInfo struct {
    numIn      int
    numOut     int
    hasContext bool
    ctxIndex   int
    reqSlots   []reqSlot // was: reqPool/reqType/reqIndex
}
```

- Registration loop (`handler.go:658-682`): on each `FromRequest` arg, **append** a `reqSlot` instead of overwriting.
- `createHandlerFromInfo` (`handler.go:704-748`): `Get` from each slot's pool, `Extract` each in argument order (first extractor failure short-circuits via `writeExtractError`, matching the typed path's order in `handler.go:214-222`), reset+`Put` all slots in the deferred cleanup, and assign each arg from its slot. The `else` panic branch then has no remaining case and is removed.

Trade-offs: the reflection path becomes consistent with the typed path and the historical doc claim — the most "behavior matches contract" outcome. Cost: touches the per-request hot path (`createHandlerFromInfo`) and the cache shape; the slice replaces three scalar fields, so cache entries grow slightly and the per-request arg assembly does a small loop instead of indexed branches. Must preserve extraction order and pooling semantics exactly; re-verify with `-race`. Higher implementation and review surface.

**Option B — Fail fast at registration (reject the unsupported signature loudly).**

Keep the single-slot `handlerInfo`. In `handlerFunc` (`handler.go:632-699`), count `FromRequest` arguments during the registration loop; if more than one, `panic` immediately at registration with an actionable message:

```go
panic("espresso: reflection handler has 2 FromRequest arguments; " +
    "two-extractor handlers require HandlerCtxReq1Req2Err (or its Lungo alias)")
```

Because registration now rejects the multi-extractor shape, `createHandlerFromInfo` can never be reached with a clobbered slot, so the `else` panic at `handler.go:744-747` becomes unreachable and can be downgraded to an unreachable-assertion comment or removed.

Trade-offs: minimal, surgical, zero hot-path change, smallest review surface — and it converts a silent request-time 500 into a startup-time failure with a fix in the message (consistent with the framework's "panic at registration, not request" philosophy at `handler.go:107-110`). Cost: the reflection path still does not honor the historical doc claim; users wanting two extractors must switch to `Lungo`/`HandlerCtxReq1Req2Err`. This is a behavior narrowing, documented as such.

**Recommendation:** Option B, unless the reflection path's parity with the typed path is judged worth the hot-path churn. B is lower-risk for a correctness-only release, aligns with the existing "fail at registration" contract, and keeps the per-request path untouched; the typed `Lungo` path already serves the two-extractor use case with no reflection cost. Whichever is chosen, record it in the PR and reflect it in the godoc.

### Step 1.3 — Implement the Chosen Approach

- **If A:** edit `handlerInfo` (`handler.go:19-27`), the registration loop (`handler.go:658-693`), and `createHandlerFromInfo` (`handler.go:704-748`); delete the `else` panic.
- **If B:** add the FromRequest-count guard in `handlerFunc` (`handler.go:632-699`); make the `createHandlerFromInfo` `else` unreachable and annotate/remove it.

Do not touch `HandlerCtxReq1Req2Err`/`HandlerCtxReq1Req2`/`Lungo`/`LungoNoErr` (`handler.go:177-286`, `560-583`).

### Step 1.4 — Docs / godoc

- `handler.go:55-110` — the `Handler` godoc's "Two-extractor handlers" paragraph (`handler.go:74-78`) currently says they are NOT supported and points to `HandlerCtxReq1Req2Err`/`Lungo`. Under approach **A**, rewrite it to document that the reflection path now supports two extractors. Under approach **B**, sharpen it to state that registration panics with the pointer to `Lungo`.
- `CLAUDE.md` — the handler-dispatch section lists `func(ctx, *Req1, *Req2) (T, error)` among the reflection-path signatures. Update to match the chosen behavior (supported under A; rejected-at-registration-with-pointer-to-typed under B).

## Tests Required

- `TestReflectionTwoExtractor`: register `func(ctx, *Req1, *Req2) (T, error)` via `router.Get`; under **A** assert both extractors populate and the response is correct; under **B** assert `router.Get` (or `Handler`) panics at registration with the actionable message.
- `TestReflectionTwoExtractor_RegistrationPanicMessage` (approach **B** only): assert the panic text names `HandlerCtxReq1Req2Err` / `Lungo`.
- `TestLungoTwoExtractor_Regression`: the typed `Lungo` / `HandlerCtxReq1Req2Err` path populates **both** extractors and is byte-identical to v2.1 (locks "typed path unchanged").
- `TestReflectionSingleExtractor_Unaffected`: `func(ctx, *Req) (T, error)` via the reflection path still works.
- The "this is a bug" panic is provably unreachable: no test can trigger `handler.go:744-747` after the fix (covered transitively by the above).
- Run with `-race -count=2`.

## Definition of Done

- [ ] All Acceptance Criteria checkboxes ticked.
- [ ] `go test -race ./... -count=2` clean.
- [ ] `golangci-lint run ./...` clean (gocyclo on `handlerFunc`/`createHandlerFromInfo` still under the min-15 budget — approach A must not push either over).
- [ ] Chosen approach (A or B) recorded in the PR description with its trade-off rationale.
- [ ] CHANGELOG `[Unreleased]` entry: under `Added` if approach A, under `Fixed` if approach B (the per-request "this is a bug" 500 no longer reachable).
- [ ] `Handler` godoc and `CLAUDE.md` updated to match the shipped behavior.
