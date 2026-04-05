package espresso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServerOptions(t *testing.T) {
	t.Run("WithAddr", func(t *testing.T) {
		cfg := defaultConfig
		opt := WithAddr(":3000")
		opt(&cfg)
		if cfg.Addr != ":3000" {
			t.Errorf("expected Addr :3000, got %s", cfg.Addr)
		}
	})

	t.Run("WithReadTimeout", func(t *testing.T) {
		cfg := defaultConfig
		opt := WithReadTimeout(5 * time.Second)
		opt(&cfg)
		if cfg.ReadTimeout != 5*time.Second {
			t.Errorf("expected ReadTimeout 5s, got %v", cfg.ReadTimeout)
		}
	})

	t.Run("WithWriteTimeout", func(t *testing.T) {
		cfg := defaultConfig
		opt := WithWriteTimeout(5 * time.Second)
		opt(&cfg)
		if cfg.WriteTimeout != 5*time.Second {
			t.Errorf("expected WriteTimeout 5s, got %v", cfg.WriteTimeout)
		}
	})

	t.Run("WithIdleTimeout", func(t *testing.T) {
		cfg := defaultConfig
		opt := WithIdleTimeout(30 * time.Second)
		opt(&cfg)
		if cfg.IdleTimeout != 30*time.Second {
			t.Errorf("expected IdleTimeout 30s, got %v", cfg.IdleTimeout)
		}
	})

	t.Run("WithReadHeaderTimeout", func(t *testing.T) {
		cfg := defaultConfig
		opt := WithReadHeaderTimeout(2 * time.Second)
		opt(&cfg)
		if cfg.ReadHeaderTimeout != 2*time.Second {
			t.Errorf("expected ReadHeaderTimeout 2s, got %v", cfg.ReadHeaderTimeout)
		}
	})

	t.Run("WithShutdownTimeout", func(t *testing.T) {
		cfg := defaultConfig
		opt := WithShutdownTimeout(5 * time.Second)
		opt(&cfg)
		if cfg.ShutdownTimeout != 5*time.Second {
			t.Errorf("expected ShutdownTimeout 5s, got %v", cfg.ShutdownTimeout)
		}
	})

	t.Run("multiple options", func(t *testing.T) {
		cfg := defaultConfig
		opts := []ServerOption{
			WithAddr(":4000"),
			WithReadTimeout(1 * time.Second),
			WithWriteTimeout(2 * time.Second),
			WithIdleTimeout(3 * time.Second),
			WithReadHeaderTimeout(500 * time.Millisecond),
			WithShutdownTimeout(4 * time.Second),
		}
		for _, opt := range opts {
			opt(&cfg)
		}

		if cfg.Addr != ":4000" {
			t.Errorf("expected Addr :4000, got %s", cfg.Addr)
		}
		if cfg.ReadTimeout != 1*time.Second {
			t.Errorf("expected ReadTimeout 1s, got %v", cfg.ReadTimeout)
		}
		if cfg.WriteTimeout != 2*time.Second {
			t.Errorf("expected WriteTimeout 2s, got %v", cfg.WriteTimeout)
		}
		if cfg.IdleTimeout != 3*time.Second {
			t.Errorf("expected IdleTimeout 3s, got %v", cfg.IdleTimeout)
		}
		if cfg.ReadHeaderTimeout != 500*time.Millisecond {
			t.Errorf("expected ReadHeaderTimeout 500ms, got %v", cfg.ReadHeaderTimeout)
		}
		if cfg.ShutdownTimeout != 4*time.Second {
			t.Errorf("expected ShutdownTimeout 4s, got %v", cfg.ShutdownTimeout)
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	if defaultConfig.Addr != ":8080" {
		t.Errorf("expected default Addr :8080, got %s", defaultConfig.Addr)
	}
	if defaultConfig.ReadTimeout != 10*time.Second {
		t.Errorf("expected default ReadTimeout 10s, got %v", defaultConfig.ReadTimeout)
	}
	if defaultConfig.WriteTimeout != 10*time.Second {
		t.Errorf("expected default WriteTimeout 10s, got %v", defaultConfig.WriteTimeout)
	}
	if defaultConfig.IdleTimeout != 60*time.Second {
		t.Errorf("expected default IdleTimeout 60s, got %v", defaultConfig.IdleTimeout)
	}
	if defaultConfig.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("expected default ReadHeaderTimeout 5s, got %v", defaultConfig.ReadHeaderTimeout)
	}
	if defaultConfig.ShutdownTimeout != 10*time.Second {
		t.Errorf("expected default ShutdownTimeout 10s, got %v", defaultConfig.ShutdownTimeout)
	}
}

func TestRouterIntegration(t *testing.T) {
	t.Run("router handles HTTP requests", func(t *testing.T) {
		router := Portafilter()

		router.Get("/health", Ristretto(func() Text {
			return Text{Body: "OK", StatusCode: http.StatusOK}
		}))

		router.Post("/users", Doppio(func(ctx context.Context, req *JSON[CreateUserReq]) (JSON[User], error) {
			return JSON[User]{Data: User{ID: 1, Name: req.Data.Name}}, nil
		}))

		t.Run("GET /health", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/health", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rec.Code)
			}
			if rec.Body.String() != "OK" {
				t.Errorf("expected body OK, got %s", rec.Body.String())
			}
		})

		t.Run("POST /users", func(t *testing.T) {
			body := `{"name":"test user"}`
			req := httptest.NewRequest("POST", "/users", stringToReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rec.Code)
			}
		})
	})

	t.Run("router with middleware", func(t *testing.T) {
		router := Portafilter()

		var middlewareCalled bool
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				middlewareCalled = true
				next.ServeHTTP(w, r)
			})
		})

		router.Get("/test", Ristretto(func() Text {
			return Text{Body: "test", StatusCode: http.StatusOK}
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if !middlewareCalled {
			t.Error("expected middleware to be called")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("router with multiple routes", func(t *testing.T) {
		router := Portafilter()

		router.Get("/api/v1/users", Ristretto(func() Text {
			return Text{Body: "list users", StatusCode: http.StatusOK}
		}))

		router.Post("/api/v1/users", Ristretto(func() Text {
			return Text{Body: "create user", StatusCode: http.StatusCreated}
		}))

		router.Get("/api/v1/users/{id}", Ristretto(func() Text {
			return Text{Body: "get user", StatusCode: http.StatusOK}
		}))

		tests := []struct {
			method string
			path   string
			status int
		}{
			{"GET", "/api/v1/users", http.StatusOK},
			{"POST", "/api/v1/users", http.StatusCreated},
			{"GET", "/api/v1/users/123", http.StatusOK},
		}

		for _, tt := range tests {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Errorf("%s %s: expected status %d, got %d", tt.method, tt.path, tt.status, rec.Code)
			}
		}
	})

	t.Run("router 404 for unknown route", func(t *testing.T) {
		router := Portafilter()

		router.Get("/known", Ristretto(func() Text {
			return Text{Body: "known", StatusCode: http.StatusOK}
		}))

		req := httptest.NewRequest("GET", "/unknown", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rec.Code)
		}
	})
}

func TestBrewWithTestServer(t *testing.T) {
	t.Run("server handles requests concurrently", func(t *testing.T) {
		router := Portafilter()

		var mu sync.Mutex
		var requestCount int

		router.Get("/concurrent", Ristretto(func() Text {
			mu.Lock()
			requestCount++
			mu.Unlock()
			return Text{Body: "ok", StatusCode: http.StatusOK}
		}))

		server := httptest.NewServer(router)
		defer server.Close()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := http.Get(server.URL + "/concurrent")
				if err != nil {
					t.Errorf("request failed: %v", err)
					return
				}
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected status 200, got %d", resp.StatusCode)
				}
			}()
		}
		wg.Wait()

		mu.Lock()
		count := requestCount
		mu.Unlock()

		if count != 10 {
			t.Errorf("expected 10 requests, got %d", count)
		}
	})
}

func TestServerConfigDefaults(t *testing.T) {
	cfg := ServerConfig{
		Addr:              ":9090",
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       90 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		ShutdownTimeout:   15 * time.Second,
	}

	if cfg.Addr != ":9090" {
		t.Errorf("expected Addr :9090, got %s", cfg.Addr)
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Errorf("expected ReadTimeout 15s, got %v", cfg.ReadTimeout)
	}
}

type CreateUserReq struct {
	Name string `json:"name"`
}

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func stringToReader(s string) *strings.Reader {
	return strings.NewReader(s)
}
