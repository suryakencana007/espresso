# Task 10: Blog Post Draft

**Priority:** 📦 Meta
**Estimated Effort:** 1 day
**Dependencies:** Tasks 1-9 complete, v1.3.0 release ready or just published

## Context

A public release needs a public announcement. A blog post serves three purposes:

1. **Awareness** — lets existing users know v1.3 is out and what's in it
2. **Discovery** — helps new users find Espresso via search and social sharing
3. **Story** — explains *why* these features were added, which creates more engagement than a dry changelog

The post should connect Espresso v1.3 to the Barista project, because the story of "framework author building their own flagship app" is more compelling than "framework adds some features."

## Acceptance Criteria

- [ ] Blog post draft complete, approximately 1500-2500 words
- [ ] Title: "Espresso v1.3: Streaming, WebSocket, and the Road to Barista"
- [ ] Covers all major v1.3 features with code examples
- [ ] Explains the Barista connection and why it shapes the framework
- [ ] Tone is educational, not salesy
- [ ] Code examples are copy-pasteable and tested
- [ ] Includes links to documentation, examples, and Barista repo
- [ ] Ready to publish after v1.3.0 release is tagged

## Technical Approach

### Step 10.1: Outline

The post follows this structure:

```
1. Hook (1 paragraph)
   — Open with a problem or a story, not a feature list.
   — "Every framework hits a point where its maintainer stops using it..."

2. The Problem with Framework Development (2-3 paragraphs)
   — Frameworks without real-world use cases drift into abstraction.
   — Announce: I'm building Barista, a PaaS, using Espresso as the HTTP layer.
   — Barista is the forcing function for what Espresso v1.3 needs.

3. What's New in v1.3 (the bulk — roughly 60% of the post)
   — One subsection per major feature, each with:
     - The use case from Barista
     - A code example
     - Why this API shape and not another

   Subsections:
   3.1. WebSocket Handlers (web terminal for Kubernetes pods)
   3.2. Typed SSE Streaming (live log streaming)
   3.3. Structured Error Responses (consistent API errors)
   3.4. Graceful Shutdown Hooks (Kubernetes rolling updates)

4. The Design Philosophy (2-3 paragraphs)
   — Why type-safety first
   — Why context-first
   — Why coffee metaphor (fun, but also useful for naming consistency)

5. What's Next (1-2 paragraphs)
   — Barista v0.1 milestone
   — Espresso v1.4 ideas (gathered from v1.3 dogfooding)

6. Try It Out (1 paragraph + links)
   — Installation command
   — Links to docs, examples, Barista repo
   — How to provide feedback
```

### Step 10.2: Draft Content

Create `docs/v1.3-roadmap/blog-post-draft.md` with this content as a starting point. The writer should adapt based on their voice.

```markdown
# Espresso v1.3: Streaming, WebSocket, and the Road to Barista

*[Publication date: when v1.3.0 is released]*

Every framework hits a point where its maintainer stops using it for real
projects. The API starts to drift. Examples grow stale. Features get added
based on hypothetical needs rather than concrete pain. I didn't want Espresso
to go that way, so six months into maintaining it, I decided to build
something I'd actually depend on.

That something is Barista: a self-hosted Platform-as-a-Service, think of it
as a tiny Heroku you run on your own Kubernetes cluster. Barista is
ambitious. It handles Git-based deployments, container image builds,
SSL certificates, log streaming, backups — the works. But more importantly,
every feature Barista needs surfaces a gap in Espresso, and Espresso v1.3
is the result of filling those gaps.

This post walks through what's new in v1.3 and how each feature connects
back to Barista.

## The Problem with Framework Development

A framework written without a flagship application is a collection of
opinions about problems nobody has actually solved yet. You can see this
in too many Go web frameworks: they handle the happy path well but fall
over on the second day of real use. Connection cleanup is an afterthought.
Error responses are inconsistent. Long-lived connections leak goroutines.

I wanted Espresso to be different, which meant using it for something
hard enough to surface these issues. Barista requires:

- Browser-based terminal into a running Kubernetes pod
- Live streaming of pod logs to the browser (for hours)
- Consistent error responses across dozens of endpoints
- Clean shutdown when Kubernetes sends SIGTERM during a rolling update
- Large file uploads (gigabyte-scale backup restores)

Not one of these was well-supported in Espresso v1.2. They are now.

## WebSocket Handlers

Barista's web terminal needs bidirectional streaming between the browser
and a `kubectl exec` session in the target pod. That's WebSockets.

v1.3 adds first-class WebSocket support that feels like the rest of
Espresso: typed, context-aware, state-injected.

```go
type ExecReq struct {
    PodName   string `path:"pod"`
    Container string `query:"container"`
}

func execHandler(ctx context.Context, req *extractor.Path[ExecReq], ws *espresso.WS) error {
    state := espresso.MustGetState[AppState](ctx)

    // Open exec stream to Kubernetes pod
    stream, err := state.K8s.Exec(ctx, req.Data.PodName, req.Data.Container)
    if err != nil {
        return err
    }
    defer stream.Close()

    // Bridge browser WebSocket <-> kubectl exec
    return bridge(ctx, ws, stream)
}

router.Get("/ws/exec/{pod}", espresso.WebSocket(execHandler))
```

A few design choices worth calling out:

**Context propagation is mandatory.** Every Read and Write accepts a
`context.Context`. When the browser closes the tab, the context cancels,
the bridge function returns, and the Kubernetes exec stream gets cleaned
up. No leaked goroutines, no zombie subprocess.

**State injection just works.** `MustGetState[T]` is the same API you'd
use in a JSON handler. The Kubernetes client, database connection,
logger — all available through the same pattern.

**Graceful shutdown is built in.** When Barista receives SIGTERM from
Kubernetes, Espresso sends a close frame (code 1001 "going away") to
every open WebSocket before the HTTP server stops. Clients can reconnect
to the new instance once it's ready.

## Typed SSE Streaming

Logs are the second-most-used feature of any PaaS. Barista needs to
stream pod logs to a browser, continuously, potentially for hours.

Server-Sent Events is the right tool here — simpler than WebSocket,
works over HTTP/2, survives proxies that mangle WebSocket upgrades.
Espresso v1.2 had low-level SSE support, but it was the odd one out:
every other handler type was typed and generic-aware, and SSE was
`http.ResponseWriter` exposed directly.

v1.3 fixes that:

```go
type LogsReq struct {
    Pod    string `path:"pod"`
    Follow bool   `query:"follow"`
}

type LogLine struct {
    Timestamp time.Time `json:"timestamp"`
    Level     string    `json:"level"`
    Message   string    `json:"message"`
}

func logsStream(ctx context.Context, req *extractor.Path[LogsReq],
    stream *espresso.SSEStream) error {

    state := espresso.MustGetState[AppState](ctx)

    logs, err := state.K8s.StreamLogs(ctx, req.Data.Pod, req.Data.Follow)
    if err != nil {
        return err
    }
    defer logs.Close()

    for line := range logs.Lines() {
        if err := stream.SendJSON("log", LogLine{
            Timestamp: line.Time,
            Level:     line.Level,
            Message:   line.Text,
        }); err != nil {
            return err // client likely disconnected
        }
    }
    return nil
}

router.Get("/stream/logs/{pod}",
    espresso.Stream(logsStream, espresso.WithKeepAlive(30*time.Second)))
```

`SSEStream` handles the SSE protocol details (framing, flushing, event
IDs). The handler just calls `SendJSON` and the right bytes appear on
the wire. Keepalive comments are sent automatically every 30 seconds
so proxies don't close the connection.

The Last-Event-ID header is parsed and exposed via `stream.LastEventID()`
for implementing resume-from-offset patterns — essential for log
streaming where you don't want to show the user the same lines twice
after a reconnect.

## Structured Error Responses

By the time Barista's frontend was querying a dozen endpoints, the lack
of a consistent error format became obvious. Every handler was cobbling
together its own `{"error": ...}` response. When Barista's error handling
middleware wanted to route based on error type, it had to parse strings.

v1.3 establishes one format for all Espresso error responses:

```json
{
  "error": {
    "code": "PROJECT_NOT_FOUND",
    "message": "project with id 'abc123' does not exist",
    "details": {
      "projectId": "abc123"
    },
    "request_id": "req_01H8XYZ..."
  }
}
```

The handler side is ergonomic:

```go
func getProject(ctx context.Context, req *extractor.Path[GetReq]) (espresso.JSON[Project], error) {
    state := espresso.MustGetState[AppState](ctx)

    project, err := state.DB.FindProject(req.Data.ID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return espresso.JSON[Project]{},
                espresso.ErrNotFound("project not found").
                    WithCode("PROJECT_NOT_FOUND").
                    WithDetail("projectId", req.Data.ID)
        }
        return espresso.JSON[Project]{},
            espresso.ErrInternal("database error").Wrap(err)
    }
    return espresso.JSON[Project]{Data: project}, nil
}
```

A few things to note:

- **`Wrap(err)` hides internal errors from the response.** The wrapped
  error is logged for debugging, but never sent to the client. Database
  error messages and stack traces stay server-side.
- **Request IDs are auto-injected** if the `RequestIDMiddleware` is
  active. Every error includes the request ID, so user-reported issues
  can be traced to specific logs.
- **`errors.Is` and `errors.As` work.** You can wrap an Espresso error
  with `fmt.Errorf("doing thing: %w", err)` and it'll still be detected
  and serialized correctly.
- **Panics become structured errors.** The recovery middleware wraps
  panics into `*espresso.Error` with code `PANIC` and status 500. No
  more leaked stack traces or inconsistent responses from handler bugs.

## Graceful Shutdown Hooks

Kubernetes rolls out new pod versions by creating the new pod, waiting
for it to become healthy, then sending SIGTERM to the old pod. The old
pod has a grace period (default 30 seconds) to clean up before
Kubernetes kills it with SIGKILL.

During that grace period, a PaaS control plane has real work to do:

- Flush any in-memory caches
- Close database connections cleanly
- Tell open log streams to reconnect to the new instance
- Deregister from service discovery

v1.3 adds `OnShutdown` hooks for exactly this:

```go
router := espresso.Portafilter().
    WithState(state).
    OnShutdown(func(ctx context.Context) error {
        slog.Info("closing database pool")
        return state.DB.Close()
    }).
    OnShutdown(func(ctx context.Context) error {
        slog.Info("flushing metrics")
        return state.Metrics.Flush(ctx)
    }).
    Get("/health", espresso.Ristretto(health))

ctx, cancel := signal.NotifyContext(context.Background(),
    os.Interrupt, syscall.SIGTERM)
defer cancel()

if err := router.BrewContext(ctx,
    espresso.WithAddr(":8080"),
    espresso.WithShutdownTimeout(25*time.Second),
); err != nil {
    slog.Error("server error", "error", err)
}
```

Hooks run in registration order with the configured shutdown timeout.
After hooks complete, Espresso closes all open SSE streams (with a
final comment telling clients what happened) and all open WebSockets
(with close code 1001). Only then does the HTTP server stop accepting
connections and drain in-flight requests.

## Why These Shapes?

Every framework has an opinion about APIs. Espresso's opinions come from
a few convictions:

**Types over interfaces.** Go generics let us express "a handler that
takes a typed request and returns a typed response" without
`interface{}`. Every layer of v1.3 uses this — SSE and WebSocket
handlers are parameterized on the extractor type, just like JSON
handlers are.

**Context first, always.** Every function that does I/O or blocks
accepts `context.Context`. This sounds obvious, but a surprising number
of Go libraries still have methods like `conn.Read()` with no context.
When the request goes away, everything it spawned should go away too,
and context is how that happens.

**Be ergonomic for common cases, flexible for edge cases.**
`Ristretto` handles the "no parameters, just return a response" case
with no ceremony. `Doppio` gives you extractors and state when you
need them. New handler types follow the same pattern: `Stream` and
`WebSocket` for the typed case, `StreamSimple` and `WebSocketSimple`
for the no-extractor case.

**The coffee theme is real.** `Portafilter` creates the router,
`Ristretto`/`Solo`/`Doppio` are handler shot sizes, `Brew` starts the
server. It's fun, but also: when you're naming a new feature, asking
"does this fit the coffee metaphor?" is a useful forcing function
for consistency.

## What's Next

With v1.3 shipped, Barista development can accelerate. The v0.1
milestone is: login, create a project, deploy it from a Git repository,
view its logs, expose it at a custom domain. That's roughly three
months of work.

Along the way, I expect to find more gaps in Espresso. Some candidates
already on the v1.4 radar:

- **Startup hooks** (complement to shutdown hooks, for lazy-init patterns)
- **Typed middleware** — middleware that can see handler type information
- **OpenAPI 3.1 support** (current support is 3.0; 3.1 enables WebSocket
  documentation via AsyncAPI)
- **HTTP/2 server push** for static asset optimization

But these will only make it into v1.4 if Barista actually needs them.
The whole point of dogfooding is letting real requirements drive design.

## Try It Out

```sh
go get github.com/suryakencana007/espresso@v1.3.0
```

- **Documentation:** [README](https://github.com/suryakencana007/espresso#readme)
- **WebSocket deep-dive:** [docs/websocket.md](https://github.com/suryakencana007/espresso/blob/main/docs/websocket.md)
- **Streaming deep-dive:** [docs/streaming.md](https://github.com/suryakencana007/espresso/blob/main/docs/streaming.md)
- **Examples:** [cmd/example/](https://github.com/suryakencana007/espresso/tree/main/cmd/example)
- **Barista (in development):** [github.com/YOUR_USERNAME/barista](https://github.com/YOUR_USERNAME/barista)

If you try it and hit something weird, open an issue. I'll be exercising
all this heavily over the next few months building Barista, so the
feedback loop will be tight.

---

*[Author bio, social links, etc.]*
```

### Step 10.3: Publication Plan

Publication targets (in priority order):

1. **Personal blog or Dev.to** — primary home for the post
2. **Cross-post to Hashnode or Medium** — for wider reach
3. **Share on relevant channels:**
   - r/golang (Reddit)
   - Gophers Slack #announcements channel
   - Golang Weekly newsletter (submit via their form)
   - Twitter/X or BlueSky with thread summarizing key points
   - LinkedIn (if professionally relevant)

### Step 10.4: Timing

- **T-minus 1 day before v1.3.0 release:** finalize draft
- **T-zero (release day):** publish blog post immediately after GitHub release is live
- **T+1 day:** share on social channels (allows pkg.go.dev to index first)
- **T+3 days:** submit to newsletters (Golang Weekly has Wednesday deadline)

### Step 10.5: Post-Publication

- Monitor comments on all platforms for 48 hours
- Respond to questions and feedback
- Collect common questions for a potential FAQ addition to docs
- If a factual error is found, correct inline and add an edit note

## Definition of Done

- [ ] Draft posted to `docs/v1.3-roadmap/blog-post-draft.md`
- [ ] Word count between 1500-2500
- [ ] All code examples tested and working
- [ ] All links verified (no broken URLs)
- [ ] Proofread for grammar and clarity
- [ ] Title compelling, includes "Espresso v1.3" for searchability
- [ ] Includes clear call-to-action (install command + links)
- [ ] Ready to publish on release day

## Style Notes

- Write in first person, singular ("I built") not plural ("we built")
- Avoid superlatives ("amazing", "revolutionary", "game-changing")
- Show humility: "I didn't want Espresso to drift", not "Espresso is better than X"
- Use active voice ("The handler returns an error") not passive ("An error is returned by the handler")
- Code examples should stand alone — readers should be able to copy them
- One idea per paragraph; paragraphs should be scannable

## What to Avoid

- Don't attack other frameworks by name
- Don't make unverifiable performance claims
- Don't over-promise future features
- Don't bury the lead (WebSocket support) under philosophy
- Don't skip the "why" — changelog already covers "what"

## Tips for AI Agents Drafting This

1. Match the tone of the existing Espresso README — technical but friendly
2. Keep code examples short (under 20 lines each)
3. Use the story of Barista as the narrative spine
4. Cite specific Barista use cases, not generic "web apps"
5. If you're unsure about a phrase, prefer clarity over cleverness
6. Make every paragraph earn its place — cut ruthlessly
