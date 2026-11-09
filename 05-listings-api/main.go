package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

// run поднимает сервер и глушит его по Ctrl+C (graceful shutdown).
// Таймауты обязательны: без них медленный клиент держал бы соединение вечно.
func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(NewMemStore(), logger)

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      srv.Handler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()

	logger.Info("listening", "addr", httpServer.Addr)
	select {
	case err := <-errCh: // сервер упал сам
		return err
	case <-ctx.Done(): // пришёл Ctrl+C — даём запросам доиграть, но не дольше 10с
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutCtx)
	}
}
