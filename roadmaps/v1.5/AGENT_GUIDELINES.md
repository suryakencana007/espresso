# Guidelines for AI Agents — v1.5

Read this file **in addition to** `roadmaps/v1.3/AGENT_GUIDELINES.md` and `roadmaps/v1.4/AGENT_GUIDELINES.md`. Most of those rules still apply. This file only lists what is **different** for v1.5.

## What's Different in v1.5

### 1. Tasks Trace Back to USAGE_ESPRESSO.md

Every task in v1.5 closes a specific friction entry in `roadmaps/USAGE_ESPRESSO.md`:

- Task 1 → F-05
- Task 2 → F-06
- Task 3 → F-07

When implementing, the test for "is this the right shape?" is "does this make Barista's workaround go away?" Read the linked friction entry, look at the cited Barista file path (e.g. `internal/api/httpx/response.go`), and verify the new Espresso API replaces the chart-internal helper one-for-one. If your design doesn't, you've drifted.

After landing the task, update the corresponding F-NN entry in `USAGE_ESPRESSO.md` to mark it closed (the convention `#### F-04 (closed)` is already established — see v0.3.0).

### 2. Additive Only — No Existing Surface Changes

v1.5 is the smallest possible release that closes the three friction items. Do not:

- Rename existing types (e.g. don't rename `Cookie[T]` → `CookieExtractor[T]`).
- Change existing function signatures.
- Tighten existing struct fields' types.
- Remove anything.

If you find yourself wanting to "tidy up while we're here," stop. Tidying is v2.0 scope (`roadmaps/v2.0/tasks/task-02-remove-deprecated-apis.md`). v1.5's value proposition is "drop in, no migration needed."

### 3. New Extractors Mirror Existing Shapes

`extractor.RawBodyWithHeaders[H]` (Task 2) must match the convention established by `Path[T]`, `Header[T]`, `Cookie[T]`:

- Generic over a struct type that uses tag-keyed fields (`header:"X-Foo"`).
- `Extract(*http.Request) error` with **pointer receiver**.
- `Reset()` for `sync.Pool` reuse.
- Tests live in `extractor/extractor_test.go` next to existing extractor tests.

Don't invent a new pattern. The shape exists; copy it.

### 4. Response Type Additions Stay Backward Compatible

`JSON[T]` (Task 1) gets a new `Cookies []*http.Cookie` field. The zero value (`nil`) must produce **byte-identical** responses to v1.4 — that's the entire backward-compat contract. If a v1.4 user upgrades and never sets `Cookies`, their app must behave the same as before.

Test this explicitly: a regression test that constructs `JSON[T]{Data: x}` with no `Cookies` field and asserts the response bytes match the v1.4 fixture.

### 5. Documentation Lands in the Same PR

For each feature task, update the docs site **in the same PR**:

- Task 1 → `docs/api/response.md` (cookies on JSON response).
- Task 2 → `docs/api/extractor.md` (`RawBodyWithHeaders[H]` section).
- Task 3 → `docs/error-handling.md` and the constructor list in the main README.

If you can't write the doc in the same PR, you don't understand the API well enough to ship it.

## Carried Over From v1.3 + v1.4

Re-read v1.3 and v1.4 guidelines if you haven't recently. The following remain unchanged:

- Read before writing (handler.go, core.go, state.go, router.go, extractor patterns).
- Coffee metaphor for any new public surface.
- Type-safety over `any`.
- `context.Context` mandatory on I/O.
- Conventional Commits, small focused commits.
- `cmd/example/` updated when user-facing APIs change.
- Race detector mandatory.

## Common Mistakes to Avoid (v1.5-specific)

1. **Putting `Cookies` somewhere other than `JSON[T]`.** Don't invent a wrapper type (`JSONWithCookies[T]`); add the field to the existing `JSON[T]`. The whole point is to retire Barista's `httpx.JSONWithCookies[T]` workaround, not to bake the same shape into Espresso.
2. **Designing `RawBodyWithHeaders[H]` to also parse the body.** It's deliberately not a JSON-and-headers extractor — Barista's webhook handlers verify HMAC against the *raw* bytes before decoding. Body must remain `[]byte`. If a user wants typed body + typed headers, they implement `Extract` themselves.
3. **Adding 412 alongside other "while we're here" status codes.** v1.5 adds only `ErrPreconditionFailed`. If users want 410 / 451 / 418, they file a separate request. Scope discipline.
4. **Letting cookies in `JSON[T]` change WriteResponse's encoding path.** Cookies are written via `http.SetCookie(w, c)` *before* the JSON body is encoded. The encoding path itself stays untouched — sonic still does the body, the buffer pool still applies, the existing benchmarks must not regress.
