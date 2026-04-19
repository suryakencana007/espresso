package espresso

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func BenchmarkWebSocket_Echo(b *testing.B) {
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

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn, resp, err := websocket.Dial(context.Background(), wsURL, nil)
		if err != nil {
			b.Fatalf("dial failed: %v", err)
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		err = conn.Write(ctx, websocket.MessageText, []byte("hello"))
		if err != nil {
			_ = conn.Close(websocket.StatusNormalClosure, "")
			cancel()
			b.Fatalf("write failed: %v", err)
		}

		_, _, err = conn.Read(ctx)
		if err != nil {
			_ = conn.Close(websocket.StatusNormalClosure, "")
			cancel()
			b.Fatalf("read failed: %v", err)
		}

		_ = conn.Close(websocket.StatusNormalClosure, "")
		cancel()
	}
}

func BenchmarkWebSocket_WriteText(b *testing.B) {
	handler := func(ctx context.Context, ws *WS) error {
		for i := 0; i < 1000; i++ {
			if err := ws.WriteText(ctx, "hello"); err != nil {
				return err
			}
		}
		return nil
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn, resp, err := websocket.Dial(context.Background(), wsURL, nil)
		if err != nil {
			b.Fatalf("dial failed: %v", err)
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		for j := 0; j < 1000; j++ {
			_, _, err := conn.Read(ctx)
			if err != nil {
				break
			}
		}

		_ = conn.Close(websocket.StatusNormalClosure, "")
		cancel()
	}
}

func BenchmarkWebSocket_ConcurrentReads(b *testing.B) {
	handler := func(ctx context.Context, ws *WS) error {
		for {
			_, data, err := ws.Read(ctx)
			if err != nil {
				return nil
			}
			_ = data
		}
	}

	router := Portafilter().Get("/ws", WebSocketSimple(handler, WithPingInterval(0)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn, resp, err := websocket.Dial(context.Background(), wsURL, nil)
		if err != nil {
			b.Fatalf("dial failed: %v", err)
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)

		for j := 0; j < 100; j++ {
			err := conn.Write(ctx, websocket.MessageText, []byte("hello"))
			if err != nil {
				break
			}
		}

		_ = conn.Close(websocket.StatusNormalClosure, "")
		cancel()
	}
}
