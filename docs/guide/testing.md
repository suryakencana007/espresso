# Testing Patterns

Espresso doesn't enforce a particular dependency-injection style. Most apps that grow past a few handlers eventually face the same question: **how do you test a service that holds unexported function-field dispatch without leaking test-only API into the production type?** This page is the framework's recommended pattern for that case.

The page is application-layer guidance, not a feature. The framework does not ship a `testing/` helper package — the pattern is small enough that codifying it would over-fit. Read this as "the convention" rather than "the API".

## The Setup

A real service often looks like this — production logic delegated to a function field so the service can stay testable without standing up the whole world:

```go
package service

import "context"

type WebhookService struct {
    state appstate.AppState

    // Production dispatch: real git-clone + push to deploy queue.
    // Unexported so callers can't reach in and swap it.
    deployFromGit func(ctx context.Context, repo string) error
}

func NewWebhookService(state appstate.AppState) *WebhookService {
    return &WebhookService{
        state:         state,
        deployFromGit: defaultDeployFromGit(state),  // real implementation
    }
}

func (s *WebhookService) Handle(ctx context.Context, payload WebhookPayload) error {
    // ...validation, HMAC, etc...
    return s.deployFromGit(ctx, payload.Repository)
}
```

The function-field shape gives you a clean boundary: the handler doesn't care whether `deployFromGit` is real or a stub; it just calls it. Testing the handler without performing a real `git clone` is the goal.

## The Temptation: Exported `*ForTest` Setters

The path of least resistance is to add an exported setter so handler-level tests in a different package (e.g., `internal/api/handler_test.go`) can swap the function:

```go
// service/webhook_service.go

// SetDeployFromGitForTest swaps the dispatch function for tests.
//
//nolint:unused // used in tests
func (s *WebhookService) SetDeployFromGitForTest(fn func(ctx context.Context, repo string) error) {
    s.deployFromGit = fn
}
```

This works. But:

- **The production type now has a `*ForTest` method.** Every IDE autocomplete shows it. Every code review has to ignore it. Production callers can call it.
- **It scales linearly.** Each new function field needs its own setter. After 4–6 services, `*ForTest` setters accumulate to a meaningful fraction of the public surface.
- **It pollutes the godoc.** `pkg.go.dev` happily renders the method. Anyone reading the API thinks `SetDeployFromGitForTest` is part of the contract.

If you've already shipped these and want to retire them, this page's recommendation is the natural target.

## The Pattern: Private Functional Options

The pattern that scales is **functional options keyed by an unexported sentinel type**. Tests in any package can pass options that touch private fields; production callers can't see the options because the option type is unexported.

### Step 1 — Make the constructor accept options

```go
package service

type WebhookOption func(*WebhookService)

func NewWebhookService(state appstate.AppState, opts ...WebhookOption) *WebhookService {
    s := &WebhookService{
        state:         state,
        deployFromGit: defaultDeployFromGit(state),
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

Production callers continue to write `NewWebhookService(state)`. They never need an option.

### Step 2 — Expose the test-only option in an internal package

The option that swaps `deployFromGit` lives in `internal/servicetest/` (or wherever feels right):

```go
package servicetest

import (
    "context"

    "your-app/internal/service"
)

// WithDeployFromGit is a test-only option that swaps WebhookService's
// dispatch function. Only importable from the same module.
func WithDeployFromGit(fn func(ctx context.Context, repo string) error) service.WebhookOption {
    return func(s *service.WebhookService) {
        s.SetDeployFromGitForTest(fn)
    }
}
```

Wait — `SetDeployFromGitForTest` is the thing we're trying to remove. Two ways out:

**Option A — friend-package trick via `internal/`:**

`internal/` is Go's idiom for "code visible to a parent module, invisible to anyone else." If `servicetest` lives at `your-app/internal/servicetest/`, it can see `your-app/internal/service` package internals **only if they're exported**. So a `*ForTest` setter is still required, but you can name it without the `*ForTest` suffix (since the only caller is `servicetest`, and that's invisible from outside the module).

**Option B — write the option in the same package as the service:**

The cleanest version. `service/options.go` carries both production and test options; the test-only ones use lowercase first character so they're unexported, but the `WebhookOption` *type* is exported. Then a tiny `servicetest` package re-exports the test-only options through a typed accessor.

```go
// service/options_test.go  (note: _test.go — only compiled during go test)
package service

func WithDeployFromGit(fn func(context.Context, string) error) WebhookOption {
    return func(s *WebhookService) { s.deployFromGit = fn }
}
```

`_test.go` files in the same package have full access to unexported fields **and** are excluded from the regular build. The option doesn't appear in `pkg.go.dev`, doesn't appear in autocomplete from production callers' IDEs, and doesn't pollute the API. Internal-package tests in `service_test.go` can use it directly. Tests in `handler_test.go` (a different package) cannot — unless `WebhookOption` itself is exported and the `service` package's `_test.go` exposes a helper.

For cross-package tests, the cleanest approach is:

```go
// service/exporttest_test.go
package service

// SetDeployFromGitForTest is exposed only to tests via a tiny exporter
// in an internal test-only package. It does NOT appear in the production
// API because this file is _test.go-suffixed.
func SetDeployFromGitForTest(s *WebhookService, fn func(context.Context, string) error) {
    s.deployFromGit = fn
}
```

```go
// internal/servicetest/options.go  (regular file, importable by any test in the module)
package servicetest

import (
    "context"

    "your-app/internal/service"
)

// WithDeployFromGit returns an option suitable for service.NewWebhookService.
// Implementation lives in service/exporttest_test.go so it's compiled only
// during go test.
func WithDeployFromGit(fn func(context.Context, string) error) service.WebhookOption {
    return func(s *service.WebhookService) {
        service.SetDeployFromGitForTest(s, fn)
    }
}
```

This is the Go-idiomatic shape. The setter only exists during `go test`; the rest of the time it's not even compiled.

### Step 3 — Tests in any package can wire stubs cleanly

```go
// handler_test.go (different package)
import (
    "your-app/internal/service"
    "your-app/internal/servicetest"
)

func TestWebhookHandler(t *testing.T) {
    var called bool
    svc := service.NewWebhookService(
        testAppState(t),
        servicetest.WithDeployFromGit(func(ctx context.Context, repo string) error {
            called = true
            return nil
        }),
    )
    // ...exercise the handler with svc, assert called == true
}
```

No `*ForTest` setter visible to production code. Every test gets the same fluent options-pattern composition. The setter wiring lives in a `_test.go` file that's never compiled into a release binary.

## Why Espresso Doesn't Ship a Helper

The pattern above is ~30 lines of code per service. A framework helper would have to cover a wide range of dispatch shapes (function fields, interface fields, struct embedding) and end up either too general to be useful or too narrow to fit. The convention scales better than a library here.

If you find yourself writing the same boilerplate three times, extract a per-app helper. Don't expect the framework to generalize for you — your shape is more specific than ours can be.

## When to Reach for Interfaces Instead

If a service's function field is replaced often enough that you have **four or more** test stubs across different test files, consider promoting it to an interface field:

```go
type GitDeployer interface {
    Deploy(ctx context.Context, repo string) error
}

type WebhookService struct {
    deployer GitDeployer
}
```

Now your tests pass mock structs that satisfy the interface — no options-pattern needed, and the interface itself documents the seam. The function-field-with-options pattern shines for one-off seams; interfaces shine when the seam is conceptually first-class.

## See Also

- [Service Layers](middleware/service.md) — the `Service[Req, Res]` interface is the equivalent first-class seam at the request-pipeline level.
- [State & Dependency Injection](state.md) — the `MustGetState[T]` pattern for handler-side dependency access (a different kind of seam, less prone to the `*ForTest` problem).
