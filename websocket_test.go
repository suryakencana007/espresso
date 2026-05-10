package espresso

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/suryakencana007/espresso/v2/extractor"
)

func dialWS(ctx context.Context, t *testing.T, url string, opts *websocket.DialOptions) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.Dial(ctx, url, opts)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	return conn
}

func TestWebSocket_Upgrade(t *testing.T) {
	handler := func(ctx context.Context, ws *WS) error {
		for {
			msgType, data, err := ws.Read(ctx)
			if err != nil {
				return nil
			}
			if err := ws.Write(ctx, msgType, data); err != nil {
				return err
			}
		}
	}

	router := Portafilter().
		Get("/ws", WebSocketSimple(handler))

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn := dialWS(context.Background(), t, wsURL, nil)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
}

func TestWebSocket_RejectNonUpgrade(t *testing.T) {
	handler := func(ctx context.Context, ws *WS) error {
		return nil
	}

	router := Portafilter().
		Get("/ws", WebSocketSimple(handler))

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/ws")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// The handler attempts to upgrade via websocket.Accept which writes to w,
	// but a plain HTTP GET can't be upgraded, so we get a 4xx response
	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected non-200 status for non-WebSocket request, got %d", resp.StatusCode)
	}
}

func TestWebSocket_EchoText(t *testing.T) {
	handler := func(ctx context.Context, ws *WS) error {
		msgType, data, err := ws.Read(ctx)
		if err != nil {
			return nil
		}
		return ws.Write(ctx, msgType, data)
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn := dialWS(context.Background(), t, wsURL, nil)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := conn.Write(ctx, websocket.MessageText, []byte("hello"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(data) != "hello" {
		t.Errorf("expected %q, got %q", "hello", string(data))
	}
}

func TestWebSocket_EchoBinary(t *testing.T) {
	handler := func(ctx context.Context, ws *WS) error {
		msgType, data, err := ws.Read(ctx)
		if err != nil {
			return nil
		}
		return ws.Write(ctx, msgType, data)
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn := dialWS(context.Background(), t, wsURL, nil)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	binaryData := []byte{0x01, 0x02, 0x03}
	err := conn.Write(ctx, websocket.MessageBinary, binaryData)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msgType != websocket.MessageBinary {
		t.Errorf("expected binary message, got text")
	}

	if !bytes.Equal(data, binaryData) {
		t.Errorf("expected %v, got %v", binaryData, data)
	}
}

func TestWebSocket_JSON(t *testing.T) {
	type testMsg struct {
		Text string `json:"text"`
	}

	handler := func(ctx context.Context, ws *WS) error {
		var msg testMsg
		if err := ws.ReadJSON(ctx, &msg); err != nil {
			return err
		}
		return ws.WriteJSON(ctx, testMsg{Text: "echo: " + msg.Text})
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn := dialWS(context.Background(), t, wsURL, nil)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := conn.Write(ctx, websocket.MessageText, []byte(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if !strings.Contains(string(data), "echo: hello") {
		t.Errorf("expected echo response, got %q", string(data))
	}
}

func TestWebSocket_WriteText(t *testing.T) {
	handler := func(ctx context.Context, ws *WS) error {
		return ws.WriteText(ctx, "hello text")
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn := dialWS(context.Background(), t, wsURL, nil)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(data) != "hello text" {
		t.Errorf("expected %q, got %q", "hello text", string(data))
	}
}

func TestWebSocket_StateInjection(t *testing.T) {
	type appState struct {
		Message string
	}

	state := appState{Message: "from state"}

	var received atomic.Value

	handler := func(ctx context.Context, ws *WS) error {
		s := MustGetState[appState](ctx)
		received.Store(s.Message)
		return ws.WriteText(ctx, s.Message)
	}

	router := Portafilter().
		WithState(state).
		Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn := dialWS(context.Background(), t, wsURL, nil)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(data) != "from state" {
		t.Errorf("expected %q, got %q", "from state", string(data))
	}

	if received.Load() != "from state" {
		t.Errorf("state not injected: got %v", received.Load())
	}
}

func TestWebSocket_ClientDisconnect(t *testing.T) {
	var handlerDone atomic.Bool

	handler := func(ctx context.Context, ws *WS) error {
		defer handlerDone.Store(true)
		<-ctx.Done()
		return nil
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, resp, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	_ = conn.Close(websocket.StatusNormalClosure, "bye")

	time.Sleep(1 * time.Second)

	if !handlerDone.Load() {
		t.Error("handler did not finish after client disconnect")
	}
}

func TestWebSocket_HandlerError(t *testing.T) {
	handler := func(ctx context.Context, ws *WS) error {
		return io.ErrUnexpectedEOF
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn := dialWS(context.Background(), t, wsURL, nil)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, err := conn.Read(ctx)
	if err == nil {
		t.Error("expected connection to be closed due to handler error")
	}
}

type testRequestIDKey struct{}

func TestWebSocket_MiddlewareChain(t *testing.T) {
	var requestIDSeen atomic.Bool

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), testRequestIDKey{}, "req-123"))
			requestIDSeen.Store(true)
			next.ServeHTTP(w, r)
		})
	}

	handler := func(ctx context.Context, ws *WS) error {
		return ws.WriteText(ctx, "ok")
	}

	router := Portafilter().
		Use(middleware).
		Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn := dialWS(context.Background(), t, wsURL, nil)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(data) != "ok" {
		t.Errorf("expected %q, got %q", "ok", string(data))
	}

	if !requestIDSeen.Load() {
		t.Error("middleware was not executed")
	}
}

func TestWebSocket_CloseCode(t *testing.T) {
	tests := []struct {
		name     string
		code     CloseCode
		expected websocket.StatusCode
	}{
		{"normal", CloseNormal, websocket.StatusNormalClosure},
		{"going away", CloseGoingAway, websocket.StatusGoingAway},
		{"internal error", CloseInternalError, websocket.StatusInternalError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if websocket.StatusCode(tc.code) != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, websocket.StatusCode(tc.code))
			}
		})
	}
}

func TestWebSocket_Subprotocol(t *testing.T) {
	handler := func(ctx context.Context, ws *WS) error {
		proto := ws.Subprotocol()
		return ws.WriteText(ctx, "proto: "+proto)
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler,
		WithPingInterval(0),
		WithSubprotocols("my-protocol"),
	))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	opts := &websocket.DialOptions{
		Subprotocols: []string{"my-protocol"},
	}
	conn := dialWS(context.Background(), t, wsURL, opts)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(data) != "proto: my-protocol" {
		t.Errorf("expected %q, got %q", "proto: my-protocol", string(data))
	}
}

func TestWebSocket_PathExtractor(t *testing.T) {
	type pathReq struct {
		Room string `path:"room"`
	}

	handler := func(ctx context.Context, req *extractor.Path[pathReq], ws *WS) error {
		return ws.WriteText(ctx, "room: "+req.Data.Room)
	}

	router := Portafilter().Get("/ws/{room}", WebSocket(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/lobby"
	conn := dialWS(context.Background(), t, wsURL, nil)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(data) != "room: lobby" {
		t.Errorf("expected %q, got %q", "room: lobby", string(data))
	}
}

func TestWebSocket_PanicRecovery(t *testing.T) {
	handler := func(ctx context.Context, ws *WS) error {
		panic("test panic")
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn := dialWS(context.Background(), t, wsURL, nil)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, err := conn.Read(ctx)
	if err == nil {
		t.Error("expected connection to be closed after panic")
	}
}

func TestWebSocket_ContextCancellation(t *testing.T) {
	var contextDone atomic.Bool

	handler := func(ctx context.Context, ws *WS) error {
		<-ctx.Done()
		contextDone.Store(true)
		return nil
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, resp, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	_ = conn.Close(websocket.StatusNormalClosure, "test done")
	time.Sleep(1 * time.Second)

	if !contextDone.Load() {
		t.Error("handler context was not canceled after client disconnect")
	}
}

func TestWebSocket_GracefulShutdown(t *testing.T) {
	// End-to-end: verify gracefulShutdown sends close code 1001 to connected
	// WebSocket clients before shutting the HTTP server down.
	handler := func(ctx context.Context, ws *WS) error {
		<-ctx.Done()
		return nil
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))

	httpSrv := httptest.NewServer(router)
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer dialCancel()
	conn, resp, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	deadline := time.Now().Add(time.Second)
	for router.wsReg.len() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if router.wsReg.len() == 0 {
		t.Fatal("expected WebSocket to be registered on router.wsReg")
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		router.gracefulShutdown(context.Background(), httpSrv.Config, 2*time.Second)
	}()

	readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer readCancel()
	_, _, readErr := conn.Read(readCtx)
	if readErr == nil {
		t.Fatal("expected read to fail after graceful shutdown")
	}
	if got := websocket.CloseStatus(readErr); got != websocket.StatusGoingAway {
		t.Errorf("expected close status %d (StatusGoingAway), got %d (err=%v)", websocket.StatusGoingAway, got, readErr)
	}

	select {
	case <-shutdownDone:
	case <-time.After(3 * time.Second):
		t.Error("gracefulShutdown did not return within timeout")
	}
}

func TestWSRegistry_AddRemove(t *testing.T) {
	reg := newWSRegistry()

	if reg.len() != 0 {
		t.Errorf("expected 0 connections, got %d", reg.len())
	}

	ws1 := &WS{}
	ws1.closed.Store(true)
	ws2 := &WS{}
	ws2.closed.Store(true)

	reg.add(ws1)
	if reg.len() != 1 {
		t.Errorf("expected 1 connection, got %d", reg.len())
	}

	reg.add(ws2)
	if reg.len() != 2 {
		t.Errorf("expected 2 connections, got %d", reg.len())
	}

	reg.remove(ws1)
	if reg.len() != 1 {
		t.Errorf("expected 1 connection, got %d", reg.len())
	}

	reg.remove(ws2)
	if reg.len() != 0 {
		t.Errorf("expected 0 connections, got %d", reg.len())
	}
}
