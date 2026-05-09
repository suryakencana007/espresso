# Task 4: CHANGELOG & v1.5.0 Release

**Priority:** 📦 Meta
**Estimated Effort:** 0.25 day
**Dependencies:** Tasks 1, 2, 3 must be complete

## Context

Final gate before tagging v1.5.0. Consolidate changes from Tasks 1-3 into a coherent CHANGELOG entry, run quality gates, tag the release, publish notes, **and update `roadmaps/USAGE_ESPRESSO.md`** to mark F-05, F-06, F-07 as closed — the latter is what makes this release legible against the friction backlog.

## Acceptance Criteria

- [ ] `CHANGELOG.md` has a complete `[1.5.0] - 2026-MM-DD` section in Keep-a-Changelog format.
- [ ] Three subsections used: `Added`, `Migration Notes` (no removed/changed/fixed in this release).
- [ ] No breaking changes — v1.5 is a strictly additive minor bump. v1.4 callers compile unchanged.
- [ ] `go test ./... -race` clean.
- [ ] `golangci-lint run ./...` clean.
- [ ] `cd bench && go test -bench . -benchmem -benchtime=3s -count=1` — JSON round-trip numbers within noise of v1.4.
- [ ] Git tag `v1.5.0` created and pushed.
- [ ] GitHub release created with release notes mirroring the CHANGELOG entry.
- [ ] `package.json` `version` field bumped to `1.5.0`.
- [ ] `roadmaps/USAGE_ESPRESSO.md` updated:
  - F-05 marked `(closed)` with pointer to Task 1.
  - F-06 marked `(closed)` with pointer to Task 2.
  - F-07 marked `(closed)` with pointer to Task 3.
  - Cross-reference section at the bottom updated.
- [ ] Barista downstream pinged so they can plan the pin bump and retire `httpx.JSONWithCookies[T]`, `webhookRequest`, and `NewError(412, ...)`.

## Technical Approach

### Step 4.1: Verify all tasks complete

- [ ] Task 1: Cookies on `JSON[T]`
- [ ] Task 2: `extractor.RawBodyWithHeaders[H]`
- [ ] Task 3: `ErrPreconditionFailed`

Walk each task file. All Acceptance Criteria checkboxes ticked.

### Step 4.2: Update CHANGELOG

Promote `[Unreleased]` content into `[1.5.0] - <ship date>`. Final shape:

```markdown
## [Unreleased]

## [1.5.0] - 2026-MM-DD

### Added

- **`JSON[T].Cookies` field** — set HTTP cookies alongside JSON responses.
  Cookies are written via `http.SetCookie` before the status header,
  ensuring `Set-Cookie` lands in the response head. Zero-value behavior
  is byte-identical to v1.4. Closes Barista F-05.

- **`extractor.RawBodyWithHeaders[H]`** — extract raw body bytes alongside
  structured headers in one read pass. Useful for webhook receivers that
  verify HMAC against the unparsed payload. Closes Barista F-06.

- **`ErrPreconditionFailed(message string)`** — 412 Precondition Failed
  constructor, completing the symmetry with the other status-keyed
  helpers. Closes Barista F-07.

### Migration Notes

v1.5 is **strictly additive**. No v1.4 caller is affected.

If you wrote a chart-internal `JSONWithCookies[T]` wrapper (per Barista's
`httpx.JSONWithCookies[T]` pattern), you can now retire it:

```go
// Before
return httpx.JSONWithCookies[Token]{
    Data:    Token{Access: t},
    Cookies: []*http.Cookie{refreshCookie},
}

// After
return espresso.JSON[Token]{
    Data:    Token{Access: t},
    Cookies: []*http.Cookie{refreshCookie},
}
```

If you wrote a custom `webhookRequest` extractor for "raw body + provider
header", switch to `extractor.RawBodyWithHeaders[H]`. The migration is
mechanical: define a struct using `header:"X-Foo,required"` tags and use
the generic.

If you used `NewError(http.StatusPreconditionFailed, msg).WithCode(...)`,
swap for `ErrPreconditionFailed(msg)`.
```

### Step 4.3: Final quality gates

```bash
go test ./... -race
golangci-lint run ./...
go test -tags=integration ./tests/integration/...
cd bench && go test -bench . -benchmem -benchtime=3s -count=1 && cd ..
```

All four pass. The bench numbers must show no regression on `BenchmarkJSON_*`-style tests vs. v1.4 (the new `Cookies` field at zero value should be free).

### Step 4.4: Bump and tag

- `package.json` version → `1.5.0`.
- Commit: `chore(release): bump to v1.5.0`.
- Tag: `git tag v1.5.0 && git push origin v1.5.0`.
- GitHub release: paste the `[1.5.0]` CHANGELOG section verbatim.

### Step 4.5: Update USAGE_ESPRESSO.md

For each closed friction item, edit the entry in place (don't delete; preserve the historical observation):

```markdown
#### F-05 (closed)

`JSON[T]` was the v0.3 friction (refresh-token cookies). Espresso v1.5.0
shipped `JSON[T].Cookies []*http.Cookie` and the chart-internal
`httpx.JSONWithCookies[T]` was retired in <Barista PR / commit ref>.
Closing the entry — no follow-up needed.
```

Same shape for F-06 and F-07.

Update the `## Cross-reference` section at the bottom to note F-05/06/07 are now closed by v1.5.

### Step 4.6: Notify Barista

Open a GitHub issue or send a chart-team ping pointing to:

- The v1.5.0 release notes.
- The three migration recipes in the CHANGELOG `Migration Notes`.
- The expected effect: retire `httpx.JSONWithCookies[T]`, `webhookRequest`, `NewError(412, ...)`.

This is the validation step — if Barista lifts the workarounds without re-introducing them, the v1.5 design is correct. If they hit a snag, that's a candidate for a v1.5.1 patch.

## Tests Required

The full test suite, plus the bench module spot-check.

## Definition of Done

- [ ] `CHANGELOG.md` `[1.5.0]` section finalized with the ship date.
- [ ] `package.json` version bumped.
- [ ] Git tag `v1.5.0` pushed.
- [ ] GitHub release published.
- [ ] `roadmaps/USAGE_ESPRESSO.md` updated — F-05, F-06, F-07 marked closed.
- [ ] Barista pinged with migration recipes.
