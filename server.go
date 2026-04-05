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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}
	log.Info().Msg("✅ Server stopped")
}
