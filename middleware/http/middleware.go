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
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/suryakencana007/espresso/v2/internal/errorenvelope"
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
type RequestIDKey struct{}

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

// RecoverMiddleware recovers panics and responds with a structured JSON error.
func RecoverMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()

					_ = rec // panic value used for logging above

					log.Error().
						Interface("error", rec).
						Str("path", r.URL.Path).
						Str("method", r.Method).
						Str("request_id", GetRequestID(r.Context())).
						Str("stack", string(stack)).
						Msg("Panic recovered")

					// Emit the canonical error envelope via the cycle-safe leaf
					// so a recovered panic is byte-identical to a writeHandlerError
					// 500. Details is left nil -> omitempty keeps the "details" key
					// absent, matching the root package's 500 exactly.
					errorenvelope.Write(w, http.StatusInternalServerError, errorenvelope.Body{
						Code:      "PANIC",
						Message:   "internal server error",
						RequestID: GetRequestID(r.Context()),
					})
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

// RateLimitOption configures RateLimitMiddleware behavior.
type RateLimitOption func(*rateLimitConfig)

type rateLimitConfig struct {
	trustedProxies []*net.IPNet
}

// WithTrustedProxies configures RateLimitMiddleware to trust the
// X-Forwarded-For header ONLY when the request's RemoteAddr host is in
// one of the given CIDR blocks. Each entry accepts CIDR notation
// ("10.0.0.0/8", "2001:db8::/32") or a bare IP (auto-wrapped as /32 for
// IPv4, /128 for IPv6).
//
// When RemoteAddr is trusted, the key is extracted from X-Forwarded-For
// by walking right-to-left, skipping any hops whose IPs are also in a
// trusted CIDR, and returning the first non-trusted address (RFC 7239
// rightmost-trusted-hop semantics). This prevents a client that connects
// directly to the server from spoofing another client's identity via
// XFF, while still respecting the header on legitimately-proxied
// requests.
//
// Without this option (default), X-Forwarded-For is IGNORED — every
// request keys on its RemoteAddr host, matching the safe default. This
// is a breaking behavior change from earlier versions that trusted XFF
// unconditionally.
func WithTrustedProxies(cidrs ...string) RateLimitOption {
	return func(c *rateLimitConfig) {
		for _, cidr := range cidrs {
			if _, n, err := net.ParseCIDR(cidr); err == nil {
				c.trustedProxies = append(c.trustedProxies, n)
				continue
			}
			// Bare IP → /32 or /128.
			if ip := net.ParseIP(cidr); ip != nil {
				suffix := "/32"
				if ip.To4() == nil {
					suffix = "/128"
				}
				if _, n, err := net.ParseCIDR(cidr + suffix); err == nil {
					c.trustedProxies = append(c.trustedProxies, n)
				}
			}
		}
	}
}

// RateLimitMiddleware rejects requests that exceed rate limits. The key
// used to identify a client defaults to the host portion of RemoteAddr
// (see net.SplitHostPort — dropping the ephemeral port so per-connection
// bucket bypass is impossible). Configure trusted upstream proxies via
// WithTrustedProxies to opt into X-Forwarded-For hop resolution.
func RateLimitMiddleware(limiter RateLimiter, opts ...RateLimitOption) Middleware {
	cfg := &rateLimitConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientKey(r, cfg)

			if !limiter.Allow(key) {
				errorenvelope.Write(w, http.StatusTooManyRequests, errorenvelope.Body{
					Code:      "TOO_MANY_REQUESTS",
					Message:   "Too Many Requests",
					RequestID: GetRequestID(r.Context()),
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// clientKey extracts the rate-limit key from r per the rules in cfg.
// Splits RemoteAddr into host:port and keys on the host (never the port,
// which is ephemeral). If RemoteAddr's host is in a trusted-proxy CIDR
// and X-Forwarded-For is present, walks the XFF list right-to-left,
// skipping any hops also in a trusted CIDR, and returns the first
// non-trusted address (RFC 7239). Otherwise ignores XFF entirely.
func clientKey(r *http.Request, cfg *rateLimitConfig) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if !isIPTrusted(host, cfg.trustedProxies) {
		return host
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}
	hops := strings.Split(xff, ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			continue
		}
		if !isIPTrusted(hop, cfg.trustedProxies) {
			return hop
		}
	}
	return host
}

// isIPTrusted reports whether ipStr belongs to any of the given CIDRs.
// Returns false when trustedProxies is empty or ipStr does not parse.
func isIPTrusted(ipStr string, trustedProxies []*net.IPNet) bool {
	if len(trustedProxies) == 0 {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, cidr := range trustedProxies {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// TokenBucketLimiter enforces rate limits using token bucket semantics.
//
// Per-key limiters (see NewTokenBucketLimiterPerKey) evict idle buckets in
// the background so the per-key map is bounded by the active-client set,
// not the process-lifetime spoofable-key set. Global limiters (see
// NewTokenBucketLimiter) hold no per-key state and do not evict.
type TokenBucketLimiter struct {
	rate       int
	capacity   int
	mu         sync.RWMutex
	perKey     bool
	buckets    map[string]*tokenBucket
	tokens     int
	lastRefill time.Time

	// Per-key eviction plumbing. bucketTTL is the idle duration after
	// which a bucket becomes eligible for sweeping; done signals the
	// sweeper goroutine to exit on Close. Only used when perKey=true.
	bucketTTL time.Duration
	done      chan struct{}
	closeOnce sync.Once
}

type tokenBucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// TokenBucketLimiterOption configures a per-key TokenBucketLimiter.
type TokenBucketLimiterOption func(*TokenBucketLimiter)

// WithBucketTTL sets the idle duration after which an unused per-key
// bucket is evicted from the limiter's internal map. Default 10 minutes.
// Applied only to per-key limiters; ignored on the global limiter.
func WithBucketTTL(d time.Duration) TokenBucketLimiterOption {
	return func(l *TokenBucketLimiter) {
		l.bucketTTL = d
	}
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

// NewTokenBucketLimiterPerKey creates a per-key token bucket limiter with
// TTL-based bucket eviction. A background goroutine sweeps idle buckets
// every bucketTTL/2. Callers should call Close when the limiter is no
// longer needed so the goroutine can exit; forgetting to Close leaks one
// goroutine per constructed limiter but does not affect correctness.
func NewTokenBucketLimiterPerKey(rate, capacity int, opts ...TokenBucketLimiterOption) *TokenBucketLimiter {
	l := &TokenBucketLimiter{
		rate:      rate,
		capacity:  capacity,
		perKey:    true,
		buckets:   make(map[string]*tokenBucket),
		bucketTTL: 10 * time.Minute,
		done:      make(chan struct{}),
	}
	for _, opt := range opts {
		opt(l)
	}
	go l.sweepIdleBuckets()
	return l
}

// Close stops the background sweeper goroutine on per-key limiters. Safe
// to call multiple times and safe to call on a global limiter (no-op).
func (l *TokenBucketLimiter) Close() error {
	if !l.perKey {
		return nil
	}
	l.closeOnce.Do(func() { close(l.done) })
	return nil
}

// sweepIdleBuckets runs in a background goroutine and periodically
// removes per-key buckets whose lastRefill is older than bucketTTL.
// Exits when Close is called (done is closed).
func (l *TokenBucketLimiter) sweepIdleBuckets() {
	interval := l.bucketTTL / 2
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-l.done:
			return
		case <-ticker.C:
			l.evictExpiredBuckets(time.Now())
		}
	}
}

// evictExpiredBuckets removes buckets idle beyond bucketTTL as of now.
// Splits into a snapshot phase (read lock, gather stale keys) and a
// delete phase (write lock, re-verify staleness under the write lock and
// remove) to avoid holding the outer write lock across N bucket mutex
// acquisitions and to be race-safe against concurrent Allow calls that
// might revive a bucket between the snapshot and the delete.
func (l *TokenBucketLimiter) evictExpiredBuckets(now time.Time) {
	l.mu.RLock()
	stale := make([]string, 0)
	for key, bucket := range l.buckets {
		bucket.mu.Lock()
		if now.Sub(bucket.lastRefill) > l.bucketTTL {
			stale = append(stale, key)
		}
		bucket.mu.Unlock()
	}
	l.mu.RUnlock()
	if len(stale) == 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range stale {
		bucket, ok := l.buckets[key]
		if !ok {
			continue
		}
		bucket.mu.Lock()
		expired := now.Sub(bucket.lastRefill) > l.bucketTTL
		bucket.mu.Unlock()
		if expired {
			delete(l.buckets, key)
		}
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
	// Refill in nanoseconds so fractional seconds are credited. The previous
	// implementation used int(elapsed.Seconds()) which truncated to 0 for any
	// call arriving <1s after the last, while lastRefill still advanced —
	// starving all traffic under sustained sub-second load.
	refill := int(int64(elapsed) * int64(l.rate) / int64(time.Second))

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
	// See allowGlobal for the rationale on nanosecond math.
	refill := int(int64(elapsed) * int64(l.rate) / int64(time.Second))

	bucket.tokens = min(bucket.tokens+refill, l.capacity)
	bucket.lastRefill = now

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}
	return false
}

// SlidingWindowLimiter enforces rate limits using a sliding window.
//
// Note: uses a single mutex and performs an O(n) prune per Allow, where n
// is the number of requests in the current window. Fine at low-to-moderate
// concurrency; for sub-millisecond p99 at high load, prefer
// TokenBucketLimiter or TokenBucketLimiterPerKey (both hold a per-key
// mutex only for the duration of the token math).
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
				errorenvelope.Write(w, http.StatusUnauthorized, errorenvelope.Body{
					Code:      "UNAUTHORIZED",
					Message:   "Unauthorized",
					RequestID: GetRequestID(r.Context()),
				})
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

// Flush, Hijack, Push, and Unwrap forward to the underlying ResponseWriter so
// that installing LoggingMiddleware does not break long-lived connections
// (SSE requires http.Flusher; WebSocket upgrade requires http.Hijacker;
// http.ResponseController walks the chain via Unwrap()). Without these,
// SSE routes return 500 "streaming not supported" and WS upgrades fail.
// Mirrors gzipResponseWriter above.
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := r.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("responseWriter does not implement http.Hijacker")
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := r.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return fmt.Errorf("responseWriter does not implement http.Pusher")
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
