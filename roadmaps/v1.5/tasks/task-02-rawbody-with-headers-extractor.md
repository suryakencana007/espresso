# Task 2: `extractor.RawBodyWithHeaders[H]`

**Priority:** 🔴 P0 — Feature
**Estimated Effort:** 1 day
**Dependencies:** None
**Closes:** F-06 in `roadmaps/USAGE_ESPRESSO.md`

## Context

Webhook receivers need both the raw body bytes (for HMAC verification against the unparsed payload) AND a provider-specific header (`X-Hub-Signature-256` for GitHub, `X-Gitlab-Token` for GitLab, etc.). Espresso's `extractor.RawBody` carries only the body; `extractor.Header[T]` carries only headers; combining them requires a custom `Extract` method.

Barista's v0.4 work shipped two such webhook handlers (`internal/api/handler/webhook.go:24-44`) — each ~35 lines of boilerplate around the same shape. v0.5 confirmed the pattern stays in place under live HMAC load (W-11). v0.6 didn't add new webhook receivers but the pattern is anticipated to recur (Stripe, Slack, observability platforms).

A first-class `extractor.RawBodyWithHeaders[H]` erases the boilerplate while staying out of the way of HMAC verification (body remains `[]byte`, never decoded by the framework).

## Acceptance Criteria

- [ ] New type `extractor.RawBodyWithHeaders[H any]` with fields `Body []byte` and `Headers H`.
- [ ] `Extract(r *http.Request) error` populates both in one read pass, propagating errors as `*espresso.Error`.
- [ ] Required header support via existing `header:"X-Foo,required"` tag — missing required header returns `ErrBadRequest`.
- [ ] `Reset()` zeros both fields for `sync.Pool` reuse.
- [ ] Body length cap respects the `MaxBodyBytes` convention if `RawBody` already enforces one (audit `RawBody` first, mirror its behavior).
- [ ] Tests in `extractor/extractor_test.go` next to existing extractor tests.
- [ ] `docs/api/extractor.md` documents the new extractor with a webhook example.
- [ ] `cmd/example/` includes a webhook receiver that uses the new extractor (or augment the existing example).

## Technical Approach

### Step 2.1: Type definition

```go
// extractor/extractor.go

// RawBodyWithHeaders extracts raw body bytes alongside structured headers.
// Useful for webhook receivers that verify HMAC against the unparsed payload
// while reading provider-specific signature headers.
//
// The Headers type uses the same `header:"Name,required"` tag convention
// as Header[T]. The Body field is the raw request body, unchanged.
//
// Example:
//
//   type GitHubHeaders struct {
//       Signature string `header:"X-Hub-Signature-256,required"`
//       Event     string `header:"X-GitHub-Event,required"`
//   }
//
//   func handle(ctx context.Context, req *extractor.RawBodyWithHeaders[GitHubHeaders]) (espresso.Status, error) {
//       if !verifyHMAC(req.Body, req.Headers.Signature, secret) {
//           return 0, espresso.ErrUnauthorized("invalid signature")
//       }
//       // ...
//   }
type RawBodyWithHeaders[H any] struct {
    Body    []byte
    Headers H
}
```

### Step 2.2: Extract implementation

Read body and headers in one pass. Reuse the existing `extractStructTagsFromHeaders[T]` helper (same one `Header[T]` uses).

```go
func (rb *RawBodyWithHeaders[H]) Extract(r *http.Request) error {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        return espresso.ErrBadRequest("failed to read body").Wrap(err)
    }
    rb.Body = body

    if err := extractStructTagsFromHeaders(&rb.Headers, r.Header); err != nil {
        return err  // already returns *espresso.Error
    }
    return nil
}
```

If `RawBody` has a body-size cap (audit `extractor.RawBody.Extract` first), copy the same cap here.

### Step 2.3: Reset

```go
func (rb *RawBodyWithHeaders[H]) Reset() {
    rb.Body = rb.Body[:0]
    var zero H
    rb.Headers = zero
}
```

### Step 2.4: Tests

- Happy path: raw body + one required header + one optional header.
- Missing required header → `ErrBadRequest`.
- Empty body is allowed (some webhook providers send GETs with empty body).
- Multiple values for a single header (HTTP allows it; document the chosen behavior — first value wins, matching `Header[T]`).
- Pool reuse via `Reset` — second extraction does not see first request's data.
- Concrete provider-shaped tests: GitHub-style (`X-Hub-Signature-256`), GitLab-style (`X-Gitlab-Token`).

### Step 2.5: Example

`cmd/example/webhook/main.go` (new) — a single-file example with a `/webhook/github` route that uses `extractor.RawBodyWithHeaders[GitHubHeaders]` to verify an HMAC signature. Reads HMAC secret from env. Mirror the Barista pattern so users can copy it.

### Step 2.6: Documentation

`docs/api/extractor.md` — add a `RawBodyWithHeaders[H]` section between `RawBody` and `XML[T]`. Include the GitHub webhook example.

Cross-link from the F-06 entry in `roadmaps/USAGE_ESPRESSO.md` once it's marked closed (Task 4 will do the close).

## Tests Required

- `TestRawBodyWithHeaders_Happy` — typical webhook payload.
- `TestRawBodyWithHeaders_RequiredHeaderMissing` — returns `ErrBadRequest` with field info.
- `TestRawBodyWithHeaders_EmptyBody` — empty body allowed, headers still extracted.
- `TestRawBodyWithHeaders_PoolReuse` — `Reset` clears state.
- `TestRawBodyWithHeaders_LargeBody` — if `RawBody` has a cap, mirror the cap test.
- `TestRawBodyWithHeaders_GitHubShape` / `TestRawBodyWithHeaders_GitLabShape` — concrete provider headers.

## Definition of Done

- [ ] All acceptance criteria checked.
- [ ] `go test ./extractor/... -race` clean.
- [ ] `golangci-lint run ./extractor/...` clean.
- [ ] CHANGELOG entry under `[Unreleased]` → `Added`: `extractor.RawBodyWithHeaders[H]`.
- [ ] PR description references F-06 in `roadmaps/USAGE_ESPRESSO.md`.
- [ ] Webhook example runs against a curl HMAC call documented in its README.
