# Task 7: Lower `go.mod` Directive to 1.23 + Fix Gosec G115

**Priority:** ⚪ Cleanup
**Estimated Effort:** 0.5 day
**Dependencies:** None

## Context

Two unrelated cleanups bundled because they're both small and both touch toolchain-level concerns:

1. **`go.mod` declared `go 1.25.6`** but Espresso doesn't actually use any post-1.23 language or stdlib feature. The high directive locked out users on Go 1.23 and 1.24 toolchains for no benefit.
2. **Gosec flagged G115** ("integer overflow conversion int → rune") on two test-only sites where we did `string(rune(intVar))` to embed an integer in a path. Cosmetic in test code, but `golangci-lint run ./...` was non-clean.

## Acceptance Criteria

- [x] `go.mod` declares `go 1.23` (down from `go 1.25.6`).
- [x] Full test suite passes under Go 1.23 toolchain.
- [x] `golangci-lint run ./...` clean under Go 1.23.
- [x] Two G115 hits fixed in `handler_test.go` and `sse_test.go`.
- [x] `mise.toml` continues to pin the dev toolchain at the latest stable Go (developer convenience; not the same as the module's minimum).

## Technical Approach

### Step 7.1: Verify codebase is 1.23-compatible

Grep for usage of post-1.23 features:

- `unique.Handle` (1.23+) — check for `unique.Make`.
- `slices.Sorted` / `slices.Sort` overloads added post-1.21.
- `iter.Seq` ranger functions — check for `range func()` patterns.
- New stdlib types in `os`, `sync`, `time` post-1.23.

If any usage exists, either keep `go 1.25` and stop, or replace with a 1.23-compatible alternative. If none, proceed.

### Step 7.2: Lower the directive

```diff
-go 1.25.6
+go 1.23
```

Run `go mod tidy` — should be a no-op for `require` since dependencies haven't changed.

### Step 7.3: Verify under 1.23

```bash
go test ./... -race
golangci-lint run ./...
```

Both must pass under a 1.23 toolchain. If `golangci-lint` is configured for a different version, update `.golangci.yml`'s `run.go: "1.23"`.

### Step 7.4: G115 fix

Two test-only sites (`handler_test.go`, `sse_test.go`) build a path/ID string from an integer using `string(rune(x))`. That can produce non-ASCII codepoints when `x > 127`, hence the G115 flag.

Replacement options:

```go
// Before
path := "/users/" + string(rune(id))

// After (numeric id)
path := "/users/" + strconv.FormatInt(int64(id), 10)
```

If the test wants a printable ASCII char specifically, use a slice lookup:

```go
var letters = []byte("abcdefghijklmnopqrstuvwxyz")
ch := string(letters[id%len(letters)])
```

Pick whichever matches the test's intent.

## Tests Required

Existing tests under Go 1.23.

## Definition of Done

- [x] `go.mod` reads `go 1.23`.
- [x] `go test ./... -race` clean under Go 1.23.
- [x] `golangci-lint run ./...` clean — no G115 hits.
- [x] CHANGELOG `[Unreleased]` → `Changed` (go directive) and `Fixed` (G115).
- [x] `mise.toml` left alone (or bumped to the latest stable separately; not part of this task).
