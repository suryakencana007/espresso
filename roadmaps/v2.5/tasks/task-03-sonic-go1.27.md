# Task 3: sonic v1.15.3 — Go 1.27 support

**Priority:** 🟡 P1 — hardening
**Estimated Effort:** 0.5 day
**Dependencies:** None

> **Status: ✅ Shipped 2026-11-15 (v2.5.0).** Delivered via #78, closing
> issue #76.

## Context

espresso v2.4.0 pinned `github.com/bytedance/sonic v1.15.0`, which does not
support Go 1.27. Any consumer building with Go 1.27.x gets a warning on every
process start and sonic silently falls back to `encoding/json`, so
`JSON[T].WriteResponse` loses its fast path:

```
WARNING: sonic/ast only supports (go1.17~1.26 and amd64 CPU) or (go1.20~1.26 and arm64 CPU), but your environment is not suitable and will fallback to encoding/json
```

The warning fires at package init, so it prints even for a `version` command,
before any request, and lands in pod logs interleaved with zerolog output.
Found while scaffolding **resi** (a new espresso app on Go 1.27.0); **barista**
was unaffected only because it pins Go 1.26.3.

## Acceptance Criteria

- [x] `sonic` ≥ v1.15.3 in both modules (root and `bench/`).
- [x] No process-start warning and no fallback under Go 1.27.
- [x] Encoded JSON output unchanged.
- [x] Existing minimum-Go compatibility preserved.

## Technical Approach

### Step 3.1 — Bump in both modules

```sh
go get github.com/bytedance/sonic@v1.15.3 && go mod tidy          # root
cd bench && go get github.com/bytedance/sonic@v1.15.3 && go mod tidy
```

Result: `sonic v1.15.0 → v1.15.3`, `sonic/loader v0.5.0 → v0.5.2`. sonic's own
`go.mod` requires Go 1.18, so the root `go 1.23` directive still binds — the
bump does not raise the minimum toolchain.

### Step 3.2 — Verify the warning is actually gone

The warning is emitted at package init, so it does **not** appear under
`go test`. Verify with a real binary:

```sh
GOTOOLCHAIN=go1.27.4 go run ./cmd/example   # main (v1.15.0): prints WARNING
GOTOOLCHAIN=go1.27.4 go run ./cmd/example   # fix  (v1.15.3): silent
```

## Tests Required

- No new unit test — a missing init-time warning is not expressible as one.
  The verification is the `go run` check above plus the existing JSON tests.

## Definition of Done

- [x] Both `go.mod`/`go.sum` pairs updated.
- [x] `go test ./...` and `bench` build green on Go 1.25 and Go 1.27.4.
- [x] CHANGELOG `[2.5.0]` `Fixed` entry references #76.
