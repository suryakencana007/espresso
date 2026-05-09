# Espresso Usage — friction, wins, and concrete API feedback from Barista

> Barista is [Espresso](https://github.com/suryakencana007/espresso)'s flagship application. This file records what the framework made easy (wins) and where it forced workarounds (friction), so Espresso's API surface can evolve against real pressure rather than speculation.
>
> Format per entry:
> - **ID** (`W-NN` for wins, `F-NN` for friction)
> - **Surface** (e.g. `Stream`, `WS`, `MustGetState`)
> - **Observation** — neutral description
> - **Evidence** — file path + line, or the PR / commit
> - **Suggested change** (friction only) — what Espresso could do to remove the workaround

Update when a new Espresso-shaped decision lands. Batch entries by milestone headings so they're easy to cite in release notes.

## v0.1.0 — seed observations

### Wins

#### W-01 — `MustGetState[T]` holds well at scale

- **Surface:** `espresso.WithState` / `espresso.MustGetState[T]`
- **Observation:** Injecting `appstate.AppState` via `WithState` and retrieving it in every handler with `MustGetState[appstate.AppState](ctx)` is ergonomic and stays type-safe across 20+ handlers. No dependency-injection framework needed; the whole wire-up fits in `cmd/barista/main.go`.
- **Evidence:** `internal/api/server.go:31`, every file under `internal/api/handler/`.

#### W-02 — Structured error envelope is quiet-day-one reliable

- **Surface:** `*espresso.Error` + `.WithCode(...)`
- **Observation:** Forcing handlers to return `*espresso.Error` instead of `error` makes the wire shape `{"error":{"code","message","details","request_id"}}` uniform for every failure path — including extractors and upgrade failures. Front-end code matches on `err.code` (see `web/src/lib/entities/user/fetcher.ts:16`) and never touches HTTP status in isolation. Zero drift across 23 tasks.
- **Evidence:** `internal/api/handler/app.go` (mapAppError), `mapDomainError`, `mapDeploymentCreateError`, etc.

#### W-03 — SSE interface-seam testing

- **Surface:** `*espresso.SSEStream`
- **Observation:** The log-stream handler narrows the SSE dependency to a tiny `logStreamSender` interface (`SendJSON(name string, v any) error`). Pipeline logic can be driven with a fake sender in unit tests while the handler itself still accepts the real `*espresso.SSEStream`. Pattern carries over unchanged to the Task 04 build-log handler.
- **Evidence:** `internal/api/handler/logs.go:21` and `internal/api/handler/logs_test.go`.
- **Why it's worth repeating:** Future streaming handlers (web terminal, anything bidirectional) should extract the same 2–3-method interface instead of depending on the full `*espresso.SSEStream` / `*espresso.WS`.

### Friction

#### F-01 — `Ristretto` can't reach state

- **Surface:** `espresso.Ristretto()`
- **Observation:** `Ristretto` is pitched as the lightweight "health check" primitive, but its signature is `func() Res` — no `context.Context`. That means `MustGetState[T]` can't run inside it, so any handler that wants to, say, verify the DB is reachable immediately drops back to `HandlerCtx`. In Barista the only `Ristretto`-shaped handler is `/healthz`, and even that uses `HandlerCtx` today because it asserts state is retrievable.
- **Evidence:** `internal/api/server.go:41` — `r.Get("/healthz", espresso.HandlerCtx(healthz))`.
- **Suggested change:** Either (a) deprecate `Ristretto` in favour of `HandlerCtx` as the zero-arg primitive, or (b) give `Ristretto` a `func(ctx) Res` variant. Current `Ristretto` is a foot-gun because the first real health check needs state.

#### F-02 — `Stream` commits headers before the handler runs

- **Surface:** `espresso.Stream`
- **Observation:** `Stream` flushes HTTP headers as part of accepting the request — so a "resource not found" decision the handler would like to surface as an HTTP 404 can only be emitted as an SSE `event: error` frame on a 200-OK stream. Standard REST clients (plus CDNs, proxies, observability tools) don't treat `event: error` on a 200 stream like a real 4xx, and every downstream integrator has to special-case it.
- **Evidence (workaround):** `internal/api/middleware/preflight.go:43` — `RequireAppAccess` / `RequireDeploymentAccess` run the ownership lookup and return a real 404 before `Stream` is ever invoked. See also `internal/api/server.go:81`.
- **Suggested change:** Let `Stream` handlers return an `espresso.Error` before the stream opens — e.g. a pre-flight phase that still lets the handler inspect the request context. v0.2 landed a middleware workaround (Task 05), but it's boilerplate per SSE route; first-class support in `Stream` would erase it.

## v0.2.0 — additional observations

> Snapshot frozen at `v0.2.0` ship (2026-05-01). The 14-step browser smoke walked top-to-bottom against Docker Desktop k8s — every Espresso surface listed in this section was exercised under live load (BuildKit gRPC + ring-buffer SSE + Traefik IngressRoute apply), and the wins below are validated, not speculative.

### Wins

#### W-04 — The keepalive knob on `Stream` works transparently

- **Surface:** `espresso.Stream(handler, espresso.WithKeepAlive(30 * time.Second))`
- **Observation:** Both SSE endpoints (`/api/apps/{id}/logs`, `/api/deployments/{id}/logs`) use the 30-second keepalive option and neither had to spell the keepalive out in the handler body. The browser's `LogPanel` (fetch + `eventsource-parser`) sees the comment frames and stays connected indefinitely without ever needing to reconnect mid-build.
- **Evidence:** `internal/api/server.go:82,84`, `web/src/lib/shared/ui/log-panel/log-panel.svelte`.

#### W-05 — `WithState` is compatible with the `log-stream-bus` pattern

- **Surface:** `appstate.AppState` + `MustGetState[T]`
- **Observation:** Sharing the `BuildLogBus` across the deployment service (which `Append`s) and the SSE handler (which `Subscribe`s) is a single-field addition to `AppState`. No middleware dance, no extra constructor parameters on `Stream`, no Espresso-specific plumbing.
- **Evidence:** `internal/api/appstate/appstate.go:38`, `internal/service/buildlogs.go`.

### Friction

#### F-02 (continued) — workaround documented but costs per-route boilerplate

- Task 05 landed `RequireAppAccess` and `RequireDeploymentAccess` middlewares. Every new SSE route now has to wrap the `Stream(...)` handler in the matching middleware — see `internal/api/server.go:81-84`. That's two extra lines per route, plus a middleware per resource kind, plus a `MustGetApp` / `MustGetDeployment` stash-and-read in each handler. Scales poorly if v0.3 adds web-terminal (`WS`) or more SSE endpoints.

#### F-03 — handler signature sprawl for `Stream` and `WebSocket`

- **Surface:** `espresso.Stream` / `espresso.WebSocket`
- **Observation:** `Stream` handlers take `(ctx, req, stream) error`; `Doppio` takes `(ctx, req) (Res, error)`; `HandlerCtx` takes `(ctx) (Res, error)`. Switching a handler from JSON to SSE requires reshaping the signature *and* changing the `r.Get(...)` registration call. In Barista this is fine — the handlers are distinct — but it's a reason the `logStreamSender` interface-seam pattern (W-03) matters: the signature churn is contained to the outermost adapter.
- **Evidence:** compare `internal/api/handler/deployment.go:GetDeployment` with `internal/api/handler/logs.go:StreamBuildLogs`.
- **Suggested change:** None urgent. Flag in case v0.3's `WS` handlers hit the same friction.

#### W-06 — `Stream` + `WithKeepAlive` survives a 19-minute build under live load

- **Surface:** `espresso.Stream(handler, espresso.WithKeepAlive(30 * time.Second))` + `*espresso.SSEStream`
- **Observation:** During the v0.2.0 DoD walk, `/api/deployments/{id}/logs` carried a real BuildKit solve from `cloning` → `building` → `failed` (broken-Dockerfile path, ~30s) and a healthy build (~minutes). The browser's `LogPanel` stayed connected across both, the keepalive comment frames kept Traefik from idle-killing the stream, and the per-subscription drop counter from `BuildLogBus` never fired — confirmed via the absence of any `event: meta` `dropped > 0` frame across multiple sessions. The pattern handles "fast errors" (build fails before the first vertex) just as cleanly as "slow happy paths" without any per-handler keepalive code.
- **Evidence:** `internal/api/server.go:81-84`, `internal/service/buildlogs.go`, DoD walk transcript (the BuildKit `RUN ... && exit 1` test case rendered as `[error] [2/4] RUN ...: exit code: 1` end-to-end).

## v0.3.0 — additional observations

> Snapshot frozen at `v0.3.0` ship (2026-05-02). The web-terminal flow
> was exercised live against the chart-installed stack (xterm.js
> ↔ kubelet exec via `*espresso.WS`); the refresh-token cookie path
> validated reload-survival end-to-end. Both observations below are
> validated, not speculative.

### Wins

#### W-07 — `*espresso.WS` is small enough to bridge to k8s exec without an interface seam

- **Surface:** `espresso.WebSocket[Req](handler, opts...)` + `*espresso.WS`
- **Observation:** The web-terminal handler bridges `ws.Read` / `ws.WriteText` directly to `io.Pipe` + a small `io.Writer` adapter that wraps `ws.WriteText`. No `wsBridge` interface seam was needed for tests — the WS protocol is exercised via a fake `WebSocket`-shaped object on the SPA side, and the resize-queue + envelope-parsing logic on the Go side test as plain functions. Closes the W-03 anticipation: the SSE pattern was specifically about narrowing the dependency, but `*espresso.WS` is already narrow enough that direct use doesn't impede tests.
- **Evidence:** `internal/api/handler/terminal.go` (handler bridges to `k8s.Client.ExecPod` via `io.Pipe`); `internal/api/handler/terminal_test.go` (resize-queue tests); `web/src/lib/features/app/terminal-stream.test.ts` (fake-WebSocket tests for the protocol envelope).
- **Subprotocol auth:** F-04 anticipated friction around credentialing the upgrade (browsers don't allow custom WS headers). Solved with a `RequireAuthWS` middleware that accepts the bearer token via a `base64url.bearer.authorization.barista.io.<…>` `Sec-WebSocket-Protocol` entry — same scheme Kubernetes itself uses. Middleware strips the entry before negotiation so coder/websocket only sees the content protocol when picking. The pattern is small (~120 LOC) and cleanly reusable for any future WS route.

### Friction

#### F-04 (closed)

`*espresso.WS` was the v0.2 anticipated friction (web terminal). v0.3 sprint 1 landed the implementation; see W-07 above for what we found. The `Read` / `Write` API is straightforward; the WSConfig knobs (`WithSubprotocols`, `WithPingInterval`) were sufficient. Closing the entry — implementation done, no friction emerged that warrants a follow-up.

#### F-05 (closed)

`JSON[T]` was the v0.3 friction (refresh-token cookies). Espresso
v1.5.0 shipped `JSON[T].Cookies []*http.Cookie` (commit `e986754`,
roadmaps/v1.5/tasks/task-01-json-response-cookies.md). Cookies are
written via `http.SetCookie` before `WriteHeader`, so `Set-Cookie`
lands in the response head. The chart-internal
`httpx.JSONWithCookies[T]` can be retired in a Barista pin-bump PR.

The cookie **read** side was already covered by
`extractor.Cookie[T]` (struct-tag based at `extractor/extractor.go:251`);
that surface predated this entry's "no cookie reader in extractor.*"
note. The `middleware.ReadRefreshCookie` shim can be retired in
favor of an `extractor.Cookie[T]` struct with `cookie:"refresh"` tag.

Closing the entry — workaround retired upstream.

## v0.4.0 — additional observations

> Snapshot frozen at `v0.4.0` implementation complete (2026-05-03).
> Six tasks shipped over the milestone (#91 / #94 / #95-96 / #99 /
> #100 / #103 / #104), exercising new surfaces: custom extractors
> for raw-body + headers (webhook receivers), an additional 412
> error code (canary precondition), AppState scaling to 8 services
> + 2 k8s fields. Wins below are validated against the merged code;
> friction below is reproducible from the corresponding files.

### Wins

#### W-08 — Custom `Extract(r *http.Request) error` extractors are simple to compose

- **Surface:** the `FromRequest` interface implemented via `Extract(r *http.Request) error`
- **Observation:** Webhook receivers needed both the raw body bytes (for HMAC verification) AND a provider-specific header (`X-Hub-Signature-256` or `X-Gitlab-Token`). `extractor.RawBody` only carries the body. A 35-line custom `webhookRequest` struct + `Extract` method delivered both in one read pass, slotted into `espresso.Doppio[*webhookRequest, espresso.Status]` cleanly. No framework changes needed; the `FromRequest` contract was the right level of abstraction.
- **Evidence:** `internal/api/handler/webhook.go:24-44` (`webhookRequest` extractor + `Extract` method), `internal/api/server.go:108-116` (registration).
- **Why it's worth repeating:** Future "raw body + N specific headers" handlers (e.g. signed payloads from observability platforms, slack interactivity) follow the same pattern in ~30 lines.

#### W-09 — `MustGetState[T]` keeps absorbing service additions

- **Surface:** `appstate.AppState` + `MustGetState[T]`
- **Observation:** v0.4 added 4 new services (`MembershipSvc` + `CanarySvc` + `WebhookSvc` + `ClusterSvc`) plus 1 new k8s field (`K8sRegistry`) onto `AppState` — total 8 services + 4 k8s/auth + 4 build-pipeline + 4 SSE buses. Every handler still reads them via the same `MustGetState[appstate.AppState](ctx)` one-liner; no measurable startup overhead, no awkward refactor. The "single big bag" pattern continues to scale where DI-framework approaches would have introduced a graph traversal.
- **Evidence:** `internal/api/appstate/appstate.go` (struct grew from 11 fields at v0.2 → 17 at v0.4), every handler under `internal/api/handler/`.

### Friction

#### F-06 (closed)

The "raw body + provider header" boilerplate was the v0.4 friction
(webhook receivers). Espresso v1.5.0 shipped
`extractor.RawBodyWithHeaders[H]` (commit `a59f8e1`,
roadmaps/v1.5/tasks/task-02-rawbody-with-headers-extractor.md). The
shape matches what was suggested: struct-of-headers keyed by
`header:"X-Foo,required"` tags + a `Body []byte` field, populated in
one read pass. Buffers >64KB release on `Reset` to prevent pool
memory bloat (mirrors `RawBodyExtractor`).

Barista's `internal/api/handler/webhook.go:24-44` `webhookRequest`
custom extractor can be retired in favor of:

```go
type GitHubHeaders struct {
    Signature string `header:"X-Hub-Signature-256,required"`
    Event     string `header:"X-GitHub-Event,required"`
}

func handle(ctx context.Context, req *extractor.RawBodyWithHeaders[GitHubHeaders]) (espresso.Status, error) {
    // verify HMAC against req.Body using req.Headers.Signature
}
```

Closing the entry — workaround retired upstream.

#### F-07 (closed)

The missing 412 helper was the v0.4 friction (Flagger CRD not
installed). Espresso v1.5.0 shipped `ErrPreconditionFailed(message
string)` (commit `cb0cc80`,
roadmaps/v1.5/tasks/task-03-precondition-failed-error.md). Returns
status 412 with code `PRECONDITION_FAILED`, matching the convention
of the other constructors.

Barista's `internal/api/handler/canary.go:88-91`
`espresso.NewError(http.StatusPreconditionFailed, msg).WithCode(...)`
fallback can be replaced with `espresso.ErrPreconditionFailed(msg)`.

Closing the entry — workaround retired upstream.

#### F-08 — Test seam patterns accumulate as exported setters

- **Surface:** any service using a function field for dispatch (e.g. `WebhookService.deployFromGit`, `ClusterService.clientFromKubeconfig` / `clientInCluster`)
- **Observation:** v0.4 introduced two test seams — `WebhookService.SetDeployFromGitForTest` and `ClusterService.SetClientBuildersForTest`. Both are exported methods that swap unexported function fields so handler-level tests in different packages can stub the dispatch. The pattern works but each occurrence pollutes the production type with a `*ForTest` method. Espresso doesn't enforce a particular DI structure, so this is an application-layer concern — but as a flagship app, Barista's accumulating seams suggest the framework could supply an idiomatic Options pattern (e.g. `service.WithOption(...)`) to keep test stubs out of the production surface.
- **Evidence:** `internal/service/webhook_service.go` (`SetDeployFromGitForTest`), `internal/service/cluster_service.go` (`SetClientBuildersForTest`).
- **Suggested change:** Not framework-side per se — but a pattern note in Espresso docs (or a tiny `optionalfield` helper) would help every flagship app avoid the same drift. Track in `roadmaps/v0.4.0/TECH_DEBT.md` (TD-HOT-04) for the v0.5 retro.

## v0.5.0 — additional observations

> Snapshot frozen at `v0.5.0` ship (2026-05-08). Five sprint tasks
> shipped over the milestone (#132 / #133 / #134 / #135 / #136) plus
> two walk-fix PRs (#137 / #138). New surfaces exercised: an
> additional `system_role` middleware (`RequireClusterAdmin`,
> mirrors the per-project `RequireProjectAccess` shape), the existing
> `webhookRequest` raw-body extractor under live HMAC load (Step 25
> webhook round-trip), and AppState scaling further to 9 services
> + 4 k8s/auth fields. **No new framework friction surfaced.** All
> v0.4 carry-forwards (F-05, F-06, F-07, F-08) remain open, with the
> same workarounds in place — none became more painful at v0.5
> scope, and none had upstream Espresso fixes to fold in.

### Wins

#### W-10 — `RequireClusterAdmin` middleware composes cleanly with `RequireAuth`

- **Surface:** Espresso's `r.Group(...)` + `r.Use(...)` middleware ergonomics
- **Observation:** Task 02 (cluster-admin-and-ui) needed cluster CRUD endpoints behind two stacked gates: authenticated AND `system_role == cluster-admin`. The natural Espresso shape — `r.Group` carrying `RequireAuth` + a nested `r.Use(adminGate)` — worked first try. Read endpoints stay on the auth-only path; mutations get the admin gate. Zero framework changes; existing primitives composed as expected.
- **Evidence:** `internal/api/server.go` (cluster route block), `internal/api/middleware/system_role.go` (`RequireClusterAdmin(authSvc)` builder), `internal/api/middleware/system_role.go` (`MustGetSystemRole` accessor mirrors `MustGetUserID`).
- **Why it's worth repeating:** Future role-based gates (per-cluster ACLs in v0.6, audit-log scopes, etc.) follow the same pattern — middleware that reads from auth state, sets a typed value into the request context via the existing context-key idiom, handler reads via `MustGetX`.

#### W-11 — Webhook receiver round-trip stays decoupled from encryption swap

- **Surface:** `webhookRequest` extractor (custom `Extract(*http.Request) error`) + `WebhookService` boundaries
- **Observation:** Task 00 swapped the at-rest storage of webhook secrets from plaintext to AES-GCM ciphertext (chart-supplied KEK). The receiver handler — `HandleGitHubWebhook` — needed zero changes. The extractor still emits `(rawBody, signatureHeader)`; the service still computes HMAC against the plaintext (decrypted in-memory at verify time). Espresso's "extractor returns a typed input, handler stays thin" shape kept the encryption swap entirely below the handler layer.
- **Evidence:** `internal/api/handler/webhook.go` (handler unchanged across v0.4 → v0.5), `internal/service/webhook_service.go` (`Seal` / `Open` calls inside `EncryptLegacyWebhookSecrets` + `verifyAndDispatch`), `internal/auth/secretbox.go` (the new at-rest encryption primitive).
- **Why it's worth repeating:** Storage-format migrations under a stable wire surface should generally not touch handlers. This was a clean test of that contract.

### Friction

(No new framework-side friction surfaced at v0.5 scope. F-05 / F-06 / F-07 / F-08 remain open with the same workarounds in place — see above.)

## v0.6.0 — additional observations

> Snapshot frozen at `v0.6.0` ship (2026-05-09). Four sprint tasks
> shipped over three sprints (#141–#143 audit-log, #144–#147
> per-cluster ACLs, #148 CodeMirror, #152 FUSE) plus 7 walk-fix PRs
> (#149/#150/#151/#153/#154/#155). New surfaces exercised: a second
> stacked auth-gate middleware (`RequireClusterRole(min)`, mirrors
> the per-project `RequireProjectAccess` shape exactly), audit-event
> capture wrapping every authenticated mutation (no Espresso
> change required — middleware reads the response writer's status
> via the existing `ResponseWriter` interface), and AppState scaling
> further to 11 services + 4 k8s/auth fields. **No new framework
> friction surfaced.** All v0.4 carry-forwards (F-05, F-06, F-07,
> F-08) remain open, with the same workarounds in place — none
> became more painful at v0.6 scope, and none had upstream Espresso
> fixes to fold in.

### Wins

#### W-12 — `RequireClusterRole(min)` builder composes cleanly with `RequireAuth`

- **Surface:** Espresso's `r.Group(...)` + `r.Use(...)` middleware ergonomics
- **Observation:** Task 02 (per-cluster-acls) needed cluster-scoped endpoints behind two stacked gates: authenticated AND `EffectiveRole(actor, cluster) >= min`. The natural Espresso shape — `r.Group` carrying `RequireAuth` + a nested `r.Use(RequireClusterRole(domain.ClusterRoleViewer))` for read endpoints, `RequireClusterRole(domain.ClusterRoleAdmin)` for member-mutation endpoints — worked first try, exactly mirroring W-10's `RequireClusterAdmin` shape from v0.5. Zero framework changes; existing primitives composed as expected. v0.6 is now the third role-based gate built on this pattern (project membership in v0.4 / system-cluster-admin in v0.5 / per-cluster-role in v0.6) and the API stays the same.
- **Evidence:** `internal/api/server.go` (cluster member route block), `internal/api/middleware/cluster_role.go` (`RequireClusterRole(min)` builder), `internal/api/middleware/cluster_role.go` (`MustGetClusterRole` accessor mirrors `MustGetSystemRole`).
- **Why it's worth repeating:** Future role-based gates (audit-log scopes if they're added, per-app ACLs if they're added) follow the same pattern — middleware reads from auth state + lookup, sets a typed value into the request context via the existing context-key idiom, handler reads via `MustGetX`. Three deployments and counting; the shape is load-bearing.

#### W-13 — Audit middleware wraps every mutation without touching handlers

- **Surface:** Espresso middleware + `ResponseWriter` interface + handler error contract
- **Observation:** Task 01 (audit-log) inserts an audit event for every authenticated mutation — captured `actor_id` from `MustGetUserID`, `request_id` from the existing trace middleware, and the handler's response status by wrapping the `http.ResponseWriter`. Because every Barista handler returns a `*espresso.Error` with a stable `code` field, the audit middleware can record both `result=ok` (status < 400) and `result=denied:<error.code>` (4xx) without any handler change. Zero handlers touched in v0.6 to add audit; the entire log surface is observed at the middleware seam.
- **Evidence:** `internal/api/middleware/audit.go` (the wrap), `internal/api/server.go` (where it composes after `RequireAuth`), `internal/audit/audit.go` (`AuditEvent` struct + `Logger` interface).
- **Why it's worth repeating:** Cross-cutting observability (audit, metrics, latency tracing) belongs at the middleware seam, not the handler — and Espresso's "handler returns `*espresso.Error`" contract is what makes this clean. If handlers returned plain `error` and let the framework infer status, the middleware would have to either re-derive status or duplicate the mapping. The structured-error envelope (W-02) is the foundation that makes audit cheap.

### Friction

(No new framework-side friction surfaced at v0.6 scope. F-05 / F-06 / F-07 / F-08 remain open with the same workarounds in place — see above.)

## Cross-reference

- TD-CONV-01 in [`roadmaps/v0.1.0/TECH_DEBT.md`](roadmaps/v0.1.0/TECH_DEBT.md) — this file closes that finding.
- TD-ESP-01 through TD-ESP-04 — folded into F-01, F-02, W-03, F-04 / W-07 above.
- TD-HOT-04 in [`roadmaps/v0.4.0/TECH_DEBT.md`](roadmaps/v0.4.0/TECH_DEBT.md) — F-08 is the framework-side framing of the same observation.
- F-05, F-06, F-07 closed by Espresso v1.5.0 (2026-05-10) — see `roadmaps/v1.5/`.
- F-01, F-02 deferred to Espresso v2.0 — both require breaking changes (Ristretto signature) or restructuring `serveStream`'s pre-flight phase. Tracked in `roadmaps/v2.0/`.
