# Guidelines for AI Agents — v2.0

This document applies to any AI agent (Claude Code, Cursor agent, GitHub Copilot Workspace, etc.) working on Espresso v2.0.0 tasks. Read this file **in addition to** `roadmaps/v1.3/AGENT_GUIDELINES.md` — most of those rules still apply. This file only lists what is **different** for a major-version bump.

## What's Different in v2.0

### 1. Breaking Changes Are Allowed — But Each Must Be Justified

v1.3 rule was "no breaking changes." v2.0 flips that: breaking is the point. But the bar is higher, not lower:

- Every breaking change must live in a specific task file.
- Every task that breaks API must produce a paired migration-guide entry (Task 6 collects these).
- "Cleanup for its own sake" is out of scope. Remove an API only if:
  - It was marked `// Deprecated:` in v1.x, **or**
  - It has zero callers inside the repo **and** the replacement is documented.

If you notice a potential cleanup that isn't already in a task file, open a discussion before acting — don't bundle unscoped breakage into an existing task.

### 2. Module Path Change

Per Go's module convention, a v2 major release changes the import path:

```
github.com/suryakencana007/espresso        →   v1.x
github.com/suryakencana007/espresso/v2     →   v2.x
```

This happens as part of Task 7 (release). Until Task 7 lands, work on the v1-path module; Task 7 is a single atomic commit that bumps `go.mod`, rewrites every `github.com/suryakencana007/espresso` import, and tags `v2.0.0`.

Do **not** do partial rename commits. One commit, whole repo.

### 3. Migration Guide Is a First-Class Deliverable

Every breaking change must write its migration recipe as part of the same PR that introduces the break — not later, not in Task 6 catch-up.

Recipe format:

```markdown
### Renamed: `NewBadRequest` → `ErrBadRequest`

**Before (v1):**
```go
err := espresso.NewBadRequest("invalid id")
```

**After (v2):**
```go
err := espresso.ErrBadRequest("invalid id")
```

**Mechanical fix:**
```bash
gofmt -r 'espresso.NewBadRequest(x) -> espresso.ErrBadRequest(x)' -w .
```
```

If you can't write the mechanical fix, say so and provide a manual recipe instead.

### 4. Downstream Projects Track v2 Separately

Barista (the PaaS downstream of Espresso) pins v1.x until v2.0 ships. Do **not** assume Barista compiles against work-in-progress v2 code. Do not make "Barista-friendly" compromises during v2 development — if Barista needs to migrate, that's Barista's job.

### 5. No "Compatibility Shims"

A common temptation in major bumps is to keep the old API as a thin shim that calls the new one. Resist. Shims:

- Turn into permanent load-bearing code
- Defeat the purpose of the major bump
- Create two paths to the same behavior, confusing users

If you think a shim is warranted, justify it in the task PR. Default is: remove it, document the migration.

## What's the Same

Read `roadmaps/v1.3/AGENT_GUIDELINES.md` for rules that carry over unchanged:

- Read existing code before writing new code
- Coffee metaphor for new surface
- Type-safety over flexibility
- Context mandatory on I/O
- Testing requirements (80%+ coverage, `-race`, table-driven)
- Conventional Commits
- Small, focused commits
- Performance benchmarks on hot paths

## Commit Scopes New for v2.0

Add to the scope list in v1.3's guidelines:

- `v2` — release engineering and migration-guide work
- `deprecated` — removals only (pairs with explicit task in v2 scope)

## Files You Should NOT Touch

Same as v1.3 guidelines, plus:

- `roadmaps/v1.3/` — historical record; keep frozen
- `bench/go.mod` — the separate bench module does not depend on v1 vs v2 module path; do not modify its `replace` line during v2 work except in Task 7

## What To Do When Stuck

Same escalation path as v1.3. One addition for v2: if a breaking change turns out to be wider than its task scope, **stop and split the task** rather than expanding a single PR into a monster. A "migrate-half-of-everything" PR is not reviewable.
