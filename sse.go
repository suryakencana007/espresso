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
// After Close, all Send calls return an error.
func (s *SSEStream) Close() error {
	s.closed.Store(true)
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

type streamConfig struct {
	keepAliveInterval time.Duration
	initialRetryHint  time.Duration
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
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		serveStream(w, r, req, h, cfg)
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
		serveStreamSimple(w, r, h, cfg)
	}
}

func serveStreamSimple(w http.ResponseWriter, r *http.Request, h func(ctx context.Context, stream *SSEStream) error, cfg *streamConfig) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Flush headers
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	stream := &SSEStream{
		w:           w,
		flusher:     flusher,
		ctx:         ctx,
		lastEventID: r.Header.Get("Last-Event-ID"),
	}

	defaultSSERegistry.add(stream)

	// Send initial retry hint if configured
	if cfg.initialRetryHint > 0 {
		_ = stream.SetRetry(cfg.initialRetryHint)
	}

	// Start keepalive goroutine.
	// keepAliveStop signals the goroutine to shut down.
	// keepAliveExited is closed by the goroutine when it has fully exited,
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

	// Monitor client disconnect
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

	// Signal keepalive to stop and wait for it to fully exit
	// before returning, so no goroutine writes to w after the handler returns.
	if keepAliveStop != nil {
		close(keepAliveStop)
		<-keepAliveExited
	}

	_ = stream.Close()

	defaultSSERegistry.remove(stream)
}

func serveStream[Req any](w http.ResponseWriter, r *http.Request, req Req, h func(ctx context.Context, req Req, stream *SSEStream) error, cfg *streamConfig) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Flush headers
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	stream := &SSEStream{
		w:           w,
		flusher:     flusher,
		ctx:         ctx,
		lastEventID: r.Header.Get("Last-Event-ID"),
	}

	defaultSSERegistry.add(stream)

	// Send initial retry hint if configured
	if cfg.initialRetryHint > 0 {
		_ = stream.SetRetry(cfg.initialRetryHint)
	}

	// Start keepalive goroutine.
	// keepAliveStop signals the goroutine to shut down.
	// keepAliveExited is closed by the goroutine when it has fully exited,
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

	// Monitor client disconnect by watching for context cancellation.
	// When the HTTP connection closes, r.Context() will be canceled,
	// which propagates to our derived ctx.
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
		_ = h(ctx, req, stream)
	}()

	// Signal keepalive to stop and wait for it to fully exit
	// before returning, so no goroutine writes to w after the handler returns.
	if keepAliveStop != nil {
		close(keepAliveStop)
		<-keepAliveExited
	}

	_ = stream.Close()

	defaultSSERegistry.remove(stream)
}

// sseStreamRegistry tracks open SSE streams for graceful shutdown.
type sseStreamRegistry struct {
	mu      sync.RWMutex
	streams map[*SSEStream]struct{}
}

func newSSERegistry() *sseStreamRegistry {
	return &sseStreamRegistry{streams: make(map[*SSEStream]struct{})}
}

func (r *sseStreamRegistry) add(s *SSEStream) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streams[s] = struct{}{}
}

func (r *sseStreamRegistry) remove(s *SSEStream) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.streams, s)
}

func (r *sseStreamRegistry) closeAll(reason string) {
	r.mu.RLock()
	streams := make([]*SSEStream, 0, len(r.streams))
	for s := range r.streams {
		streams = append(streams, s)
	}
	r.mu.RUnlock()

	for _, s := range streams {
		_ = s.Comment("shutdown: " + reason)
		_ = s.Close()
	}
}

func (r *sseStreamRegistry) len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.streams)
}

// defaultSSERegistry is the global SSE stream registry.
var defaultSSERegistry = newSSERegistry()
