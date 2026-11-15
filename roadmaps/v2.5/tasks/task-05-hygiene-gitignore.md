# Task 5: Ignore toolchain `snip-*` test binaries

**Priority:** ⚪ Cleanup
**Estimated Effort:** 0.1 day
**Dependencies:** None

> **Status: ✅ Shipped 2026-11-15 (v2.5.0).** Delivered via #80.

## Context

The local Go toolchain shim emitted random-suffixed test binaries named
`snip-<digits>` into the working directory on every test run — 39 files
accumulated during the v2.5 cycle. They are build artifacts, not source, but
they showed up as untracked files and invited accidental staging.

## Acceptance Criteria

- [x] `snip-*` ignored.
- [x] A plausible-looking tracked file is not silently ignored in error.

## Technical Approach

### Step 5.1 — Add the pattern

`.gitignore`, next to the existing `*.test` entry:

```
# Random-suffixed test binaries emitted by the local toolchain shim
snip-*
```

Verified with `touch snip-9999` — no longer appears in `git status`.

## Tests Required

None.

## Definition of Done

- [x] `git status` clean after a test run.
