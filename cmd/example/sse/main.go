package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/suryakencana007/espresso/v2"
)

type AppState struct {
	Logger *slog.Logger
}

func timeStream(ctx context.Context, stream *espresso.SSEStream) error {
	state := espresso.MustGetState[AppState](ctx)
	state.Logger.Info("client connected to time stream")
	defer state.Logger.Info("client disconnected from time stream")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := stream.SendText("time", time.Now().Format(time.RFC3339)); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func main() {
	if err := run(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	state := AppState{Logger: slog.Default()}

	router := espresso.Portafilter().
		WithState(state).
		Get("/stream", espresso.StreamSimple(timeStream,
			espresso.WithKeepAlive(30*time.Second),
			espresso.WithRetryHint(5*time.Second),
		))

	// BrewContext runs the server and blocks until ctx cancels; on cancel
	// it invokes the framework's graceful shutdown sequence (OnShutdown
	// hooks → SSE close → WS close → http.Server.Shutdown). Running Brew in
	// a goroutine and blocking main on a separate signal.NotifyContext for
	// the same signals races the shutdown to the process exit — see v2.4
	// task-04a (PR #62) for the audit repro of that antipattern.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("listening on :8080")
	if err := router.BrewContext(ctx, espresso.WithAddr(":8080")); err != nil {
		return err
	}
	slog.Info("shutdown complete")
	return nil
}
