package espresso

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
}

// ServerOption is a function that modifies ServerConfig.
type ServerOption func(*ServerConfig)

// Default server configuration with production-ready timeouts.
var defaultConfig = ServerConfig{
	Addr:              ":8080",
	ReadTimeout:       10 * time.Second,
	WriteTimeout:      10 * time.Second,
	IdleTimeout:       60 * time.Second,
	ReadHeaderTimeout: 5 * time.Second,
	ShutdownTimeout:   10 * time.Second,
}

// WithAddr sets the server address.
func WithAddr(addr string) ServerOption {
	return func(c *ServerConfig) { c.Addr = addr }
}

// WithReadTimeout sets the maximum duration for reading the entire request.
func WithReadTimeout(d time.Duration) ServerOption {
	return func(c *ServerConfig) { c.ReadTimeout = d }
}

// WithWriteTimeout sets the maximum duration before timing out writes.
func WithWriteTimeout(d time.Duration) ServerOption {
	return func(c *ServerConfig) { c.WriteTimeout = d }
}

// WithIdleTimeout sets the maximum amount of time to wait for the next request.
func WithIdleTimeout(d time.Duration) ServerOption {
	return func(c *ServerConfig) { c.IdleTimeout = d }
}

// WithReadHeaderTimeout sets the amount of time allowed to read request headers.
func WithReadHeaderTimeout(d time.Duration) ServerOption {
	return func(c *ServerConfig) { c.ReadHeaderTimeout = d }
}

// WithShutdownTimeout sets the maximum duration for graceful shutdown.
func WithShutdownTimeout(d time.Duration) ServerOption {
	return func(c *ServerConfig) { c.ShutdownTimeout = d }
}

// Brew starts the HTTP server with graceful shutdown support.
// It blocks until the server is stopped by signal (SIGINT, SIGTERM, SIGQUIT).
//
// The shutdown sequence is:
//  1. Registered OnShutdown hooks run in order
//  2. All open SSE streams close with a final comment
//  3. All open WebSockets close with code 1001 (going away)
//  4. The HTTP server stops accepting new connections
//  5. In-flight HTTP requests complete up to the shutdown timeout
//  6. Remaining connections are force-closed
//
// Options can be used to customize server configuration:
//
//	router.Brew(
//	    espresso.WithAddr(":3000"),
//	    espresso.WithReadTimeout(5*time.Second),
//	)
func (r *Router) Brew(opts ...ServerOption) {
	cfg := defaultConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}

	serverErr := make(chan error, 1)

	go func() {
		log.Info().Str("addr", cfg.Addr).Msg("🚀 Server running")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case err := <-serverErr:
		log.Fatal().Err(err).Msg("Server failed to start")
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("🛑 Shutting down server...")
	}

	r.gracefulShutdown(context.Background(), srv, cfg.ShutdownTimeout)
}

// BrewContext starts the HTTP server with graceful shutdown, using the provided
// context for cancellation. When the context is canceled, the server begins
// its graceful shutdown sequence.
//
// This is useful for programmatic control of the server lifecycle (e.g., in tests
// or when embedding Espresso in another application).
//
// Example:
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
//	defer cancel()
//	if err := router.BrewContext(ctx, espresso.WithAddr(":8080")); err != nil {
//	    slog.Error("server error", "error", err)
//	}
func (r *Router) BrewContext(ctx context.Context, opts ...ServerOption) error {
	cfg := defaultConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}

	serverErr := make(chan error, 1)

	go func() {
		log.Info().Str("addr", cfg.Addr).Msg("🚀 Server running")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info().Msg("🛑 Context canceled, shutting down server...")
	case err := <-serverErr:
		return err
	}

	r.gracefulShutdown(ctx, srv, cfg.ShutdownTimeout)
	return nil
}

// gracefulShutdown performs the full graceful shutdown sequence:
//  1. Run registered OnShutdown hooks in order
//  2. Close all SSE streams with a final comment
//  3. Close all WebSockets with close code 1001
//  4. Stop accepting new HTTP connections
//  5. Wait for in-flight requests to complete within timeout
func (r *Router) gracefulShutdown(parentCtx context.Context, srv *http.Server, timeout time.Duration) {
	log.Info().Msg("shutdown initiated")

	shutdownCtx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	// 1. Run user-registered hooks in order
	for i, hook := range r.shutdownHooks {
		runHookSafely(shutdownCtx, hook, i)
	}

	// 2. Close all SSE streams owned by this Router before shutting down.
	r.sseReg.closeAll("server shutting down")

	// 3. Close all WebSocket connections owned by this Router (close code 1001).
	r.wsReg.closeAll(CloseGoingAway, "server shutting down")

	// 4-5. Stop accepting new connections and wait for in-flight requests
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server shutdown error")
	}

	log.Info().Msg("✅ Server stopped")
}

// runHookSafely executes a shutdown hook with panic recovery and error logging.
// If the hook panics or returns an error, it is logged and shutdown continues.
func runHookSafely(ctx context.Context, hook ShutdownHook, index int) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Error().Int("hook", index).Interface("panic", rec).Msg("shutdown hook panicked")
		}
	}()

	if err := hook(ctx); err != nil {
		// Don't log context canceled errors since that's expected during shutdown
		if ctx.Err() != nil {
			log.Warn().Int("hook", index).Err(err).Msg("shutdown hook timed out")
		} else {
			log.Error().Int("hook", index).Err(err).Msg("shutdown hook failed")
		}
	}
}
