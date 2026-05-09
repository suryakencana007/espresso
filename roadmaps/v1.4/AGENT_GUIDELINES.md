# Guidelines for AI Agents — v1.4

This document applies to any AI agent (Claude Code, Cursor agent, GitHub Copilot Workspace, etc.) working on Espresso v1.4.0 tasks. Read this file **in addition to** `roadmaps/v1.3/AGENT_GUIDELINES.md` — most of those rules still apply. This file only lists what is **different** for v1.4.

## What's Different in v1.4

### 1. Hardening Beats Features

v1.3 added four major capabilities (WebSocket, SSE, structured errors, graceful shutdown). v1.4 stops adding for a beat and pays back the debt those features incurred:

- Long-lived connection tests landed in v1.3 — they exposed real races that were never reachable from short-lived unit tests.
- The new structured-error pipeline only covered the handler return path; extractor errors still fell through to `http.Error()` text/plain.
- The service-layer surface advertised `Validation(any)` but did not ship a concrete validator.

Treat hardening as the headline. New features (validator, bench module) exist only because they close standing TODOs (#7 and #10) and are bounded.

### 2. Backward Compatibility Is Still Strict

v1.4 is a **minor** bump. Every v1.3 caller must continue to compile and behave the same way, with one documented exception:

- **Extractor failures now return JSON instead of text/plain.** This is a wire-format change, but the status code is unchanged and most clients treat 4xx as an error regardless of body shape. Document under "Migration Notes" in the CHANGELOG.

If you spot another wire-format-affecting change in your task, stop and ask. Wire-format is the line.

Renames, removals, and signature changes are still off the table for v1.4. The v2.0 roadmap (`roadmaps/v2.0/`) is where those go.

### 3. Race Detector Is Mandatory, Not Aspirational

Every task that touches `sse.go`, `websocket.go`, or anything they call into must run `go test ./... -race` clean. The v1.3 long-lived tests already exposed two races (`WS.closed` plain-bool reads in handler-end guards, channel-send blocking after handler return). Assume there are more; chase any new failures all the way down.

### 4. New Subpackages Live at the Root

`validator/` and `bench/` are added as **siblings** of `extractor/`, `middleware/`, `pool/`, `openapi/`. Do not nest them. `bench/` is a separate Go module with a `replace` directive back to the parent — do this so the comparison-framework deps (gin, echo, fiber, fasthttp, …) never pollute the main module's `go.sum`.

### 5. Documentation Lands With the Code

Every user-facing addition in v1.4 ships a `docs/` page in the same PR:

- Validator → `docs/guide/validation.md` + `docs/api/validator.md`.
- Bench numbers → README "Framework Comparison" table + `bench/README.md` methodology.
- Handler-cache growth → `handler.go` doc comment + `docs/performance.md` "Known Limitations".

If you can't write the doc in the same PR, you don't understand the API well enough to ship it.

## Carried Over From v1.3

Re-read v1.3's guidelines if you haven't recently. The following are unchanged and not repeated here:

- Read before writing (handler.go, core.go, state.go, router.go, extractor patterns).
- Coffee metaphor for any new public surface.
- Type-safety over `any`.
- `context.Context` is mandatory on I/O.
- Conventional Commits, small focused commits.
- `cmd/example/` updated when user-facing APIs change.
- Files NOT to touch unless explicitly scoped.

## Common Mistakes to Avoid (v1.4-specific)

1. **Adding the validator's struct tags to existing extractors in this release.** That's the v2.0 "auto-validate on extract" task (`roadmaps/v2.0/tasks/task-05-auto-validate-on-extract.md`). v1.4 ships the validator only; opt-in integration is v2.0.
2. **Letting bench-module deps leak into the main `go.sum`.** Always work inside `bench/` for comparison benchmarks; never `go get` gin/echo/fiber from the root.
3. **Fixing a race "by sprinkling a Mutex".** Reach for `atomic.*` first when the data is a single word and the access pattern is read-mostly. The `WS.closed bool → atomic.Bool` migration is the template.
4. **Bundling deprecation removal into v1.4.** v2.0 is the place for `Deprecated:` cleanup. Do not preempt it; the v2.0 roadmap is already scoped.
