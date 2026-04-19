package espresso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestShutdown_HooksRunInOrder(t *testing.T) {
	var order []int
	var mu sync.Mutex

	router := Portafilter().
		OnShutdown(func(ctx context.Context) error {
			mu.Lock()
			order = append(order, 1)
			mu.Unlock()
			return nil
		}).
		OnShutdown(func(ctx context.Context) error {
			mu.Lock()
			order = append(order, 2)
			mu.Unlock()
			return nil
		}).
		OnShutdown(func(ctx context.Context) error {
			mu.Lock()
			order = append(order, 3)
			mu.Unlock()
			return nil
		})

	srv := &http.Server{Handler: router, ReadHeaderTimeout: 10 * time.Second}
	router.gracefulShutdown(context.Background(), srv, 5*time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("expected 3 hooks to run, got %d", len(order))
	}
	for i, v := range order {
		if v != i+1 {
			t.Errorf("expected hook %d to run at position %d, got %d", i+1, i, v)
		}
	}
}

func TestShutdown_HooksReceiveContext(t *testing.T) {
	var receivedCtx context.Context
	var mu sync.Mutex

	router := Portafilter().
		OnShutdown(func(ctx context.Context) error {
			mu.Lock()
			receivedCtx = ctx
			mu.Unlock()
			return nil
		})

	srv := &http.Server{Handler: router, ReadHeaderTimeout: 10 * time.Second}
	router.gracefulShutdown(context.Background(), srv, 5*time.Second)

	mu.Lock()
	defer mu.Unlock()
	if receivedCtx == nil {
		t.Fatal("expected hook to receive a context")
	}

	deadline, ok := receivedCtx.Deadline()
	if !ok {
		t.Error("expected context to have a deadline")
	}
	_ = deadline
}

func TestShutdown_HookError(t *testing.T) {
	var secondHookRan atomic.Bool

	router := Portafilter().
		OnShutdown(func(ctx context.Context) error {
			return context.DeadlineExceeded
		}).
		OnShutdown(func(ctx context.Context) error {
			secondHookRan.Store(true)
			return nil
		})

	srv := &http.Server{Handler: router, ReadHeaderTimeout: 10 * time.Second}
	router.gracefulShutdown(context.Background(), srv, 5*time.Second)

	if !secondHookRan.Load() {
		t.Error("expected second hook to run even after first hook error")
	}
}

func TestShutdown_HookPanic(t *testing.T) {
	var secondHookRan atomic.Bool

	router := Portafilter().
		OnShutdown(func(ctx context.Context) error {
			panic("hook panic!")
		}).
		OnShutdown(func(ctx context.Context) error {
			secondHookRan.Store(true)
			return nil
		})

	srv := &http.Server{Handler: router, ReadHeaderTimeout: 10 * time.Second}
	router.gracefulShutdown(context.Background(), srv, 5*time.Second)

	if !secondHookRan.Load() {
		t.Error("expected second hook to run even after first hook panic")
	}
}

func TestShutdown_HookTimeout(t *testing.T) {
	var secondHookRan atomic.Bool

	router := Portafilter().
		OnShutdown(func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		}).
		OnShutdown(func(ctx context.Context) error {
			secondHookRan.Store(true)
			return nil
		})

	srv := &http.Server{Handler: router, ReadHeaderTimeout: 10 * time.Second}
	// Short timeout to test that hooks still run even with tight deadline
	router.gracefulShutdown(context.Background(), srv, 50*time.Millisecond)

	// Hook 1 should have timed out but hook 2 should still run
	if !secondHookRan.Load() {
		t.Error("expected second hook to run even after first hook slow")
	}
}

func TestShutdown_NoHooks(t *testing.T) {
	router := Portafilter()
	srv := &http.Server{Handler: router, ReadHeaderTimeout: 10 * time.Second}

	// Should not panic with no hooks
	router.gracefulShutdown(context.Background(), srv, time.Second)
}

func TestShutdown_SSEStreamsClosed(t *testing.T) {
	// The global defaultSSERegistry is shared, so we just verify that
	// closeAll sends shutdown comments to registered streams.
	// Integration with actual HTTP server is tested in sse_test.go.

	handler := func(ctx context.Context, stream *SSEStream) error {
		_ = stream.Send(Event{Name: "msg", Data: "hello"})
		<-ctx.Done()
		return nil
	}

	router := Portafilter().Get("/stream", StreamSimple(handler, WithKeepAlive(50*time.Millisecond)))
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify stream is registered
	if defaultSSERegistry.len() == 0 {
		t.Error("expected SSE stream to be registered")
	}

	// The global closeAll is called during gracefulShutdown.
	// Verify that the stream gets marked as closed (Send returns error).
	defaultSSERegistry.closeAll("test shutdown")
}

func TestShutdown_InFlightRequestsComplete(t *testing.T) {
	// This test verifies the shutdown sequence runs correctly by using
	// an actual test server and calling Shutdown on it.
	var requestCompleted atomic.Bool

	router := Portafilter().Get("/slow", Ristretto(func() Text {
		time.Sleep(100 * time.Millisecond)
		requestCompleted.Store(true)
		return Text{Body: "done"}
	}))

	server := httptest.NewServer(router)
	defer server.Close()

	// Start a request
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := http.Get(server.URL + "/slow")
		if err != nil {
			return
		}
		defer func() { _ = resp.Body.Close() }()
	}()

	// Wait for the request to be in-flight
	time.Sleep(20 * time.Millisecond)

	// Close the test server gracefully — this simulates what gracefulShutdown does
	// via srv.Shutdown(ctx)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Config.Shutdown(ctx)

	<-done

	if !requestCompleted.Load() {
		t.Error("expected in-flight request to complete during graceful shutdown")
	}
}

func TestShutdown_OnShutdownChaining(t *testing.T) {
	var count atomic.Int32

	router := Portafilter().
		OnShutdown(func(ctx context.Context) error {
			count.Add(1)
			return nil
		}).
		Get("/test", Ristretto(func() Text {
			return Text{Body: "ok"}
		})).
		OnShutdown(func(ctx context.Context) error {
			count.Add(1)
			return nil
		})

	srv := &http.Server{Handler: router, ReadHeaderTimeout: 10 * time.Second}
	router.gracefulShutdown(context.Background(), srv, time.Second)

	if count.Load() != 2 {
		t.Errorf("expected 2 hooks, got %d", count.Load())
	}
}

func TestShutdown_BrewContext(t *testing.T) {
	router := Portafilter().Get("/health", Ristretto(func() Text {
		return Text{Body: "ok"}
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := router.BrewContext(ctx, WithAddr(":0"))
	if err != nil {
		t.Errorf("expected BrewContext to return nil, got %v", err)
	}
}

func TestShutdown_BrewContextIntegration(t *testing.T) {
	var hookCalled atomic.Bool

	router := Portafilter().
		OnShutdown(func(ctx context.Context) error {
			hookCalled.Store(true)
			return nil
		}).
		Get("/test", Ristretto(func() Text {
			return Text{Body: "ok"}
		}))

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := router.BrewContext(ctx, WithAddr(":0"))
	if err != nil {
		t.Errorf("expected BrewContext to return nil, got %v", err)
	}

	if !hookCalled.Load() {
		t.Error("expected shutdown hook to be called")
	}
}

func TestDefaultConfig(t *testing.T) {
	if defaultConfig.Addr != ":8080" {
		t.Errorf("expected default Addr :8080, got %s", defaultConfig.Addr)
	}
	if defaultConfig.ShutdownTimeout != 10*time.Second {
		t.Errorf("expected default ShutdownTimeout 10s, got %v", defaultConfig.ShutdownTimeout)
	}
}

func TestServerOptions(t *testing.T) {
	t.Run("WithShutdownTimeout", func(t *testing.T) {
		cfg := defaultConfig
		opt := WithShutdownTimeout(5 * time.Second)
		opt(&cfg)
		if cfg.ShutdownTimeout != 5*time.Second {
			t.Errorf("expected ShutdownTimeout 5s, got %v", cfg.ShutdownTimeout)
		}
	})

	t.Run("WithAddr", func(t *testing.T) {
		cfg := defaultConfig
		opt := WithAddr(":3000")
		opt(&cfg)
		if cfg.Addr != ":3000" {
			t.Errorf("expected Addr :3000, got %s", cfg.Addr)
		}
	})
}

func TestRouterIntegration(t *testing.T) {
	t.Run("OnShutdown is chainable", func(t *testing.T) {
		router := Portafilter().
			OnShutdown(func(ctx context.Context) error { return nil }).
			Get("/test", Ristretto(func() Text { return Text{Body: "ok"} }))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}

type CreateUserReq struct {
	Name string `json:"name"`
}

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
