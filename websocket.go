package espresso

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// MessageType distinguishes text and binary WebSocket messages.
type MessageType int

const (
	// MessageText indicates a UTF-8 encoded text message.
	MessageText MessageType = MessageType(websocket.MessageText)
	// MessageBinary indicates a binary message.
	MessageBinary MessageType = MessageType(websocket.MessageBinary)
)

// CloseCode is a WebSocket close status code as defined in RFC 6455.
type CloseCode int

const (
	// CloseNormal indicates a normal closure.
	CloseNormal CloseCode = 1000
	// CloseGoingAway indicates the endpoint is going away.
	CloseGoingAway CloseCode = 1001
	// CloseProtocolError indicates a protocol error.
	CloseProtocolError CloseCode = 1002
	// CloseUnsupportedData indicates unsupported data type.
	CloseUnsupportedData CloseCode = 1003
	// CloseNoStatus indicates no status code was received.
	CloseNoStatus CloseCode = 1005
	// CloseAbnormal indicates an abnormal closure.
	CloseAbnormal CloseCode = 1006
	// CloseInvalidPayload indicates invalid payload data.
	CloseInvalidPayload CloseCode = 1007
	// ClosePolicyViolation indicates a policy violation.
	ClosePolicyViolation CloseCode = 1008
	// CloseMessageTooBig indicates message is too big.
	CloseMessageTooBig CloseCode = 1009
	// CloseInternalError indicates an internal error.
	CloseInternalError CloseCode = 1011
	// CloseServiceRestart indicates the service is restarting.
	CloseServiceRestart CloseCode = 1012
	// CloseTryAgainLater indicates to try again later.
	CloseTryAgainLater CloseCode = 1013
)

// wsMessage is a WebSocket message read from the connection.
type wsMessage struct {
	msgType MessageType
	data    []byte
	err     error
}

// WSConfig holds configuration for a WebSocket handler.
type WSConfig struct {
	PingInterval   time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxMessageSize int64
	Subprotocols   []string
	OriginPatterns []string
	Compression    websocket.CompressionMode
}

// WSOption configures a WebSocket handler.
type WSOption func(*WSConfig)

// WithPingInterval sets how often ping frames are sent. Set to 0 to disable.
func WithPingInterval(d time.Duration) WSOption {
	return func(c *WSConfig) { c.PingInterval = d }
}

// WithWSReadTimeout sets the timeout for Read operations. 0 means no timeout.
func WithWSReadTimeout(d time.Duration) WSOption {
	return func(c *WSConfig) { c.ReadTimeout = d }
}

// WithWSWriteTimeout sets the timeout for Write operations.
func WithWSWriteTimeout(d time.Duration) WSOption {
	return func(c *WSConfig) { c.WriteTimeout = d }
}

// WithMaxMessageSize sets the maximum allowed message size in bytes.
func WithMaxMessageSize(size int64) WSOption {
	return func(c *WSConfig) { c.MaxMessageSize = size }
}

// WithSubprotocols sets the supported WebSocket subprotocols.
func WithSubprotocols(protos ...string) WSOption {
	return func(c *WSConfig) { c.Subprotocols = protos }
}

// WithOriginPatterns sets allowed Origin header patterns for CORS validation.
func WithOriginPatterns(patterns ...string) WSOption {
	return func(c *WSConfig) { c.OriginPatterns = patterns }
}

// WithCompression sets the per-message deflate compression mode.
func WithCompression(mode websocket.CompressionMode) WSOption {
	return func(c *WSConfig) { c.Compression = mode }
}

// WS wraps a WebSocket connection with an Espresso-friendly API.
// Obtain a *WS instance by registering a handler via WebSocket() or
// WebSocketSimple().
//
// WS is safe for concurrent writes. Reads are handled internally via a
// background goroutine, and messages are delivered through the Read method.
// The connection's context is canceled when the client disconnects, which
// enables handlers to detect disconnect via ctx.Done().
type WS struct {
	conn     *websocket.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	config   WSConfig
	mu       sync.Mutex
	closed   bool
	msgCh    chan wsMessage
	closeErr error
}

func newWS(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, cfg WSConfig) *WS {
	return &WS{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
		config: cfg,
		msgCh:  make(chan wsMessage, 64),
	}
}

// readLoop reads messages from the WebSocket in a background goroutine
// and sends them to the msgCh channel. When a read error occurs (including
// client disconnect), it sends the error and cancels the context.
func (w *WS) readLoop() {
	defer close(w.msgCh)
	for {
		msgType, data, err := w.conn.Read(w.ctx)
		if err != nil {
			w.mu.Lock()
			if !w.closed {
				w.closeErr = err
				w.closed = true
				w.cancel()
			}
			w.mu.Unlock()
			w.msgCh <- wsMessage{err: err}
			return
		}
		w.msgCh <- wsMessage{msgType: MessageType(msgType), data: data}
	}
}

// Read reads the next message from the WebSocket connection.
// It blocks until a message is received, the context is canceled,
// or the connection is closed.
func (w *WS) Read(ctx context.Context) (MessageType, []byte, error) {
	select {
	case msg, ok := <-w.msgCh:
		if !ok {
			return 0, nil, io.ErrClosedPipe
		}
		if msg.err != nil {
			return 0, nil, msg.err
		}
		return msg.msgType, msg.data, nil
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	}
}

// Write sends a message to the WebSocket connection.
func (w *WS) Write(ctx context.Context, msgType MessageType, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return io.ErrClosedPipe
	}
	writeCtx := ctx
	if w.config.WriteTimeout > 0 {
		var cancel context.CancelFunc
		writeCtx, cancel = context.WithTimeout(ctx, w.config.WriteTimeout)
		defer cancel()
	}
	return w.conn.Write(writeCtx, websocket.MessageType(msgType), data)
}

// WriteText is a convenience method that writes a UTF-8 text frame.
func (w *WS) WriteText(ctx context.Context, text string) error {
	return w.Write(ctx, MessageText, []byte(text))
}

// WriteBinary is a convenience method that writes a binary frame.
func (w *WS) WriteBinary(ctx context.Context, data []byte) error {
	return w.Write(ctx, MessageBinary, data)
}

// WriteJSON marshals v to JSON and sends it as a text frame.
func (w *WS) WriteJSON(ctx context.Context, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.Write(ctx, MessageText, data)
}

// ReadJSON reads the next message and unmarshals it into v.
// v must be a non-nil pointer.
func (w *WS) ReadJSON(ctx context.Context, v any) error {
	_, data, err := w.Read(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// Close closes the WebSocket connection with the given status code and reason.
// If the connection is already closed, Close returns nil.
func (w *WS) Close(code CloseCode, reason string) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	err := w.conn.Close(websocket.StatusCode(code), reason)
	cancel := w.cancel
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	defaultRegistry.remove(w)
	return err
}

// Context returns the WebSocket's context.
// This context is canceled when the WebSocket is closed, either by
// the server, the client, or due to an error.
func (w *WS) Context() context.Context {
	return w.ctx
}

// Subprotocol returns the negotiated WebSocket subprotocol.
// Returns an empty string if no subprotocol was negotiated.
func (w *WS) Subprotocol() string {
	return w.conn.Subprotocol()
}

// startPing sends ping frames at the configured interval.
// It stops when the context is canceled or when done is closed.
func (w *WS) startPing(ctx context.Context, done chan struct{}) {
	if w.config.PingInterval <= 0 {
		return
	}
	ticker := time.NewTicker(w.config.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := w.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// wsRegistry tracks open WebSocket connections for graceful shutdown.
type wsRegistry struct {
	mu    sync.RWMutex
	conns map[*WS]struct{}
}

func newWSRegistry() *wsRegistry {
	return &wsRegistry{conns: make(map[*WS]struct{})}
}

func (r *wsRegistry) add(ws *WS) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conns[ws] = struct{}{}
}

func (r *wsRegistry) remove(ws *WS) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns, ws)
}

func (r *wsRegistry) closeAll(code CloseCode, reason string) {
	r.mu.RLock()
	conns := make([]*WS, 0, len(r.conns))
	for ws := range r.conns {
		conns = append(conns, ws)
	}
	r.mu.RUnlock()

	for _, ws := range conns {
		_ = ws.Close(code, reason)
	}
}

func (r *wsRegistry) len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.conns)
}

// defaultWSConfig returns the default WebSocket configuration.
func defaultWSConfig() WSConfig {
	return WSConfig{
		PingInterval:   30 * time.Second,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxMessageSize: 1 << 20,
	}
}

// ============================================
// WebSocket Handler Wrappers
// ============================================

// WebSocket wraps a WebSocket handler so it can be registered as a route.
// The type parameter Req must implement FromRequest for request data extraction
// before the WebSocket upgrade. State injection via MustGetState[T] works inside
// the handler because the request context (with state middleware) is passed through.
//
// Example:
//
//	router.Get("/ws/{room}", espresso.WebSocket(echoHandler))
func WebSocket[Req FromRequest](h func(ctx context.Context, req Req, ws *WS) error, opts ...WSOption) http.HandlerFunc {
	cfg := defaultWSConfig()
	for _, opt := range opts {
		opt(&cfg)
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

		acceptOpts := &websocket.AcceptOptions{
			Subprotocols:    cfg.Subprotocols,
			OriginPatterns:  cfg.OriginPatterns,
			CompressionMode: cfg.Compression,
		}

		conn, err := websocket.Accept(w, r, acceptOpts)
		if err != nil {
			http.Error(w, "websocket upgrade failed", http.StatusUpgradeRequired)
			return
		}

		if cfg.MaxMessageSize > 0 {
			conn.SetReadLimit(cfg.MaxMessageSize)
		}

		ctx, cancel := context.WithCancel(r.Context())

		ws := newWS(ctx, cancel, conn, cfg)
		defaultRegistry.add(ws)

		pingDone := make(chan struct{})
		go ws.startPing(ctx, pingDone)
		go ws.readLoop()

		handlerErr := func() (handlerErr error) {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("websocket handler panic", "panic", rec)
					handlerErr = fmt.Errorf("panic: %v", rec)
				}
			}()
			return h(ctx, req, ws)
		}()

		close(pingDone)

		if handlerErr != nil {
			_ = ws.Close(CloseInternalError, handlerErr.Error())
		} else if !ws.closed {
			_ = ws.Close(CloseNormal, "")
		}
	}
}

// WebSocketSimple wraps a WebSocket handler that doesn't need an extractor.
// This is the Ristretto-equivalent for WebSockets.
//
// Example:
//
//	router.Get("/ws", espresso.WebSocketSimple(pingHandler))
func WebSocketSimple(h func(ctx context.Context, ws *WS) error, opts ...WSOption) http.HandlerFunc {
	cfg := defaultWSConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		acceptOpts := &websocket.AcceptOptions{
			Subprotocols:    cfg.Subprotocols,
			OriginPatterns:  cfg.OriginPatterns,
			CompressionMode: cfg.Compression,
		}

		conn, err := websocket.Accept(w, r, acceptOpts)
		if err != nil {
			http.Error(w, "websocket upgrade failed", http.StatusUpgradeRequired)
			return
		}

		if cfg.MaxMessageSize > 0 {
			conn.SetReadLimit(cfg.MaxMessageSize)
		}

		ctx, cancel := context.WithCancel(r.Context())

		ws := newWS(ctx, cancel, conn, cfg)
		defaultRegistry.add(ws)

		pingDone := make(chan struct{})
		go ws.startPing(ctx, pingDone)
		go ws.readLoop()

		handlerErr := func() (handlerErr error) {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("websocket handler panic", "panic", rec)
					handlerErr = fmt.Errorf("panic: %v", rec)
				}
			}()
			return h(ctx, ws)
		}()

		close(pingDone)

		if handlerErr != nil {
			_ = ws.Close(CloseInternalError, handlerErr.Error())
		} else if !ws.closed {
			_ = ws.Close(CloseNormal, "")
		}
	}
}

// defaultRegistry is the global WebSocket connection registry used by
// WebSocket handlers. Initialized on package load.
var defaultRegistry = newWSRegistry()
