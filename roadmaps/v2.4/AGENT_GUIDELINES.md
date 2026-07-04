# Guidelines for AI Agents — v2.4

Read this file **in addition to** `roadmaps/v1.3/AGENT_GUIDELINES.md` (the baseline), `roadmaps/v1.4/AGENT_GUIDELINES.md`, `roadmaps/v1.5/AGENT_GUIDELINES.md`, `roadmaps/v2.0/AGENT_GUIDELINES.md`, `roadmaps/v2.1/AGENT_GUIDELINES.md`, `roadmaps/v2.2/AGENT_GUIDELINES.md`, and `roadmaps/v2.3/AGENT_GUIDELINES.md`. Most rules carry forward unchanged. This file only lists what is **different** for v2.4.

## What's Different in v2.4

### 1. Every P0 Fix Is Locked by a Test Shaped Like the Audit's Repro

The 2026-07-02/03/04 audit reproduced each P0 finding with a specific goroutine/timing/traffic pattern — a `-race` run against the exact abandon-and-recycle scenario for Task 1, a request-per-100ms-for-3s loop for Task 2, a `LoggingMiddleware` + SSE route for Task 3, a `signal.NotifyContext` + in-flight request for Task 4, a concurrent `AddPath` + `ServeHTTP` for Task 5, a large body for Task 6. **The regression test must be that same shape**, not a paraphrase of the fix. A green unit test that only exercises the fixed function in isolation would not have caught the original bug, so it does not lock the fix. Every P0 task file names the repro shape explicitly under `## Tests Required`. If the test does not reliably fail on the pre-fix code, it does not lock the fix.

### 2. Truth-in-Docs Is a First-Class Axis, Not a Follow-Up

Task 10 is P1, not `docs`, because doc drift is what let five of the six P0 findings sit in the code long enough to ship: `Generator` godoc claimed race-safety it did not have, `WSConfig.ReadTimeout` godoc claimed a timeout it never enforced, `TimeoutLayer` godoc did not mention retention hazards, `middleware/index.md` documented execution order backwards, and the "zero-allocation handlers" claim across README/docs/index/core.go was contradicted by measurement on every path. When you change behavior in Tasks 1-9, **update the doc comment in the same PR** if the code diverges from it — do not push the doc drift into Task 10. Task 10 owns the docs whose behavior is not changing but whose text is wrong; it is not a catch-up commit for the code tasks.

### 3. `-race` Is Now CI-Enforced — Local `-short` Runs Do Not Substitute

PR #58 wired `go test -race -shuffle=on ./...` on `ubuntu-latest` on every push. Two SSE shutdown races (`TestShutdown_SSEStreamsClosed`, `TestShutdown_MultiRouter_SSEIsolation`) that existed pre-#58 only reproduced on Linux with full-suite ordering; local Windows `-short` runs missed them. **Run `go test -race -count=1 -shuffle=on ./...`** (no `-short`) locally before pushing, and expect Linux CI to catch anything the local run missed. This is the standard the framework holds itself to now — a task is not done until the CI `-race` job is green on the PR.

### 4. Do Not Bundle a JWT Rewrite or a File-Content Extractor Into v2.4

Two audit findings are deliberately out of scope (see README `Related, Deferred`): real JWT cryptographic validation, and `FileInfo` exposing uploaded content. Both are **feature additions**, not correctness fixes — they need dependency choices (a JWT library), API design decisions (an `Open()` method vs a request-in-context escape hatch), and their own doc/migration story. v2.4 is a correctness cleanup; adding either quietly is scope creep. If a task's PR touches those files, only touch what the task requires.

### 5. `internal/errorenvelope` and `internal/validatehook` Remain the Only Cross-Package Leaves

v2.2 introduced `internal/errorenvelope` for the canonical error envelope shared between `middleware/http` and `openapi`; v2.3 reused it for the OpenAPI failure path. The discipline stands: when a shared helper is needed by more than one package the root already depends on, add it as a **stdlib-only leaf under `internal/`** — never introduce a back-edge that has the leaf importing root. If Task 6's body-cap wiring reveals a need for a shared "safe reader" helper, it lives in `http.go` (already there via `DecodeSafeJSON`) or a new `internal/` leaf, not in root.

### 6. Task 4 Fixes cmd/example/sse and cmd/example/websocket in the Same PR

The audit confirmed both `cmd/example/sse/main.go:51-57` and `cmd/example/websocket/main.go:53-59` run `router.Brew(...)` in a goroutine and block main on their own `signal.NotifyContext` for the same signals `Brew` traps — racing the graceful shutdown. Task 4 owns the framework fix (`BrewContext` detach-cancellation + `SetWriteDeadline` for SSE) **and** the two example programs, since keeping them broken while fixing the framework would ship a v2.4 whose flagship examples still demonstrate the anti-pattern. The examples should use the plain `router.Brew(...)` pattern (blocks on signals internally) or the documented `BrewContext(ctx, ...)` pattern with `signal.NotifyContext`.

## Carried Over From Earlier Versions

Re-read the prior guidelines if you haven't recently. The following remain unchanged and not repeated here:

- Read before writing (`handler.go`, `core.go`, `error.go`, `router.go`, extractor patterns).
- Coffee metaphor for any new public surface (v2.4 has essentially none; Task 6's body-cap knob is exposed via existing `http.go` `MaxPayloadSize` and a router option — no new coffee-named public symbols expected).
- Type-safety over `any`.
- `context.Context` mandatory on I/O.
- `sync.Pool` for hot paths; atomic over mutex on hot paths.
- Prefer correcting behavior over adding surface — when generator output and reality disagree, fix the generator and lock it with a test (v2.3 rule 7 carries over).
- Conventional Commits, small focused commits, feature-branch + PR (UI merge — branch protection blocks CLI merge).
- `cmd/example/` updates when user-facing behavior changes.
- Race detector mandatory; `golangci-lint` (gocyclo min 15) clean; `govulncheck` clean.

## Common Mistakes to Avoid (v2.4-specific)

1. **Writing a regression test that would not have failed on the pre-fix code.** The audit's repro is the standard. If your test passes both before and after your patch, it does not lock the fix.
2. **Papering over the TimeoutLayer race by making `Reset()` "atomic".** The race is not about `Reset` — it is about the outer handler `pool.Put`-ing a struct that an abandoned goroutine still holds. The fix is to skip `pool.Put` on abandonment (or make `TimeoutLayer` synchronous), not to make the reset safer. Both approaches produce a green `-race`; only one is correct.
3. **Documenting the "zero-allocation" claim as a `sync.Pool` performance property.** The framework is not zero-allocation on any handler path (measured 2 allocs/op best case for Ristretto static text). Task 10 rewords this to the defensible pooled-request claim — do not paraphrase-preserve it.
4. **Fixing `TokenBucketLimiter` by "computing refill in floats" without touching `lastRefill`.** The bug is the pair — `int(elapsed.Seconds())` truncates AND `lastRefill = now` advances the clock past the un-credited fractional-second gap. Fix by advancing `lastRefill` only by the credited time (`lastRefill.Add(time.Duration(whole)*time.Second)`) or by doing the math in nanoseconds; do not fix one half without the other.
5. **Making `statusRecorder` embed `http.ResponseWriter` and calling it done.** Task 3 requires the full `http.ResponseController` unwrap protocol: `Flush()` (forward), `Hijack()` (forward), `Push()` (forward), `Unwrap() http.ResponseWriter`. `gzipResponseWriter` in the same file is the reference pattern.
6. **Fixing `BrewContext` by ignoring its parent context.** The fix is `context.WithoutCancel` on the shutdown context, so `srv.Shutdown` gets a fresh timeout while `OnShutdown` hooks still see a live ctx. Skipping the parent entirely breaks the case where the shutdown itself times out.
7. **Locking the OpenAPI generator with a coarse `g.mu` around every method.** The mutation methods each write different substructures; a single global mutex on the mutation side while `cachedJSON` reads under the same mutex creates read/write contention. The right shape is: hold `g.mu` across the mutation **and** the invalidateCache within the same critical section — the existing structure already has `invalidateCache` inside `mu`, so the fix is folding the mutation into that critical section, not adding a second lock.
8. **Documenting the JSON body cap as "1 MB always".** Users need a knob. Task 6 wires `MaxPayloadSize` and exposes it via a router option so it is configurable per-router; the default is 1 MB per `http.go`, but the number is not the contract — the knob is.
9. **Bumping the version chip / `package.json` before Task 12.** Same atomic-release discipline as v2.0 task-07 / v2.1 task-06 / v2.2 task-05 / v2.3 task-07.
