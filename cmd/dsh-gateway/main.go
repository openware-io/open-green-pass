// Command dsh-gateway is the internal DSH SDK gateway skeleton. It owns a
// bounded queue and worker pool, but no GP business data and no database.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/openware-io/open-green-pass/internal/dshgw"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	addr := env("GP_DSH_GATEWAY_ADDR", ":8090")
	token := os.Getenv("GP_DSH_INTERNAL_TOKEN")
	if token == "" {
		logger.Error("GP_DSH_INTERNAL_TOKEN is required")
		os.Exit(1)
	}
	workers := atoi(env("GP_DSH_WORKERS", "2"), 2)
	queue := atoi(env("GP_DSH_QUEUE", "64"), 64)
	manager := dshgw.NewManager(dshgw.StubRunner{}, workers, queue)
	defer manager.Close()

	server := &http.Server{
		Addr:              addr,
		Handler:           dshgw.NewHandler(manager, token),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("dsh gateway starting", "addr", addr, "workers", workers)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("dsh gateway stopped", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func atoi(raw string, fallback int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
