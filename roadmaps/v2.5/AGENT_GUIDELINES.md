# Guidelines for AI Agents — v2.5

Read this file **in addition to** `roadmaps/v1.3/AGENT_GUIDELINES.md` (the
baseline) and the delta files for v1.4, v1.5, v2.0, v2.1, v2.2, v2.3, and
v2.4. Most rules carry forward unchanged. This file only lists what is
**different** for v2.5.

## What's Different in v2.5

### 1. This Release Was Issue-Driven, Not Audit-Driven

v2.0–v2.4 started from an audit or a planned capability and wrote the roadmap
first. v2.5 started from two filed issues (#76, #77) and a CI failure that
became issue #82. The consequence for an agent: **the issue thread is the
specification.** Before touching a task, read the linked issue in full —
acceptance criteria, the concrete downstream case (tamper#163 for #77), and
any decision the issue left open (any-header vs allowlist for #77). The task
file records what was decided; the issue records why.

### 2. Additive Only — a Minor Release May Not Move Existing Surface

v2.5 is a SemVer **minor**, not a major: new public symbols are allowed, but
no existing signature, behavior contract, or wire format may change except by
fixing a defect. When a fix changes observable behavior (the circuit-breaker
probe bound in Task 2), it is a **fix**, is documented as such, and is called
out in the migration guide — it is not treated as an additive footnote. Do not
rename, reorder, or re-type anything an existing app compiles against.

### 3. A Fix Is Not Locked Until the Regression Test Fails on the Pre-Fix Code

v2.4 rule 1 said the regression test must be the audit's repro shape. v2.5
tightens it: **prove the test fails before the fix.** Task 2's regression test
was validated by checking out the unfixed `layer.go`, running the new test,
and watching it fail deterministically (`round 0: … got 2`). A test that only
passes after the fix and was never run against the broken revision does not
lock anything. If the test is timing-dependent, repeat the scenario behind a
start barrier until the pre-fix failure is reliable — do not ship a test that
trips the bug "once per few hundred runs", which is how the defect survived
v2.4.

### 4. The Error Path Is a Contract, and Its Tests Are Byte-Level

Task 1 touches `writeErrorResponse`, which every error origin funnels through
(handler return, extractor failure, translated service-layer error, panic
recovery, auth, rate limit, SSE/WebSocket setup). Two rules follow. First,
**the zero-header response must stay byte-identical** — `TestError_WithHeader_NoHeaderUnchanged`
asserts the exact body string; keep that guard green. Second, **`Content-Type`
stays writer-owned**: `applyErrorHeaders` runs before `errorenvelope.Write`,
and the reserved-header denylist is defense-in-depth, not the only protection.
Do not move header application after the status line, and do not add a header
path that bypasses `errorenvelope.Write`.

### 5. CI Had to Be Un-Red Before Anything Else Could Be Verified

When this cycle started, `govulncheck` was red on **every** branch because CI
pinned Go 1.25 while the standard library advisories were fixed only in Go
1.26.9+. Task 4 bumps the test and vulncheck jobs to Go 1.27. **Do not "fix"
the red by suppressing the check or pinning `govulncheck` older** — the
toolchain was the problem, and moving to the toolchain the release targets
(Go 1.27, the subject of issue #76) is the fix. The lint job intentionally
stays on Go 1.25 because golangci-lint is pinned to a version that predates
Go 1.27 and bumping it would surface unrelated pre-existing findings.

## Carried Over From Earlier Versions

Re-read the prior guidelines if you have not recently. The following remain
unchanged and are not repeated here:

- Read before writing (`error.go`, `middleware/service/layer.go`, the
  envelope writer, existing tests).
- Coffee metaphor for any new public surface.
- Type-safety over `any`.
- `context.Context` mandatory on I/O.
- `sync.Pool` for hot paths; atomic over mutex on hot paths.
- Structured errors and the canonical JSON envelope are the contract.
- Conventional Commits, small focused commits, feature branch + PR.
- Race detector mandatory; `golangci-lint` (gocyclo min 15) clean;
  `govulncheck` clean.
- Single atomic release commit; version chips and migration guide land
  together (v2.0 task-07 … v2.4 task-12).

## Common Mistakes to Avoid (v2.5-specific)

1. **Denying a header with an allowlist when a denylist is the right shape.**
   Issue #77 left "any-header vs allowlist" open. The chosen answer is a
   denylist of framing + hop-by-hop headers: `Retry-After`,
   `WWW-Authenticate`, `Set-Cookie`, `Link`, and `X-*` are all legitimate on
   an error response, and an allowlist would reject them. Keep the list to
   headers that actually corrupt the response.
2. **Forgetting that `WithHeader` is Set and `WithHeaderAdd` is Add.** Issue
   #77 proposed only `WithHeader`; review flagged that Set-only cannot emit
   two `Set-Cookie` or two `WWW-Authenticate` values, so `WithHeaderAdd`
   mirrors `http.Header.Add`. A later `WithHeader` collapses `WithHeaderAdd`
   values — document it, don't "fix" it.
3. **Guarding the error-header write behind a nil check that hides a bug.**
   `applyErrorHeaders` assumes a non-nil `*Error`; every caller already
   guarantees one. If you ever route a nil error through
   `writeErrorResponse`, fix the caller, not the writer.
4. **Fixing the circuit breaker by making the probe counter reset "less
   often".** The defect is that the `Open → HalfOpen` transition is
   unconditional. The fix re-checks `state.state == StateOpen` **and** the
   elapsed timeout under the write lock so exactly one goroutine owns the
   transition and the `Store(0)`. Do not paper over it by moving the reset
   elsewhere; the atomic `Add` must stay the only admission gate.
5. **Restructuring the entry gate to hold the write lock across the whole
   call.** The probe-slot acquisition stays on the atomic counter for the hot
   path. Re-check state under the lock, release it, then `Add` — do not
   serialize every request behind `state.mu`.
6. **Bumping the version chip, `package.json`, or the sonic pin outside
   Task 7's atomic release commit.** Same discipline as every prior release.
