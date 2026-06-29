# Task 4: Docs Type-Reference Sweep + Snippet-Compile Check

**Priority:** 🟡 P1 — Should Have (cleanup)
**Estimated Effort:** 1 day
**Dependencies:** None

> **Status: ✅ Shipped 2026-06-29 (v2.3.0).** Delivered via #51 — extractor-generics sweep + snippet-compile guard.

## Context

The documentation site teaches `espresso.Path[T]`, `espresso.Query[T]`, `espresso.Form[T]`, `espresso.Header[T]`, and `espresso.XML[T]` as if those generic extractor types are exported from the root package. They are not. Those types live in the **`extractor`** package — confirmed at `extractor/extractor.go:417-426` (plus the `XML` alias at `extractor.go:429`), where they are declared as type aliases:

```go
type Query[T any]  = QueryExtractor[T]
type Form[T any]   = FormExtractor[T]
type Path[T any]   = PathExtractor[T]
type Header[T any] = HeaderExtractor[T]
type XML[T any]    = XMLExtractor[T]
```

The root package re-exports **response** and **handler** types (`JSON[T]`, `Text`, `Status`, the `Err*` constructors, `WS`, `SSEStream`, the `Ristretto`/`Solo`/`Doppio`/`Lungo` aliases, the `Handler*` family, the state helpers) — but it does **not** re-export the request-side extractor generics. A reader who copies a doc snippet verbatim gets a compile error (`undefined: espresso.Path`). This is the "trustworthy generated artifacts" emphasis of v2.3 applied to the docs: a published code sample that does not compile is a quietly-wrong artifact, exactly like a quietly-wrong OpenAPI spec.

This is a confirmed inventory, not a guess. A `2026-06-28` grep across `docs/` produced the occurrence list below. The fix is a mechanical `espresso.X[` → `extractor.X[` rewrite over a known, bounded set of files, plus a guard so the class of error cannot silently return: a docs-snippet-compile check that extracts self-contained ` ```go ` fences from `docs/` and `go build`s them.

### Confirmed occurrence inventory (verified 2026-06-28)

**`espresso.Path[` — 7 files, 10 occurrences:**

- `docs/guide/extractors.md:69`
- `docs/guide/response.md:18`, `:56`, `:189`, `:216`, `:248`, `:307`, `:323`
- `docs/api/espresso.md:117`
- `docs/api/openapi.md:316`
- `docs/api/state.md:109`
- `docs/examples/basic-api.md:107`, `:135`, `:155`, `:210`, `:410`
- `docs/examples/state-management.md:55`, `:131`
- `docs/guide/routing.md:30`, `:74`

**`espresso.Query[` — 4 files, 6 occurrences:**

- `docs/guide/extractors.md:40`
- `docs/examples/basic-api.md:85`, `:413`
- `docs/examples/production.md:418`, `:419`
- `docs/examples/state-management.md:115`

**`espresso.Form[` — 1 occurrence:**

- `docs/guide/extractors.md:87`

**`espresso.Header[` — 1 occurrence:**

- `docs/guide/extractors.md:105`

**`espresso.XML[` — 2 occurrences:**

- `docs/guide/extractors.md:144`, `:146` (note `:146` is a response-position use, `espresso.XML[XMLResponse]{...}` — still wrong, since `XML` is in the `extractor` package on both the request and response side)

> Line numbers are the 2026-06-28 snapshot and will drift as edits land; treat the file list and per-file occurrence counts as the source of truth, and re-grep before and after editing each file (see Step 4.4).

## Acceptance Criteria

- [x] Every `espresso.Path[`, `espresso.Query[`, `espresso.Form[`, `espresso.Header[`, and `espresso.XML[` reference in `docs/` is rewritten to its `extractor.X[` equivalent, across the files in the inventory above.
- [x] Where a rewritten snippet is a whole, self-contained program (a `package main` fence), it imports `github.com/suryakencana007/espresso/v2/extractor` (alongside the existing root import) so the snippet still compiles after the rewrite.
- [x] No correct root-package symbol is touched (see the do-not-rewrite list in Step 4.2).
- [x] A docs-snippet-compile check exists that extracts self-contained ` ```go ` fences from `docs/` and `go build`s them; it passes.
- [x] A grep for `espresso\.(Path|Query|Form|Header|XML)\[` over `docs/` returns **zero** matches.

## Technical Approach

### Step 4.1 — Rewrite `espresso.X[` → `extractor.X[`

Mechanically rewrite the five extractor generics across the inventory files. The replacement is purely the package qualifier — `espresso.Path[UserPath]` → `extractor.Path[UserPath]`, `espresso.Query[Filter]` → `extractor.Query[Filter]`, and so on. Type arguments, pointers, and surrounding signatures stay byte-identical.

Per-prefix scope (matches the inventory):

- `espresso.Path[` — 7 files, 10 occurrences.
- `espresso.Query[` — 4 files, 6 occurrences.
- `espresso.Form[` — 1 occurrence (`docs/guide/extractors.md:87`).
- `espresso.Header[` — 1 occurrence (`docs/guide/extractors.md:105`).
- `espresso.XML[` — 2 occurrences (`docs/guide/extractors.md:144`, `:146`).

### Step 4.2 — Do NOT Rewrite Correct Root Symbols

These are genuinely exported by the root `espresso` package. Leave every `espresso.`-qualified use of them exactly as written — a blanket `espresso.` → `extractor.` sweep would corrupt them:

- Responses / errors: `JSON`, `Text`, `Status`, `Error`, `Err*` (e.g. `ErrBadRequest`, `ErrNotFound`, …).
- Long-lived / streaming: `WS`, `SSEStream`, `Stream`, `StreamSimple`, `Event`.
- Coffee aliases: `Ristretto`, `Solo`, `Doppio`, `Lungo`.
- Handler family: `Handler*` (e.g. `HandlerCtxReqErr`, …).
- State: `MustGetState`, `GetState`, `State`, `FromState`.
- Validation: `Validation`, `SetDefaultValidator`.

In particular, `espresso.JSON[...]` frequently appears on the **same line** as the extractor being rewritten (e.g. `func getUser(ctx, req *espresso.Path[UserPath]) (espresso.JSON[User], error)`). Rewrite only the `Path`/`Query`/`Form`/`Header`/`XML` qualifier on that line; the `espresso.JSON[...]` return stays. This is why the rewrite must target the five specific prefixes, **not** a global package substitution.

### Step 4.3 — Add the `extractor` Import to Whole-Program Snippets

Two flavors of fence appear in the docs:

- **Fragment fences** — bare function bodies / signatures with no `package` or `import` block. These cannot be compiled standalone (they reference undefined helper types like `UserPath`, `Response`), so they are out of scope for the compile check; the rewrite alone is sufficient.
- **Self-contained fences** — `package main` blocks with their own `import (...)`. After the rewrite these reference `extractor.X`, so each must gain `github.com/suryakencana007/espresso/v2/extractor` in its import block (alongside the existing root `github.com/suryakencana007/espresso/v2` import). These are exactly the fences the Step 4.5 check will build, so a missing import surfaces as a build failure rather than a silent doc bug.

### Step 4.4 — Re-grep Each File Before and After

Because line numbers drift as edits land, drive the edit off grep, not the snapshot offsets:

1. Before editing a file, grep it for `espresso\.(Path|Query|Form|Header|XML)\[` to get the live occurrence set.
2. Rewrite each occurrence per Step 4.1, leaving Step 4.2 symbols untouched.
3. After editing, re-grep the same file to confirm zero remaining matches.

### Step 4.5 — Docs-Snippet-Compile Check

Add a pragmatic, self-contained-subset compile gate so this class of error cannot silently return:

- Walk `docs/` for ` ```go ` fences.
- Keep only **self-contained** fences — heuristically, those whose body contains a `package main` (or `package` + `func main`/`import`) declaration. Fragment fences are skipped by design (they reference undefined helpers and cannot compile-all; attempting to would produce false failures).
- Write each kept fence to a temp `.go` file and `go build` it against the module (the `extractor` and root packages are real imports, so the build resolves them).
- Fail the check if any self-contained fence does not build.

Wire it as a small Go test (or a `tools/`/`cmd/` helper invoked from CI — author's call, mirror whatever the docs build job in `.github/workflows/docs.yml` already does). Keep it pragmatic: the goal is to catch "this published program does not compile," not to compile every fragment. Document the self-contained-only scope in a comment so a future reader understands why fragment fences are intentionally excluded.

## Tests Required

- The docs-snippet-compile check passes: every self-contained ` ```go ` fence in `docs/` `go build`s cleanly after the rewrite.
- A guard (the check itself, or a dedicated assertion) proves a grep for `espresso\.(Path|Query|Form|Header|XML)\[` over `docs/` returns **zero** matches.
- Spot-check that no Step 4.2 root symbol was collaterally rewritten — `espresso.JSON[`, `espresso.Status`, `espresso.Text`, the `Err*` constructors, and the coffee aliases still appear `espresso.`-qualified where they did before.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `grep -rE 'espresso\.(Path|Query|Form|Header|XML)\[' docs/` returns nothing.
- [x] The docs-snippet-compile check is committed and green in CI (or runnable locally per the documented command).
- [x] No correct root symbol (`JSON`, `Text`, `Status`, `Err*`, `WS`, `SSEStream`, `Stream`, `StreamSimple`, `Event`, coffee aliases, `Handler*`, state helpers, `Validation`, `SetDefaultValidator`) was rewritten.
- [x] CHANGELOG `[Unreleased]` → `Fixed` (docs) entry drafted noting the corrected `extractor.X[T]` references and the new snippet-compile guard.
- [x] `bun run docs:build` still succeeds (the rewrite did not break VitePress rendering).
