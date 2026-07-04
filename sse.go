package espresso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Event represents a single Server-Sent Event.
type Event struct {
	// ID is the event identifier. If empty, an auto-incremented ID is assigned on send.
	ID string
	// Name is the event type (maps to the "event:" field in SSE format).
	// If empty, no event field is sent (client receives a default "message" event).
	Name string
	// Data is the event payload (maps to the "data:" field).
	// Multi-line data is automatically split into multiple "data:" lines.
	Data string
	// Retry is a hint to the client about how long to wait before reconnecting.
	// Optional. If zero, no retry field is sent.
	Retry time.Duration
}

// SSEStream represents a Server-Sent Events stream to a client.
// Obtain an SSEStream by registering a handler via Stream() or StreamSimple().
// Do not construct SSEStream directly.
//
// SSEStream is safe for concurrent use by multiple goroutines.
// All Send methods internally acquire a mutex to ensure frame integrity.
type SSEStream struct {
	w           http.ResponseWriter
	flusher     http.Flusher
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	closed      atomic.Bool
	eventID     atomic.Uint64
	lastEventID string
}

// Send sends an event to the client.
// The event is automatically flushed to the client after writing.
// Returns an error if the client has disconnected or the stream is closed.
func (s *SSEStream) Send(event Event) error {
	if s.closed.Load() {
		return errors.New("stream closed")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed.Load() {
		return errors.New("stream closed")
	}

	if err := s.writeEvent(event); err != nil {
		s.closed.Store(true)
		return err
	}

	s.flusher.Flush()
	return nil
}

// SendJSON marshals v to JSON and sends it as an event with the given name.
func (s *SSEStream) SendJSON(name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	return s.Send(Event{Name: name, Data: string(data)})
}

// SendText sends an event with the given name and plain text data.
func (s *SSEStream) SendText(name, data string) error {
	return s.Send(Event{Name: name, Data: data})
}

// SendData sends an event with just data, no event name.
// The client will receive this as a default "message" event.
func (s *SSEStream) SendData(data string) error {
	return s.Send(Event{Data: data})
}

// Comment sends an SSE comment line.
// Comments are ignored by SSE clients but useful as keepalive pings.
func (s *SSEStream) Comment(comment string) error {
	if s.closed.Load() {
		return errors.New("stream closed")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed.Load() {
		return errors.New("stream closed")
	}

	if _, err := fmt.Fprintf(s.w, ": %s\n\n", comment); err != nil {
		s.closed.Store(true)
		return err
	}

	s.flusher.Flush()
	return nil
}

// SetRetry sets the reconnection retry interval hint for the client.
// This is sent as a "retry:" field. Clients will use this value when
// reconnecting after a disconnection.
func (s *SSEStream) SetRetry(d time.Duration) error {
	if s.closed.Load() {
		return errors.New("stream closed")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed.Load() {
		return errors.New("stream closed")
	}

	if _, err := fmt.Fprintf(s.w, "retry: %d\n\n", d.Milliseconds()); err != nil {
		s.closed.Store(true)
		return err
	}

	s.flusher.Flush()
	return nil
}

// LastEventID returns the value of the Last-Event-ID request header,
// which clients send when reconnecting. Use this to resume from where
// the client left off.
// Returns an empty string if the client did not send this header.
func (s *SSEStream) LastEventID() string {
	return s.lastEventID
}

// Context returns the stream's context.
// This context is canceled when the client disconnects or the stream is closed.
func (s *SSEStream) Context() context.Context {
	return s.ctx
}

// Close closes the stream. Safe to call multiple times.
// After Close, all Send calls return an error, and the stream's Context
// is canceled so handlers blocked on <-Context().Done() wake up.
func (s *SSEStream) Close() error {
	s.closed.Store(true)
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *SSEStream) writeEvent(e Event) error {
	var sb strings.Builder

	if e.ID == "" {
		e.ID = strconv.FormatUint(s.eventID.Add(1), 10)
	}
	sb.WriteString("id: ")
	sb.WriteString(e.ID)
	sb.WriteByte('\n')

	if e.Name != "" {
		sb.WriteString("event: ")
		sb.WriteString(e.Name)
		sb.WriteByte('\n')
	}

	if e.Data != "" {
		for _, line := range strings.Split(e.Data, "\n") {
			sb.WriteString("data: ")
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	if e.Retry > 0 {
		sb.WriteString("retry: ")
		sb.WriteString(strconv.FormatInt(e.Retry.Milliseconds(), 10))
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')

	_, err := s.w.Write([]byte(sb.String()))
	return err
}

// StreamOption configures a streaming handler.
type StreamOption func(*streamConfig)

// WithKeepAlive sets the interval for sending keepalive comment frames.
// Keepalive is useful for detecting disconnections and keeping proxies
// from closing idle connections.
// Set to 0 to disable. Default: disabled.
func WithKeepAlive(interval time.Duration) StreamOption {
	return func(c *streamConfig) { c.keepAliveInterval = interval }
}

// WithRetryHint sets an initial retry hint sent at stream start.
// This tells the client how long to wait before reconnecting.
func WithRetryHint(d time.Duration) StreamOption {
	return func(c *streamConfig) { c.initialRetryHint = d }
}

// WithPreFlight registers a pre-flight authorization closure that runs
// BEFORE the SSE response headers commit. If the closure returns a
// non-nil error, the stream is rejected and the error is routed through
// the standard JSON error pipeline (writeHandlerError) — an
// *espresso.Error surfaces with its declared status code (e.g. 404), and
// any other error becomes a 500. No SSE headers are written and no
// event frames are emitted.
//
// This closes the v0.2-era "Stream commits headers before the handler
// runs" foot-gun (USAGE_ESPRESSO.md F-02): handlers can now surface a
// "resource not found" decision as a real HTTP 4xx with a structured
// JSON body, instead of emitting an `event: error` frame on a 200-OK
// stream.
//
// The closure receives the request context, so it can read state via
// MustGetState[T] / GetState[T] and any context values populated by
// upstream middleware (request-id, auth principal, etc.). The extracted
// request body is NOT threaded into pre-flight in this iteration — keep
// pre-flight checks tied to context-derivable identity / authorization
// state.
//
// Example:
//
//	router.Get("/apps/{id}/logs", espresso.Stream(logsStream,
//	    espresso.WithPreFlight(func(ctx context.Context) error {
//	        s := espresso.MustGetState[AppState](ctx)
//	        if !s.UserCanReadApp(ctx) {
//	            return espresso.ErrNotFound("app not found")
//	        }
//	        return nil
//	    }),
//	))
//
// Zero overhead on the happy path: if no pre-flight closure is
// registered, the v2.0 stream flow is unchanged.
func WithPreFlight(fn func(ctx context.Context) error) StreamOption {
	return func(c *streamConfig) { c.preflight = fn }
}

type streamConfig struct {
	keepAliveInterval time.Duration
	initialRetryHint  time.Duration
	preflight         func(ctx context.Context) error
}

// StreamHandler is the function signature for SSE handlers.
// The type parameter T is the extractor type (e.g., Path[Req], Query[Req]).
type StreamHandler[T any] func(ctx context.Context, req T, stream *SSEStream) error

// Stream wraps an SSE handler so it can be registered as a route.
// It handles:
//   - Setting SSE response headers
//   - Extractor parsing
//   - State injection
//   - Keepalive (if configured via WithKeepAlive)
//   - Cleanup on client disconnect or handler return
//
// Example:
//
//	router.Get("/stream", espresso.Stream(counterStream))
func Stream[Req FromRequest](h StreamHandler[Req], opts ...StreamOption) http.HandlerFunc {
	cfg := &streamConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	pool := &sync.Pool{
		New: func() any {
			var zero Req
			return newReq(zero)
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		req := pool.Get().(Req) //nolint:errcheck // poolNew returns correct type
		defer func() {
			resetReq(req)
			pool.Put(req)
		}()

		fromReq, ok := any(req).(FromRequest)
		if ok {
			if err := fromReq.Extract(r); err != nil {
				writeExtractError(w, r, err)
				return
			}
		}

		serveStream(w, r, cfg, func(ctx context.Context, stream *SSEStream) error {
			return h(ctx, req, stream)
		})
	}
}

// StreamSimple wraps an SSE handler that doesn't need an extractor.
// This is the Ristretto-equivalent for SSE.
//
// Example:
//
//	router.Get("/time", espresso.StreamSimple(timeStream))
func StreamSimple(h func(ctx context.Context, stream *SSEStream) error, opts ...StreamOption) http.HandlerFunc {
	cfg := &streamConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		serveStream(w, r, cfg, h)
	}
}

// serveStream implements the shared SSE transport: headers, flushing, keepalive,
// disconnect monitoring, handler invocation with panic recovery, and cleanup.
// Both Stream[Req] and StreamSimple delegate here; Stream[Req] adapts its
// typed handler into the uniform signature via a closure.
func serveStream(w http.ResponseWriter, r *http.Request, cfg *streamConfig, h func(ctx context.Context, stream *SSEStream) error) {
	// Pre-flight phase (USAGE_ESPRESSO.md F-02): runs BEFORE any header is
	// committed, so a rejection becomes a real HTTP 4xx with a structured
	// JSON body via writeHandlerError — not an `event: error` frame on a
	// 200-OK stream. Skipped entirely when no pre-flight closure is
	// configured (zero overhead on the happy path).
	if cfg.preflight != nil {
		if err := cfg.preflight(r.Context()); err != nil {
			writeHandlerError(w, r, err)
			return
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeHandlerError(w, r, ErrInternal("streaming not supported"))
		return
	}

	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	regs, _ := routerRegistriesFrom(r.Context())
	stream := &SSEStream{
		w:           w,
		flusher:     flusher,
		ctx:         ctx,
		cancel:      cancel,
		lastEventID: r.Header.Get("Last-Event-ID"),
	}

	regs.sse.add(stream)

	if cfg.initialRetryHint > 0 {
		_ = stream.SetRetry(cfg.initialRetryHint)
	}

	// keepAliveStop signals the goroutine to shut down.
	// keepAliveExited is closed when the goroutine has fully exited,
	// ensuring no writes to http.ResponseWriter after the handler returns.
	var keepAliveStop chan struct{}
	var keepAliveExited chan struct{}
	if cfg.keepAliveInterval > 0 {
		keepAliveStop = make(chan struct{})
		keepAliveExited = make(chan struct{})
		go func() {
			defer close(keepAliveExited)
			ticker := time.NewTicker(cfg.keepAliveInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := stream.Comment("keepalive"); err != nil {
						return
					}
				case <-ctx.Done():
					return
				case <-keepAliveStop:
					return
				}
			}
		}()
	}

	// Fail-fast Send() calls once the client disconnects.
	go func() {
		<-ctx.Done()
		stream.closed.Store(true)
	}()

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("stream handler panic", "panic", rec)
				_ = stream.Close()
			}
		}()
		_ = h(ctx, stream)
	}()

	// Signal keepalive to stop and wait for it to fully exit before
	// returning, so no goroutine writes to w after the handler returns.
	if keepAliveStop != nil {
		close(keepAliveStop)
		<-keepAliveExited
	}

	_ = stream.Close()
	regs.sse.remove(stream)
}

// sseStreamRegistry tracks open SSE streams for graceful shutdown.
type sseStreamRegistry struct {
	mu      sync.RWMutex
	streams map[*SSEStream]struct{}
}

func newSSERegistry() *sseStreamRegistry {
	return &sseStreamRegistry{streams: make(map[*SSEStream]struct{})}
}

// add registers a stream. Nil-safe: a nil receiver is a no-op so call
// sites don't need to guard against handlers running outside a Router context.
func (r *sseStreamRegistry) add(s *SSEStream) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streams[s] = struct{}{}
}

// remove unregisters a stream. Nil-safe (see add).
func (r *sseStreamRegistry) remove(s *SSEStream) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.streams, s)
}

// closeAll cancels every registered stream's context so blocked handlers
// wake up and return; the HTTP server's Shutdown then drains their
// completion. The reason parameter is retained for signature stability
// but is no longer emitted as a final comment — writing to the response
// after the handler has returned races with net/http's finishRequest.
func (r *sseStreamRegistry) closeAll(_ string) {
	if r == nil {
		return
	}
	r.mu.RLock()
	streams := make([]*SSEStream, 0, len(r.streams))
	for s := range r.streams {
		streams = append(streams, s)
	}
	r.mu.RUnlock()

	for _, s := range streams {
		_ = s.Close()
	}
}

// len returns the number of registered streams. Nil receiver returns 0.
func (r *sseStreamRegistry) len() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.streams)
}

