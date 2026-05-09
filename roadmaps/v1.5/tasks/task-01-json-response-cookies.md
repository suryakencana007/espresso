# Task 1: Cookies on `JSON[T]` Response

**Priority:** 🔴 P0 — Feature
**Estimated Effort:** 0.5 day
**Dependencies:** None
**Closes:** F-05 in `roadmaps/USAGE_ESPRESSO.md` (response side)

## Context

Barista's refresh-token rollout (v0.3, 2026-05-02) discovered that `espresso.JSON[T]` only carries `StatusCode` + `Data` — no field for `Cookies` and no surfaced `http.ResponseWriter` from the handler context. To set an `HttpOnly+Secure+SameSite=Lax` refresh-token cookie alongside the JSON body, Barista wrote a 16-line `httpx.JSONWithCookies[T]` `IntoResponse` wrapper.

The pattern is general enough that every Espresso user who needs to set a cookie alongside a JSON response will hit the same workaround. v1.5 retires it upstream.

The **read side** is fine: `extractor.Cookie[T]` already exists (`extractor/extractor.go:251`) — F-05's "no cookie reader in `extractor.*`" was about a name-keyed reader, but the existing struct-tag-based `Cookie[T]` covers Barista's needs once they migrate. This task is response-side only.

## Acceptance Criteria

- [ ] `JSON[T]` struct gains a `Cookies []*http.Cookie` field.
- [ ] `JSON[T].WriteResponse` writes each cookie via `http.SetCookie(w, c)` **before** `WriteHeader(status)` (Set-Cookie must be in the response head).
- [ ] Zero-value `Cookies` (i.e. `nil`) produces **byte-identical** response output to v1.4 — locked by a regression test.
- [ ] `JSON[T].Reset()` zeroes the `Cookies` slice for `sync.Pool` reuse.
- [ ] Tests cover: no cookies, one cookie, multiple cookies, cookie with expiry, secure cookie.
- [ ] `docs/api/response.md` updated with the cookie pattern.
- [ ] Main `README.md` "Response Types" section gains a cookie example.
- [ ] `cmd/example/` includes a refresh-token-style cookie-setting handler (or augment the existing one).
- [ ] `bench/` JSON round-trip numbers do not regress for the `nil` cookies case.

## Technical Approach

### Step 1.1: Add the field

```go
// response.go

type JSON[T any] struct {
    StatusCode int
    Data       T
    Cookies    []*http.Cookie  // new — written before status code
}
```

Document it in the godoc comment with an example showing a refresh-token cookie set.

### Step 1.2: Modify `WriteResponse`

```go
func (j JSON[T]) WriteResponse(w http.ResponseWriter) error {
    for _, c := range j.Cookies {
        http.SetCookie(w, c)
    }
    w.Header().Set("Content-Type", "application/json")
    status := j.StatusCode
    if status == 0 {
        status = http.StatusOK
    }
    w.WriteHeader(status)
    return sonic.ConfigDefault.NewEncoder(w).Encode(j.Data)
}
```

Order:
1. `Set-Cookie` headers (must be set before `WriteHeader` flushes them).
2. `Content-Type` header.
3. `WriteHeader(status)`.
4. Body.

### Step 1.3: Update `Reset`

```go
func (j *JSON[T]) Reset() {
    j.StatusCode = 0
    var zero T
    j.Data = zero
    j.Cookies = j.Cookies[:0]  // keep backing array, reset length
}
```

If `j.Cookies` is `nil`, the slice operation is a no-op (length is already 0).

### Step 1.4: Backward-compat regression test

```go
func TestJSON_NoCookies_ByteIdenticalToV14(t *testing.T) {
    res := JSON[map[string]string]{Data: map[string]string{"hello": "world"}}
    rec := httptest.NewRecorder()
    if err := res.WriteResponse(rec); err != nil {
        t.Fatal(err)
    }
    // Asserts: no Set-Cookie header in the response, body unchanged from v1.4.
    if got := rec.Header().Values("Set-Cookie"); len(got) != 0 {
        t.Fatalf("unexpected Set-Cookie: %v", got)
    }
    expectedBody := `{"hello":"world"}` + "\n"  // sonic encoder appends newline
    if got := rec.Body.String(); got != expectedBody {
        t.Fatalf("body mismatch: got %q, want %q", got, expectedBody)
    }
}
```

### Step 1.5: Cookie-set tests

```go
func TestJSON_WithCookies(t *testing.T) {
    res := JSON[Empty]{
        Cookies: []*http.Cookie{
            {Name: "refresh", Value: "abc", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode},
        },
    }
    // Assert Set-Cookie header present with expected attributes.
}
```

Cover at minimum: single cookie, multiple cookies, cookie with `Expires`, cookie with `MaxAge`.

### Step 1.6: Documentation

`docs/api/response.md` — add a "Setting Cookies" subsection with the refresh-token example. Cross-link to `extractor.Cookie[T]` for the read side.

`README.md` "Response Types" section — augment the JSON example to show a cookie alongside the body.

### Step 1.7: Example update

`cmd/example/main.go` (or a new `cmd/example/auth/`) — a `/login` handler that sets a session cookie alongside the JSON body. Mirrors the Barista pattern so users can lift it directly.

## Tests Required

- `TestJSON_NoCookies_ByteIdenticalToV14` — backward-compat lock.
- `TestJSON_WithCookies_SetCookieHeader` — single cookie present and well-formed.
- `TestJSON_WithCookies_Multiple` — multiple `Set-Cookie` headers in order.
- `TestJSON_WithCookies_Expires` — cookie with `Expires` field.
- `TestJSON_WithCookies_Reset` — pool reuse zeroes the slice.
- `TestJSON_WithCookies_OrderedBeforeStatus` — verifies Set-Cookie is in the response head, not after the body.

## Definition of Done

- [ ] All acceptance criteria checked.
- [ ] `go test ./... -race` clean.
- [ ] `golangci-lint run ./...` clean.
- [ ] `bench/` framework-comparison numbers within noise of v1.4 for JSON round-trip.
- [ ] CHANGELOG entry under `[Unreleased]` → `Added`: cookies field on `JSON[T]`.
- [ ] PR description references F-05 in `roadmaps/USAGE_ESPRESSO.md`.
