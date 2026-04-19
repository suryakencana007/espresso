package espresso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestContextCancellation_SSE(t *testing.T) {
	ctxCanceled := make(chan struct{})

	handler := func(ctx context.Context, s *SSEStream) error {
		_ = s.SendData("hello")
		<-ctx.Done()
		close(ctxCanceled)
		return nil
	}

	router := Portafilter().Get("/stream", StreamSimple(handler))
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	buf := make([]byte, 128)
	_, _ = resp.Body.Read(buf)

	cancel()
	_ = resp.Body.Close()

	select {
	case <-ctxCanceled:
		t.Log("SSE handler ctx.Done() fired after client disconnect")
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler ctx.Done() did not fire within 2s")
	}
}

func TestContextCancellation_WS(t *testing.T) {
	ctxCanceled := make(chan struct{})

	handler := func(ctx context.Context, ws *WS) error {
		<-ctx.Done()
		close(ctxCanceled)
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

	time.Sleep(50 * time.Millisecond)

	_ = conn.Close(websocket.StatusNormalClosure, "test done")

	select {
	case <-ctxCanceled:
		t.Log("WS handler ctx.Done() fired after client close")
	case <-time.After(2 * time.Second):
		t.Fatal("WS handler ctx.Done() did not fire within 2s")
	}
}

func TestContextCancellation_NoGoroutineLeak(t *testing.T) {
	runtime.GC()
	baseline := runtime.NumGoroutine()

	router := Portafilter().Get("/stream", StreamSimple(func(ctx context.Context, s *SSEStream) error {
		<-ctx.Done()
		return nil
	}))

	server := httptest.NewServer(router)
	defer server.Close()

	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			t.Fatalf("iteration %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
		cancel()
		_ = resp.Body.Close()
	}

	time.Sleep(1 * time.Second)
	runtime.GC()

	final := runtime.NumGoroutine()
	if final > baseline+5 {
		t.Errorf("goroutine leak: baseline=%d, final=%d (diff=%d)", baseline, final, final-baseline)
	} else {
		t.Logf("goroutines: baseline=%d, final=%d (diff=%d)", baseline, final, final-baseline)
	}
}
