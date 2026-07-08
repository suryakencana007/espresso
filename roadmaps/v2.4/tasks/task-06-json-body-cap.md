# Task 6: Bound JSON[T] Body Extraction

**Priority:** 🔴 P0 — Must Have
**Estimated Effort:** 1 day
**Dependencies:** None

> **Status: ✅ Shipped 2026-07-05 (v2.4.0).** Delivered via #70 — internal/bodylimit leaf carries CtxKey/Middleware/ErrBodyTooLarge/ReadAllLimited; Router.WithJSONBodyLimit + ErrRequestEntityTooLarge added.

## Context

`JSON[T].Extract` (`response.go:96-102`) is unbounded:

```go
func (j *JSON[T]) Extract(r *http.Request) error {
    defer func() { _ = r.Body.Close() }()
    if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&j.Data); err != nil {
        return err
    }
    return RunDefaultValidator(&j.Data)
}
```

A client can stream an arbitrarily large body into sonic's internal buffer; `http.Server` does not cap body size by default. Meanwhile `http.go:14-45` documents:

> "Memory limits for safe JSON decoding ... prevent memory exhaustion attacks"

with `MaxPayloadSize=1MB` and a pooled buffer (`http.go:62-76`), but `DecodeSafeJSON` is used only by `cmd/example/main.go:111` and its own tests — no framework extractor calls it. The advertised safety property does not hold on the path every JSON handler actually uses.

Same exposure applies to `extractor.RawBody` and `extractor.RawBodyWithHeaders` (both call `io.ReadAll` uncapped — audit `extract-resp-pool#5`), and to `extractor.XML[T]` (uncapped decode). Bundle all three under one knob.

## Acceptance Criteria

- [x] `JSON[T].Extract` returns `413 Payload Too Large` with the canonical envelope when body exceeds the configured cap. Below the cap, behavior unchanged.
- [x] `extractor.RawBody.Extract` and `extractor.RawBodyWithHeaders.Extract` respect the same cap.
- [x] `extractor.XML[T].Extract` respects the same cap.
- [x] Cap is configurable per-router via a new `WithJSONBodyLimit(n int64)` option (or a more general `WithBodyLimit(n int64)` that also covers `RawBody`/`XML` — see Step 6.3 for the naming decision).
- [x] Default cap is `http.MaxPayloadSize` (1 MB, defined in `http.go`).
- [x] A new `espresso.ErrRequestEntityTooLarge(msg)` constructor is added (matching the `Err*` naming pattern in `error.go`); status 413, code `PAYLOAD_TOO_LARGE`.
- [x] Regression test: body of `cap+1` bytes returns 413 with canonical envelope; body at exactly `cap` succeeds.
- [x] No breaking change to `JSON[T].Extract` return signature; the cap error is a normal `error` return, translated by the existing error path to 413.

## Technical Approach

### Step 6.1 — Add ErrRequestEntityTooLarge

In `error.go`, add:

```go
// ErrRequestEntityTooLarge returns a 413 Payload Too Large error.
// The message should describe the size limit that was exceeded.
func ErrRequestEntityTooLarge(message string) *Error {
    return &Error{StatusCode: 413, Code: "PAYLOAD_TOO_LARGE", Message: message}
}
```

Also add `413` to `defaultCodeForStatus` (`error.go:536-557`) so `NewError(413, msg)` gets `PAYLOAD_TOO_LARGE` instead of `ERROR` (audit `api-surface#11`). This also fixes finding #11 as a bonus.

### Step 6.2 — Extract a shared limited-decode helper

`DecodeSafeJSON` in `http.go` already does the right thing (`io.LimitReader` + `io.ReadAll` into pooled buffer + `sonic.Unmarshal`). Refactor:

```go
// http.go
func DecodeSafeJSONLimit(r *http.Request, v any, limit int64) error {
    defer r.Body.Close()
    lr := &io.LimitedReader{R: r.Body, N: limit + 1}
    buf := getPooledBuf(); defer putPooledBuf(buf)
    n, err := buf.ReadFrom(lr)
    if err != nil { return err }
    if n > limit { return ErrRequestEntityTooLarge(fmt.Sprintf("body exceeds %d bytes", limit)) }
    return sonic.Unmarshal(buf.Bytes(), v)
}
```

`ErrRequestEntityTooLarge` returns `*espresso.Error`, which the existing extractor-error path translates to the canonical 413 JSON envelope via `writeHandlerError` (`error.go:197-233`).

`DecodeSafeJSON` (unlimited-signature callers) can wrap `DecodeSafeJSONLimit(r, v, MaxPayloadSize)`.

### Step 6.3 — Route JSON[T].Extract through the safe path

```go
func (j *JSON[T]) Extract(r *http.Request) error {
    limit := jsonBodyLimitFrom(r.Context()) // pulled from router option, default MaxPayloadSize
    if err := DecodeSafeJSONLimit(r, &j.Data, limit); err != nil {
        return err
    }
    return RunDefaultValidator(&j.Data)
}
```

Add a `WithJSONBodyLimit(n int64)` router option that injects `limit` into the request context (via a package-private ctx key) at the router-middleware level. Default: `MaxPayloadSize` (1 MB). Users can raise or lower per-router.

Naming decision: prefer `WithJSONBodyLimit` (specific) over `WithBodyLimit` (generic), so the future addition of a separate limit for `RawBody`/`XML` doesn't collide. Document that `RawBody` and `XML` share this limit today; a future release may split.

### Step 6.4 — Cap RawBody and XML

`extractor.RawBody.Extract` (`extractor/extractor.go:304`) currently does `io.ReadAll(r.Body)`. Change to `io.ReadAll(io.LimitReader(r.Body, limit+1))` with the same over-limit check and `ErrRequestEntityTooLarge` return. Same for `RawBodyWithHeaders`.

`extractor.XML[T].Extract` (`extractor/extractor.go:386`) — same pattern.

### Step 6.5 — Fix the false pooling comment (coordinate with Task 10)

`response.go:71-74` claims:

```go
// Use pooled buffer for encoding to reduce allocations
// For small responses, direct encoding is faster
// For large responses, buffered encoding reduces GC pressure
```

None of this is actually pooled encoding — the code is `sonic.ConfigDefault.NewEncoder(w).Encode(...)`. Task 10 owns the comment fix. If Task 6 lands first, coordinate on the CHANGELOG entry with Task 10.

## Tests Required

- `TestJSON_ExtractRejectsOverLimit`: body of `cap+1` bytes → 413 with canonical envelope `{"error":{"code":"PAYLOAD_TOO_LARGE",...}}`.
- `TestJSON_ExtractAcceptsAtLimit`: body at exactly `cap` bytes succeeds.
- `TestJSON_ExtractRespectsRouterOption`: `WithJSONBodyLimit(500)` router rejects a 501-byte body but accepts a 500-byte body.
- `TestRawBody_ExtractRejectsOverLimit`: same for `extractor.RawBody`.
- `TestXML_ExtractRejectsOverLimit`: same for `extractor.XML[T]`.
- `TestDefaultLimitIsMaxPayloadSize`: routers without the option default to 1 MB.
- Run with `-race`.

## Definition of Done

- [x] All Acceptance Criteria checkboxes ticked.
- [x] `go test -race ./... -count=2` clean.
- [x] `golangci-lint run ./...` clean.
- [x] `govulncheck ./...` clean.
- [x] CI's `Test (race)` job green on the PR.
- [x] CHANGELOG `[Unreleased]` entry under `Added`: `espresso.WithJSONBodyLimit(n int64)` router option; `espresso.ErrRequestEntityTooLarge(msg)` error constructor; `413` added to `defaultCodeForStatus`. Under `Fixed`: `JSON[T].Extract`, `extractor.RawBody`, and `extractor.XML[T]` now respect a configurable body-size cap (default 1 MB); previously they decoded arbitrarily large bodies into memory.
- [x] Migration note (Task 12): callers hit 413 on bodies over 1 MB by default; raise via `WithJSONBodyLimit(higher)` per router.
- [x] No breaking change to any existing extractor signature.
