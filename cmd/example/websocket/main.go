package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"
)

type AppState struct {
	Logger *slog.Logger
}

type EchoReq struct {
	Room string `path:"room"`
}

func echoHandler(ctx context.Context, req *extractor.Path[EchoReq], ws *espresso.WS) error {
	state := espresso.MustGetState[AppState](ctx)
	state.Logger.Info("client connected", "room", req.Data.Room)
	defer state.Logger.Info("client disconnected", "room", req.Data.Room)

	for {
		msgType, data, err := ws.Read(ctx)
		if err != nil {
			return nil
		}

		response := []byte("[" + req.Data.Room + "] " + string(data))
		if err := ws.Write(ctx, msgType, response); err != nil {
			return err
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
		Get("/ws/{room}", espresso.WebSocket(echoHandler,
			espresso.WithPingInterval(30*time.Second),
		))

	// BrewContext runs the server and blocks until ctx cancels; on cancel
	// it invokes the framework's graceful shutdown sequence (OnShutdown
	// hooks → SSE close → WS close (1001) → http.Server.Shutdown). Running
	// Brew in a goroutine and blocking main on a separate signal.NotifyContext
	// for the same signals races the shutdown to the process exit — see
	// v2.4 task-04a (PR #62) for the audit repro of that antipattern.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("listening on :8080")
	if err := router.BrewContext(ctx, espresso.WithAddr(":8080")); err != nil {
		return err
	}
	slog.Info("shutdown complete")
	return nil
}
