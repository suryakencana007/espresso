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
	state := AppState{Logger: slog.Default()}

	router := espresso.Portafilter().
		WithState(state).
		Get("/ws/{room}", espresso.WebSocket(echoHandler,
			espresso.WithPingInterval(30*time.Second),
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
