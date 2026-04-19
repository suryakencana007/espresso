package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/suryakencana007/espresso"
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
	state := AppState{Logger: slog.Default()}

	router := espresso.Portafilter().
		WithState(state).
		Get("/stream", espresso.StreamSimple(timeStream,
			espresso.WithKeepAlive(30*time.Second),
			espresso.WithRetryHint(5*time.Second),
		))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		router.Brew(espresso.WithAddr(":8080"))
	}()

	slog.Info("listening on :8080")
	<-ctx.Done()
	slog.Info("shutting down")
}
