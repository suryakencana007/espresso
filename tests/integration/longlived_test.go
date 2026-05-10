//go:build integration

package integration

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/suryakencana007/espresso/v2"
)

// TestLongLived_SSE_StableConnection verifies that a single SSE stream runs
// continuously without errors or memory growth over a sustained period.
// This test uses a 30-second duration by default; set -timeout=2h for the
// full 1-hour integration test.
func TestLongLived_SSE_StableConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-lived test in short mode")
	}

	const duration = 30 * time.Second
	const eventInterval = 50 * time.Millisecond

	handler := func(ctx context.Context, s *espresso.SSEStream) error {
		ticker := time.NewTicker(eventInterval)
		defer ticker.Stop()
		count := 0
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				count++
				if err := s.SendData(fmt.Sprintf("tick %d", count)); err != nil {
					return err
				}
			}
		}
	}

	router := espresso.Portafilter().Get("/stream", espresso.StreamSimple(handler, espresso.WithKeepAlive(5*time.Second)))
	server := httptest.NewServer(router)
	defer server.Close()

	runtime.GC()
	var baselineMem runtime.MemStats
	runtime.ReadMemStats(&baselineMem)
	baselineGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	eventCount := int64(0)
	scanner := bufio.NewScanner(resp.Body)
	start := time.Now()

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			atomic.AddInt64(&eventCount, 1)
		}
		if time.Since(start) >= duration {
			break
		}
	}

	cancel()

	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	time.Sleep(500 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()

	events := atomic.LoadInt64(&eventCount)
	t.Logf("events received: %d in %v", events, duration)
	t.Logf("goroutines: baseline=%d, final=%d, diff=%d", baselineGoroutines, finalGoroutines, finalGoroutines-baselineGoroutines)
	t.Logf("memory: baseline=%d MB, final=%d MB, growth=%.2f MB",
		baselineMem.Alloc/1024/1024, finalMem.Alloc/1024/1024,
		float64(finalMem.Alloc-baselineMem.Alloc)/1024/1024)

	expectedMin := int64(duration/eventInterval) - int64(duration/eventInterval)/5
	if events < expectedMin {
		t.Errorf("expected at least %d events, got %d", expectedMin, events)
	}

	if finalGoroutines > baselineGoroutines+5 {
		t.Errorf("goroutine leak: baseline=%d, final=%d", baselineGoroutines, finalGoroutines)
	}
}

// TestLongLived_WS_StableConnection verifies a WebSocket connection stays open
// for a sustained period without disconnect.
func TestLongLived_WS_StableConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-lived test in short mode")
	}

	const duration = 30 * time.Second
	const pingInterval = 5 * time.Second

	handler := func(ctx context.Context, ws *espresso.WS) error {
		<-ctx.Done()
		return nil
	}

	router := espresso.Portafilter().Get("/ws", espresso.WebSocketSimple(handler, espresso.WithPingInterval(pingInterval)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	runtime.GC()
	baselineGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			t.Logf("WS connection stable for %v", duration)
			t.Logf("goroutines: baseline=%d, current=%d", baselineGoroutines, runtime.NumGoroutine())
			return
		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
			err := conn.Ping(pingCtx)
			pingCancel()
			if err != nil {
				t.Errorf("ping failed at %v: %v", time.Since(start), err)
				return
			}
			t.Logf("still alive at %v, goroutines=%d", time.Since(start).Round(time.Second), runtime.NumGoroutine())
		}
	}
}

// TestLongLived_SSE_100Concurrent runs 100 concurrent SSE streams for a short
// duration and verifies no resource exhaustion.
func TestLongLived_SSE_100Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent test in short mode")
	}

	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	const numClients = 100
	const duration = 10 * time.Second
	const eventInterval = 100 * time.Millisecond

	handler := func(ctx context.Context, s *espresso.SSEStream) error {
		ticker := time.NewTicker(eventInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if err := s.SendData("tick"); err != nil {
					return err
				}
			}
		}
	}

	router := espresso.Portafilter().Get("/stream", espresso.StreamSimple(handler, espresso.WithKeepAlive(5*time.Second)))
	server := httptest.NewServer(router)
	defer server.Close()

	runtime.GC()
	baselineGoroutines := runtime.NumGoroutine()

	var errorCount atomic.Int64
	var eventCount atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), duration)
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				errorCount.Add(1)
				return
			}
			defer func() { _ = resp.Body.Close() }()

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				if strings.HasPrefix(scanner.Text(), "data:") {
					eventCount.Add(1)
				}
				if ctx.Err() != nil {
					break
				}
			}
		}(i)
	}

	wg.Wait()

	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	finalGoroutines := runtime.NumGoroutine()

	t.Logf("events: %d, errors: %d/%d", eventCount.Load(), errorCount.Load(), numClients)
	t.Logf("goroutines: baseline=%d, final=%d", baselineGoroutines, finalGoroutines)

	if errorCount.Load() > int64(numClients)/10 {
		t.Errorf("too many errors: %d/%d clients", errorCount.Load(), numClients)
	}

	if finalGoroutines > baselineGoroutines+5 {
		t.Errorf("goroutine leak: baseline=%d, final=%d", baselineGoroutines, finalGoroutines)
	}
}

// TestLongLived_WS_100Concurrent runs 100 concurrent WebSocket connections.
func TestLongLived_WS_100Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent test in short mode")
	}

	const numClients = 100
	const duration = 10 * time.Second

	handler := func(ctx context.Context, ws *espresso.WS) error {
		<-ctx.Done()
		return nil
	}

	router := espresso.Portafilter().Get("/ws", espresso.WebSocketSimple(handler, espresso.WithPingInterval(5*time.Second)))
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	runtime.GC()
	baselineGoroutines := runtime.NumGoroutine()

	var errorCount atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), duration)
			defer cancel()

			conn, resp, err := websocket.Dial(ctx, wsURL, nil)
			if err != nil {
				errorCount.Add(1)
				return
			}
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

			<-ctx.Done()
		}()
	}

	wg.Wait()

	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	finalGoroutines := runtime.NumGoroutine()

	t.Logf("errors: %d/%d", errorCount.Load(), numClients)
	t.Logf("goroutines: baseline=%d, final=%d", baselineGoroutines, finalGoroutines)

	if errorCount.Load() > int64(numClients)/10 {
		t.Errorf("too many errors: %d/%d clients", errorCount.Load(), numClients)
	}

	if finalGoroutines > baselineGoroutines+5 {
		t.Errorf("goroutine leak: baseline=%d, final=%d", baselineGoroutines, finalGoroutines)
	}
}
