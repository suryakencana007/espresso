# Guidelines for AI Agents — v2.3

Read this file **in addition to** `roadmaps/v1.3/AGENT_GUIDELINES.md` (the baseline), `roadmaps/v1.4/AGENT_GUIDELINES.md`, `roadmaps/v1.5/AGENT_GUIDELINES.md`, `roadmaps/v2.0/AGENT_GUIDELINES.md`, `roadmaps/v2.1/AGENT_GUIDELINES.md`, and `roadmaps/v2.2/AGENT_GUIDELINES.md`. Most rules carry forward unchanged. This file only lists what is **different** for v2.3.

## What's Different in v2.3

### 1. Verify Generator Fixes by Generating a Spec — Not by Reading Code

v2.3 makes the OpenAPI generator trustworthy. Every defect was confirmed by **generating a real spec and inspecting the emitted JSON**, and every fix must be locked the same way: register a representative route set, call `ToJSON()`, and assert against the marshaled document. A green unit test that only exercises a helper in isolation is not sufficient — the lock is a **spec-inspection test** that proves the generated artifact agrees with the routes. If you fix `extractStatusCode` (Task 1) but the generated `responses` map still reads `"200"` for a `201` route, the fix did not land. Inspect the bytes.

### 2. The `openapi` Package Must Not Import the Root `espresso` Package

`openapi/` is a lower-level package; the root package depends on it, not the other way around. Do **not** introduce a back-edge to build an `*espresso.Error` on the spec-generation failure path (Task 3, D1). Reuse the stdlib-only `internal/errorenvelope` leaf (introduced in v2.2 for exactly this kind of cross-package envelope problem) to emit the canonical JSON shape with no import cycle. This is the same discipline v2.2 applied to `middleware/http` — fix the envelope through the shared leaf, never through a root import.

### 3. Deleting the `AutoRegister` No-op Is Intended

`AutoRegister` (`router_openapi.go`) is an empty stub fronted by a detailed godoc that promises route registration it never performs. The maintainer has **chosen delete over implement** for v2.3 (Task 3, D5). Remove the no-op method **and** its misleading godoc so the API stops lying — do not leave a stub, and do not quietly turn it into a real implementation as a surprise. Real auto-registration is future work; mention it as a possible future feature in the CHANGELOG/release note, not as something v2.3 ships.

### 4. The WebSocket Item Is a Test Fix Only — Do Not Touch `websocket.go`

`TestLongLived_WS_StableConnection` fails because the **test client** never calls `conn.Read`, so `coder/websocket` never processes the pong that `Ping` waits on (Task 5). The espresso WS **server is correct** — its read loop drains the connection and auto-replies pong. The fix is a one-line `conn.CloseRead(ctx)` after the dial in the test harness (the library-documented idiom for read-less clients), plus the same correctness fix in `TestLongLived_WS_100Concurrent`. Make **no framework code change**; `websocket.go` is out of bounds for this task. After the fix, `go test -tags=integration ./tests/integration/...` must pass on this machine.

### 5. The Docs Sweep Rewrites Extractor Types Only — Never the Root Symbols

Task 4 corrects `espresso.Path`/`Query`/`Form`/`Header`/`XML[T]` to `extractor.Path`/… because those types live in the `extractor` package and are **not** re-exported by root. Rewrite **only** those extractor types. Do **not** touch the symbols that are genuinely root: `JSON`, `Text`, `Status`, `Error`, the `Err*` constructors, `WS`, `SSEStream`, `Stream`, `StreamSimple`, `Event`, `Ristretto`/`Solo`/`Doppio`/`Lungo`, the `Handler*` family, `MustGetState`/`GetState`/`State`/`FromState`, `Validation`, `SetDefaultValidator`. A correct `espresso.JSON[T]` left intact is the goal; an over-eager rewrite of a root symbol is a regression. The snippet-compile check targets self-contained whole-program (`package main`) fences — fragment fences referencing undefined helpers cannot compile and are out of scope.

## Carried Over From Earlier Versions

Re-read the prior guidelines if you haven't recently. The following remain unchanged and not repeated here:

- Read before writing (`handler.go`, `core.go`, `error.go`, `router.go`, `openapi/openapi.go`, `openapi/introspect.go`, `router_openapi.go`, extractor patterns).
- Coffee metaphor for any new public surface (the one additive API this release is `AddSecurityScheme`, Task 2).
- Type-safety over `any`.
- `context.Context` mandatory on I/O.
- `sync.Pool` for hot paths; atomic over mutex on hot paths.
- Prefer correcting behavior over adding surface — when generator output and reality disagree, fix the generator and lock it with a test.
- Conventional Commits, small focused commits, feature-branch + PR (UI merge — branch protection blocks CLI merge).
- `cmd/example/` updates when user-facing behavior changes.
- Race detector mandatory; `golangci-lint` (gocyclo min 15) clean.

## Common Mistakes to Avoid (v2.3-specific)

1. **"Fixing" the generator by reading code instead of generating a spec.** The lock is a spec-inspection test that asserts on the marshaled JSON. If you didn't generate and inspect, you didn't verify.
2. **Importing root into `openapi/` for the failure-path envelope.** Use the `internal/errorenvelope` leaf (D1). A back-edge is an import cycle the framework has always avoided.
3. **Leaving (or silently implementing) the `AutoRegister` stub.** Delete the method and its godoc (D5). Future auto-registration is a note, not code, in v2.3.
4. **Changing `websocket.go` for the WS test failure.** The server is healthy; the fix is `conn.CloseRead(ctx)` in the test (Task 5). No framework change.
5. **Over-rewriting the docs sweep.** Only `espresso.Path/Query/Form/Header/XML` → `extractor.*`. Never rewrite root symbols (`JSON`/`Text`/`Status`/`Error`/`WS`/`Stream`/`Lungo`/…).
6. **Classifying extractors by type-name prefix.** D8 is exactly this trap (`"File"` is a prefix of `"Files"`). Classify against the real extractor types, not string prefixes.
7. **Landing Task 3 before Tasks 1 and 2.** Task 3 edits `openapi.go` (shared with Task 2) and `router_openapi.go` (shared with Task 1); it must merge after both to avoid conflict churn.
8. **Bumping the version chip / `package.json` before Task 7.** Same atomic-release discipline as v2.0 task-07 / v2.1 task-06 / v2.2 task-05.
