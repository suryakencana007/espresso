package httpmiddleware

import (
	"bufio"
	"compress/gzip"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestIDMiddleware(t *testing.T) {
	t.Run("generates new request ID", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := GetRequestID(r.Context())
			if id == "" {
				t.Error("expected request ID to be set")
			}
			w.WriteHeader(http.StatusOK)
		})

		middleware := RequestIDMiddleware()
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		requestID := rec.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("expected X-Request-ID header to be set")
		}
	})

	t.Run("uses existing request ID from header", func(t *testing.T) {
		existingID := "existing-id-123"
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := GetRequestID(r.Context())
			if id != existingID {
				t.Errorf("expected request ID %s, got %s", existingID, id)
			}
			w.WriteHeader(http.StatusOK)
		})

		middleware := RequestIDMiddleware()
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", existingID)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		requestID := rec.Header().Get("X-Request-ID")
		if requestID != existingID {
			t.Errorf("expected X-Request-ID %s, got %s", existingID, requestID)
		}
	})
}

func TestRecoverMiddleware(t *testing.T) {
	t.Run("recovers from panic", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})

		middleware := RecoverMiddleware()
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, `"PANIC"`) {
			t.Errorf("expected PANIC code in response, got %s", body)
		}
	})

	t.Run("allows normal request to pass through", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

		middleware := RecoverMiddleware()
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		if rec.Body.String() != "OK" {
			t.Errorf("expected body OK, got %s", rec.Body.String())
		}
	})
}

func TestCORSMiddleware(t *testing.T) {
	t.Run("handles preflight request", func(t *testing.T) {
		config := CORSConfig{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"GET", "POST"},
			AllowHeaders:     []string{"Content-Type"},
			AllowCredentials: true,
			MaxAge:           3600,
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called for preflight")
		})

		middleware := CORSMiddleware(config)
		server := middleware(handler)

		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", rec.Code)
		}

		if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
			t.Errorf("expected CORS origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
		}

		if rec.Header().Get("Access-Control-Allow-Methods") != "GET, POST" {
			t.Errorf("expected CORS methods header, got %s", rec.Header().Get("Access-Control-Allow-Methods"))
		}
	})

	t.Run("adds CORS headers to regular request", func(t *testing.T) {
		config := CORSConfig{
			AllowOrigins:     []string{"https://example.com"},
			AllowMethods:     []string{"GET", "POST"},
			AllowHeaders:     []string{"Content-Type"},
			AllowCredentials: true,
			ExposeHeaders:    []string{"X-Custom-Header"},
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := CORSMiddleware(config)
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
			t.Errorf("expected CORS origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
		}

		if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Error("expected CORS credentials header")
		}
	})

	t.Run("rejects disallowed origin", func(t *testing.T) {
		config := CORSConfig{
			AllowOrigins: []string{"https://allowed.com"},
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := CORSMiddleware(config)
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://disallowed.com")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("expected no CORS origin header for disallowed origin")
		}
	})

	t.Run("sets MaxAge header correctly", func(t *testing.T) {
		config := CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST"},
			AllowHeaders: []string{"Content-Type"},
			MaxAge:       86400,
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called for preflight")
		})

		middleware := CORSMiddleware(config)
		server := middleware(handler)

		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		maxAge := rec.Header().Get("Access-Control-Max-Age")
		if maxAge != "86400" {
			t.Errorf("expected Max-Age header '86400', got '%s'", maxAge)
		}
	})
}

func TestCompressMiddleware(t *testing.T) {
	t.Run("compresses response when gzip accepted", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("hello world"))
		})

		middleware := CompressMiddleware()
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Encoding") != "gzip" {
			t.Error("expected Content-Encoding: gzip")
		}

		gr, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer func() { _ = gr.Close() }()

		body, err := io.ReadAll(gr)
		if err != nil {
			t.Fatalf("failed to read gzipped body: %v", err)
		}

		if string(body) != "hello world" {
			t.Errorf("expected body 'hello world', got %s", string(body))
		}
	})

	t.Run("does not compress when gzip not accepted", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("hello world"))
		})

		middleware := CompressMiddleware()
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Encoding") == "gzip" {
			t.Error("expected no gzip encoding")
		}

		if rec.Body.String() != "hello world" {
			t.Errorf("expected body 'hello world', got %s", rec.Body.String())
		}
	})

	t.Run("supports Flush for streaming", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		})

		middleware := CompressMiddleware()
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Encoding") != "gzip" {
			t.Error("expected gzip encoding")
		}
	})

	t.Run("supports WebSocket hijacking", func(t *testing.T) {
		mockHijacker := &mockHijackerResponseWriter{ResponseRecorder: httptest.NewRecorder()}

		gzWriter := gzip.NewWriter(mockHijacker)
		defer func() { _ = gzWriter.Close() }()

		gzipRW := &gzipResponseWriter{
			ResponseWriter: mockHijacker,
			writer:         gzWriter,
		}

		if _, ok := any(gzipRW).(http.Hijacker); !ok {
			t.Error("gzipResponseWriter should implement http.Hijacker")
		}

		conn, rw, err := gzipRW.Hijack()
		if conn != nil || rw != nil {
			t.Error("expected nil values from mock hijacker")
		}
		_ = err
	})
}

//nolint:gocyclo // table-less scenario coverage intentionally exercises many branches
func TestTokenBucketLimiter(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(10, 10)

		for i := 0; i < 10; i++ {
			if !limiter.Allow("key1") {
				t.Errorf("expected request %d to be allowed", i)
			}
		}
	})

	t.Run("rejects requests over limit", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(1, 1)

		if !limiter.Allow("key1") {
			t.Error("expected first request to be allowed")
		}

		if limiter.Allow("key1") {
			t.Error("expected second request to be rejected")
		}
	})

	t.Run("uses global token pool (key parameter is ignored)", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(1, 1)

		if !limiter.Allow("key1") {
			t.Error("expected key1 request to be allowed")
		}

		if limiter.Allow("key2") {
			t.Error("expected key2 request to be rejected (uses global pool)")
		}
	})

	t.Run("per-key limiting allows separate buckets", func(t *testing.T) {
		limiter := NewTokenBucketLimiterPerKey(1, 2)

		if !limiter.Allow("user1") {
			t.Error("expected user1 first request to be allowed")
		}
		if !limiter.Allow("user1") {
			t.Error("expected user1 second request to be allowed")
		}
		if limiter.Allow("user1") {
			t.Error("expected user1 third request to be rejected (bucket exhausted)")
		}

		if !limiter.Allow("user2") {
			t.Error("expected user2 first request to be allowed (separate bucket)")
		}
		if !limiter.Allow("user2") {
			t.Error("expected user2 second request to be allowed (separate bucket)")
		}
		if limiter.Allow("user2") {
			t.Error("expected user2 third request to be rejected (bucket exhausted)")
		}
	})

	t.Run("per-key limiting refills independently", func(t *testing.T) {
		limiter := NewTokenBucketLimiterPerKey(1, 1)

		if !limiter.Allow("user1") {
			t.Error("expected user1 first request to be allowed")
		}
		if limiter.Allow("user1") {
			t.Error("expected user1 second request to be rejected")
		}

		if !limiter.Allow("user2") {
			t.Error("expected user2 to have independent bucket")
		}

		time.Sleep(1100 * time.Millisecond)

		if !limiter.Allow("user1") {
			t.Error("expected user1 to refill after waiting")
		}

		if !limiter.Allow("user2") {
			t.Error("expected user2 to refill independently")
		}
	})

	// Regression: pre-fix the refill math computed
	//   refill := int(elapsed.Seconds()) * l.rate / int(time.Second.Seconds())
	// which truncated to 0 for any elapsed < 1s while advancing lastRefill,
	// starving all traffic under sustained sub-second load. At rate=100/s,
	// cap=5, 30 requests spaced 100ms apart admitted only the initial 5
	// tokens then rejected the remaining 25. Fix credits fractional seconds
	// via int64(elapsed) * int64(rate) / int64(time.Second).
	t.Run("global bucket refills under sustained sub-second traffic", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(100, 5)
		admitted := 0
		for range 30 {
			if limiter.Allow("") {
				admitted++
			}
			time.Sleep(100 * time.Millisecond)
		}
		// 100 rps allowed, 10 rps offered over 3s: ~30 admissions expected.
		// Pre-fix: exactly 5 (initial cap). Guard both directions.
		if admitted < 25 {
			t.Errorf("global limiter starved sub-second traffic: admitted=%d, expected ~30", admitted)
		}
		if admitted > 32 {
			t.Errorf("global limiter over-credited: admitted=%d, expected ~30", admitted)
		}
	})

	t.Run("per-key bucket refills under sustained sub-second traffic", func(t *testing.T) {
		limiter := NewTokenBucketLimiterPerKey(100, 5)
		admitted := 0
		for range 30 {
			if limiter.Allow("k") {
				admitted++
			}
			time.Sleep(100 * time.Millisecond)
		}
		if admitted < 25 {
			t.Errorf("per-key limiter starved sub-second traffic: admitted=%d, expected ~30", admitted)
		}
		if admitted > 32 {
			t.Errorf("per-key limiter over-credited: admitted=%d, expected ~30", admitted)
		}
	})
}

func TestSlidingWindowLimiter(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		limiter := NewSlidingWindowLimiter(time.Second, 5)

		for i := 0; i < 5; i++ {
			if !limiter.Allow("key1") {
				t.Errorf("expected request %d to be allowed", i)
			}
		}
	})

	t.Run("rejects requests over limit", func(t *testing.T) {
		limiter := NewSlidingWindowLimiter(time.Second, 2)

		if !limiter.Allow("key1") {
			t.Error("expected first request to be allowed")
		}

		if !limiter.Allow("key1") {
			t.Error("expected second request to be allowed")
		}

		if limiter.Allow("key1") {
			t.Error("expected third request to be rejected")
		}
	})

	t.Run("window slides over time", func(t *testing.T) {
		limiter := NewSlidingWindowLimiter(100*time.Millisecond, 1)

		if !limiter.Allow("key1") {
			t.Error("expected first request to be allowed")
		}

		if limiter.Allow("key1") {
			t.Error("expected second request to be rejected")
		}

		time.Sleep(150 * time.Millisecond)

		if !limiter.Allow("key1") {
			t.Error("expected request after window to be allowed")
		}
	})

	t.Run("cleanup removes old entries to prevent memory leak", func(t *testing.T) {
		limiter := NewSlidingWindowLimiterWithCleanup(
			50*time.Millisecond,
			10,
			25*time.Millisecond,
		)

		for i := 0; i < 100; i++ {
			key := "unique-key-" + string(rune('0'+i%10))
			limiter.Allow(key)
		}

		limiter.mu.RLock()
		initialCount := len(limiter.requests)
		limiter.mu.RUnlock()

		if initialCount == 0 {
			t.Error("expected some entries after initial requests")
		}

		time.Sleep(60 * time.Millisecond)

		limiter.Allow("new-request")

		limiter.mu.RLock()
		afterCleanupCount := len(limiter.requests)
		limiter.mu.RUnlock()

		if afterCleanupCount >= initialCount {
			t.Errorf("expected cleanup to remove expired entries, but count went from %d to %d", initialCount, afterCleanupCount)
		}
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(10, 10)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := RateLimitMiddleware(limiter)
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("rejects requests over limit", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(1, 1)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := RateLimitMiddleware(limiter)
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.0.2.1:1234"

		rec1 := httptest.NewRecorder()
		server.ServeHTTP(rec1, req)
		if rec1.Code != http.StatusOK {
			t.Errorf("expected first request status 200, got %d", rec1.Code)
		}

		rec2 := httptest.NewRecorder()
		server.ServeHTTP(rec2, req)
		if rec2.Code != http.StatusTooManyRequests {
			t.Errorf("expected second request status 429, got %d", rec2.Code)
		}
	})

	t.Run("uses X-Forwarded-For header", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(1, 1)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := RateLimitMiddleware(limiter)
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")

		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}

type mockAuthValidator struct {
	shouldFail bool
	userInfo   string
}

func (m *mockAuthValidator) Validate(r *http.Request) (context.Context, error) {
	if m.shouldFail {
		return nil, context.DeadlineExceeded
	}
	return context.WithValue(r.Context(), AuthKey{}, m.userInfo), nil
}

func TestAuthMiddleware(t *testing.T) {
	t.Run("allows authenticated requests", func(t *testing.T) {
		validator := &mockAuthValidator{shouldFail: false, userInfo: "user123"}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(AuthKey{}).(string)
			if !ok {
				t.Error("expected user value in context")
			}
			if user != "user123" {
				t.Errorf("expected user user123, got %s", user)
			}
			w.WriteHeader(http.StatusOK)
		})

		middleware := AuthMiddleware(validator)
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("rejects unauthenticated requests", func(t *testing.T) {
		validator := &mockAuthValidator{shouldFail: true, userInfo: ""}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called for unauthenticated request")
		})

		middleware := AuthMiddleware(validator)
		server := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("logs request and calls handler", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created"))
		})

		middleware := LoggingMiddleware()
		server := middleware(handler)

		req := httptest.NewRequest("POST", "/test", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rec.Code)
		}
	})

	// Regression: pre-fix statusRecorder embedded http.ResponseWriter and
	// overrode only WriteHeader — it implemented neither http.Flusher,
	// http.Hijacker, nor Unwrap() for http.ResponseController. So installing
	// LoggingMiddleware broke first-party features: SSE routes hit the
	// w.(http.Flusher) assertion at sse.go:346 and returned 500 "streaming
	// not supported"; coder/websocket's Accept could not hijack. This test
	// asserts that after the middleware wraps a Flusher/Hijacker/Pusher
	// underlying writer, the wrapper still satisfies those interfaces.
	t.Run("statusRecorder forwards Flusher/Hijacker/Pusher and Unwrap", func(t *testing.T) {
		// Use a real *http.Server + net.Listen so the underlying
		// http.response satisfies Flusher and Hijacker (httptest.NewRecorder
		// implements Flusher but not Hijacker, so we serve for real).
		var (
			sawFlusher  bool
			sawHijacker bool
			sawUnwrap   bool
		)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := w.(http.Flusher); ok {
				sawFlusher = true
			}
			if _, ok := w.(http.Hijacker); ok {
				sawHijacker = true
			}
			// http.ResponseController uses Unwrap() to walk wrapper chains.
			// SetWriteDeadline returns ErrNotSupported if any wrapper in the
			// chain lacks Unwrap(); a nil error proves Unwrap() forwards.
			if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err == nil {
				sawUnwrap = true
			}
			w.WriteHeader(http.StatusOK)
		})

		middleware := LoggingMiddleware()
		server := middleware(handler)

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer func() { _ = ln.Close() }()
		srv := &http.Server{Handler: server, ReadHeaderTimeout: 5 * time.Second}
		go func() { _ = srv.Serve(ln) }()
		defer func() { _ = srv.Close() }()

		resp, err := http.Get("http://" + ln.Addr().String() + "/test")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		_ = resp.Body.Close()

		if !sawFlusher {
			t.Error("statusRecorder must forward http.Flusher (breaks SSE)")
		}
		if !sawHijacker {
			t.Error("statusRecorder must forward http.Hijacker (breaks WS upgrade)")
		}
		if !sawUnwrap {
			t.Error("statusRecorder must implement Unwrap() so http.ResponseController walks the chain")
		}
	})
}

func TestMiddlewareChain(t *testing.T) {
	t.Run("applies middleware in order", func(t *testing.T) {
		var order []string

		middleware1 := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "m1-before")
				next.ServeHTTP(w, r)
				order = append(order, "m1-after")
			})
		}

		middleware2 := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "m2-before")
				next.ServeHTTP(w, r)
				order = append(order, "m2-after")
			})
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "handler")
			w.WriteHeader(http.StatusOK)
		})

		chain := MiddlewareChain(middleware1, middleware2)
		server := chain(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
		if len(order) != len(expected) {
			t.Errorf("expected %d calls, got %d", len(expected), len(order))
		}

		for i, call := range expected {
			if order[i] != call {
				t.Errorf("expected order[%d] = %s, got %s", i, call, order[i])
			}
		}
	})
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if len(id1) != 32 {
		t.Errorf("expected ID length 32, got %d", len(id1))
	}

	if id1 == id2 {
		t.Error("expected different IDs")
	}
}

type mockHijackerResponseWriter struct {
	*httptest.ResponseRecorder
}

func (m *mockHijackerResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}

// TestRateLimit_KeysOnHostNotHostPort locks D1a from v2.4 task-08. Pre-fix,
// RateLimitMiddleware keyed on the full r.RemoteAddr (host:port). Because
// every new TCP connection gets a fresh ephemeral port, a client could
// bypass a per-key limiter simply by reconnecting between requests. Post-
// fix, clientKey extracts the host portion via net.SplitHostPort so both
// requests from the same client IP land in the same bucket regardless
// of port.
func TestRateLimit_KeysOnHostNotHostPort(t *testing.T) {
	cfg := &rateLimitConfig{}

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.0.2.1:1234"
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.0.2.1:5678"

	key1 := clientKey(req1, cfg)
	key2 := clientKey(req2, cfg)

	if key1 != "192.0.2.1" {
		t.Errorf("expected key1 = '192.0.2.1', got %q (pre-fix behavior: keyed on host:port)", key1)
	}
	if key1 != key2 {
		t.Errorf("expected same key for same host, got %q vs %q (pre-fix: ephemeral port bypass)", key1, key2)
	}
}

// TestRateLimit_XFFNotTrustedByDefault locks D1b. Pre-fix, the middleware
// unconditionally trusted X-Forwarded-For, so a client that connected
// directly to the server could mint unlimited keys by rotating XFF
// values (or spoof another client's identity). Post-fix, XFF is IGNORED
// unless the request's RemoteAddr host is in a WithTrustedProxies CIDR.
func TestRateLimit_XFFNotTrustedByDefault(t *testing.T) {
	cfg := &rateLimitConfig{} // no trusted proxies

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.0.2.1:1234"
	req1.Header.Set("X-Forwarded-For", "1.1.1.1")

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.0.2.1:1234"
	req2.Header.Set("X-Forwarded-For", "2.2.2.2")

	key1 := clientKey(req1, cfg)
	key2 := clientKey(req2, cfg)

	// Both requests are from the same actual client, so their keys must match.
	// Pre-fix: key1="1.1.1.1", key2="2.2.2.2" (XFF trusted unconditionally).
	if key1 != "192.0.2.1" || key2 != "192.0.2.1" {
		t.Errorf("expected both keys to be RemoteAddr host, got %q and %q (pre-fix: XFF blindly trusted)", key1, key2)
	}
}

// TestRateLimit_TrustedProxyTakesRightmostHop locks the RFC 7239 hop
// selection logic. When RemoteAddr IS in a trusted CIDR and XFF contains
// a chain of proxies, the middleware walks right-to-left, skips trusted
// hops, and returns the first non-trusted address as the client key.
func TestRateLimit_TrustedProxyTakesRightmostHop(t *testing.T) {
	_, trustedNet, _ := net.ParseCIDR("192.168.0.0/16")
	cfg := &rateLimitConfig{trustedProxies: []*net.IPNet{trustedNet}}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.0.1:5678"
	// XFF chain: attacker-supplied leftmost, real client middle, trusted proxy rightmost.
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 203.0.113.5, 192.168.0.2")

	key := clientKey(req, cfg)

	// Walk right-to-left: 192.168.0.2 is trusted (skip), 203.0.113.5 is not
	// (return). The attacker's spoofed leftmost value must never be used.
	if key != "203.0.113.5" {
		t.Errorf("expected key '203.0.113.5' (rightmost non-trusted hop), got %q", key)
	}
}

// TestRateLimit_UntrustedRemoteAddrIgnoresXFF verifies the trust gate:
// when RemoteAddr is NOT in a trusted CIDR, XFF is ignored even if the
// option is configured. This prevents an attacker who connects directly
// (bypassing the proxy) from spoofing via XFF.
func TestRateLimit_UntrustedRemoteAddrIgnoresXFF(t *testing.T) {
	_, trustedNet, _ := net.ParseCIDR("192.168.0.0/16")
	cfg := &rateLimitConfig{trustedProxies: []*net.IPNet{trustedNet}}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:9999" // NOT in trusted CIDR
	req.Header.Set("X-Forwarded-For", "attacker-value")

	key := clientKey(req, cfg)

	if key != "1.2.3.4" {
		t.Errorf("expected RemoteAddr host '1.2.3.4' (untrusted client → XFF ignored), got %q", key)
	}
}

// TestRateLimit_WithTrustedProxiesParsesBareIP verifies WithTrustedProxies
// accepts both CIDR notation and bare IPs (auto-wrapped to /32 or /128).
func TestRateLimit_WithTrustedProxiesParsesBareIP(t *testing.T) {
	cfg := &rateLimitConfig{}
	WithTrustedProxies("10.0.0.5", "2001:db8::1", "192.168.0.0/16", "invalid-junk")(cfg)

	if len(cfg.trustedProxies) != 3 {
		t.Fatalf("expected 3 parsed CIDRs (bare IPv4, bare IPv6, CIDR; junk skipped), got %d", len(cfg.trustedProxies))
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")

	if key := clientKey(req, cfg); key != "203.0.113.9" {
		t.Errorf("expected bare-IP-trusted proxy to honor XFF hop, got %q", key)
	}
}

// TestTokenBucketLimiterPerKey_EvictsIdleBuckets locks D2: the pre-fix
// per-key map only grew — combined with the pre-fix XFF-spoof bug this
// was an unbounded memory-growth DoS. Post-fix, a background sweeper
// removes buckets whose lastRefill is older than bucketTTL.
func TestTokenBucketLimiterPerKey_EvictsIdleBuckets(t *testing.T) {
	limiter := NewTokenBucketLimiterPerKey(100, 10, WithBucketTTL(80*time.Millisecond))
	defer func() { _ = limiter.Close() }()

	limiter.Allow("key-a")
	limiter.Allow("key-b")

	limiter.mu.RLock()
	sizeBefore := len(limiter.buckets)
	limiter.mu.RUnlock()
	if sizeBefore != 2 {
		t.Fatalf("expected 2 buckets after two Allow calls, got %d", sizeBefore)
	}

	// Sleep past 3 sweep intervals (bucketTTL/2 * 3 = 120ms) so idle
	// buckets get swept. Buckets are stale after bucketTTL=80ms.
	time.Sleep(200 * time.Millisecond)

	// Poke a third key so its bucket exists and is fresh.
	limiter.Allow("key-c")

	limiter.mu.RLock()
	sizeAfter := len(limiter.buckets)
	_, hasA := limiter.buckets["key-a"]
	_, hasB := limiter.buckets["key-b"]
	_, hasC := limiter.buckets["key-c"]
	limiter.mu.RUnlock()

	if hasA || hasB {
		t.Errorf("expected idle key-a and key-b to be evicted (pre-fix behavior: map only grows), got hasA=%v hasB=%v", hasA, hasB)
	}
	if !hasC {
		t.Error("expected fresh key-c to still be present")
	}
	if sizeAfter != 1 {
		t.Errorf("expected 1 bucket after eviction, got %d", sizeAfter)
	}
}
