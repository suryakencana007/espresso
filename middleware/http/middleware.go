package httpmiddleware

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Middleware wraps an HTTP handler with additional behavior.
type Middleware func(http.Handler) http.Handler

// MiddlewareChain combines middleware into a single middleware.
func MiddlewareChain(middleware ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(middleware) - 1; i >= 0; i-- {
			final = middleware[i](final)
		}
		return final
	}
}

// RequestIDKey is the context key type used for request IDs.
type RequestIDKey = struct{}

// RequestIDMiddleware sets or propagates request IDs in context and response headers.
func RequestIDMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateID()
			}

			w.Header().Set("X-Request-ID", requestID)
			ctx := context.WithValue(r.Context(), RequestIDKey{}, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// RecoverMiddleware recovers panics and responds with HTTP 500.
func RecoverMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error().
						Interface("error", err).
						Str("path", r.URL.Path).
						Str("method", r.Method).
						Msg("Panic recovered")

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORSConfig defines CORS policy values used by CORSMiddleware.
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	ExposeHeaders    []string
	MaxAge           int
}

// DefaultCORSConfig is a permissive CORS configuration.
var DefaultCORSConfig = CORSConfig{
	AllowOrigins:     []string{"*"},
	AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	AllowHeaders:     []string{"Content-Type", "Authorization", "X-Request-ID"},
	AllowCredentials: false,
	ExposeHeaders:    []string{},
	MaxAge:           86400,
}

// CORSMiddleware applies CORS headers and handles preflight requests.
func CORSMiddleware(config CORSConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = "*"
			}

			allowed := false
			for _, o := range config.AllowOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if len(config.ExposeHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
				}
			}

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))
				if config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

var gzipPool = sync.Pool{
	New: func() any { return gzip.NewWriter(nil) },
}

// CompressMiddleware applies gzip compression when the client accepts it.
func CompressMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Content-Encoding", "gzip")
			gw, ok := gzipPool.Get().(*gzip.Writer)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			gw.Reset(w)
			defer func() {
				_ = gw.Close()
				gzipPool.Put(gw)
			}()

			wrapped := &gzipResponseWriter{ResponseWriter: w, writer: gw}
			next.ServeHTTP(wrapped, r)
		})
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	_ = w.writer.Flush()
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("responseWriter does not implement http.Hijacker")
}

func (w *gzipResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return fmt.Errorf("responseWriter does not implement http.Pusher")
}

// RateLimiter determines whether a request identified by key is allowed.
type RateLimiter interface {
	Allow(key string) bool
}

// RateLimitMiddleware rejects requests that exceed rate limits.
func RateLimitMiddleware(limiter RateLimiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				key = forwarded
			}

			if !limiter.Allow(key) {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TokenBucketLimiter enforces rate limits using token bucket semantics.
type TokenBucketLimiter struct {
	rate       int
	capacity   int
	mu         sync.RWMutex
	perKey     bool
	buckets    map[string]*tokenBucket
	tokens     int
	lastRefill time.Time
}

type tokenBucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucketLimiter creates a global token bucket limiter.
func NewTokenBucketLimiter(rate, capacity int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		rate:       rate,
		capacity:   capacity,
		perKey:     false,
		buckets:    nil,
		tokens:     capacity,
		lastRefill: time.Now(),
	}
}

// NewTokenBucketLimiterPerKey creates a per-key token bucket limiter.
func NewTokenBucketLimiterPerKey(rate, capacity int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		rate:     rate,
		capacity: capacity,
		perKey:   true,
		buckets:  make(map[string]*tokenBucket),
	}
}

// Allow reports whether a request for the key is allowed.
func (l *TokenBucketLimiter) Allow(key string) bool {
	if l.perKey {
		return l.allowPerKey(key)
	}
	return l.allowGlobal()
}

func (l *TokenBucketLimiter) allowGlobal() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastRefill)
	refill := int(elapsed.Seconds()) * l.rate / int(time.Second.Seconds())

	l.tokens = min(l.tokens+refill, l.capacity)
	l.lastRefill = now

	if l.tokens > 0 {
		l.tokens--
		return true
	}
	return false
}

func (l *TokenBucketLimiter) allowPerKey(key string) bool {
	l.mu.RLock()
	bucket, exists := l.buckets[key]
	l.mu.RUnlock()

	if !exists {
		l.mu.Lock()
		bucket, exists = l.buckets[key]
		if !exists {
			bucket = &tokenBucket{
				tokens:     l.capacity,
				lastRefill: time.Now(),
			}
			l.buckets[key] = bucket
		}
		l.mu.Unlock()
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)
	refill := int(elapsed.Seconds()) * l.rate / int(time.Second.Seconds())

	bucket.tokens = min(bucket.tokens+refill, l.capacity)
	bucket.lastRefill = now

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}
	return false
}

// SlidingWindowLimiter enforces rate limits using a sliding window.
type SlidingWindowLimiter struct {
	window          time.Duration
	maxReq          int
	requests        map[string][]time.Time
	mu              sync.RWMutex
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

// NewSlidingWindowLimiter creates a sliding-window limiter.
func NewSlidingWindowLimiter(window time.Duration, maxReq int) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		window:          window,
		maxReq:          maxReq,
		requests:        make(map[string][]time.Time),
		cleanupInterval: window,
		lastCleanup:     time.Now(),
	}
}

// NewSlidingWindowLimiterWithCleanup creates a sliding-window limiter with custom cleanup interval.
func NewSlidingWindowLimiterWithCleanup(window time.Duration, maxReq int, cleanupInterval time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		window:          window,
		maxReq:          maxReq,
		requests:        make(map[string][]time.Time),
		cleanupInterval: cleanupInterval,
		lastCleanup:     time.Now(),
	}
}

func (l *SlidingWindowLimiter) cleanup() {
	now := time.Now()
	windowStart := now.Add(-l.window)

	for key, times := range l.requests {
		valid := make([]time.Time, 0, len(times))
		for _, t := range times {
			if t.After(windowStart) {
				valid = append(valid, t)
			}
		}

		if len(valid) == 0 {
			delete(l.requests, key)
		} else if len(valid) < len(times) {
			l.requests[key] = valid
		}
	}

	l.lastCleanup = now
}

// Allow reports whether a request for the key is allowed.
func (l *SlidingWindowLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	if now.Sub(l.lastCleanup) >= l.cleanupInterval {
		l.cleanup()
	}

	windowStart := now.Add(-l.window)

	requests := l.requests[key]
	valid := make([]time.Time, 0, len(requests))
	for _, t := range requests {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.maxReq {
		l.requests[key] = valid
		return false
	}

	valid = append(valid, now)
	l.requests[key] = valid
	return true
}

// AuthValidator validates a request and returns a derived context.
type AuthValidator interface {
	Validate(r *http.Request) (context.Context, error)
}

// AuthMiddleware validates auth before forwarding to next handler.
func AuthMiddleware(validator AuthValidator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, err := validator.Validate(r)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthKey is the context key type used for authentication data.
type AuthKey = struct{}

// LoggingMiddleware logs method, path, status, duration, and request ID.
func LoggingMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapped.status).
				Dur("duration", duration).
				Str("request_id", GetRequestID(r.Context())).
				Msg("HTTP request")
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
