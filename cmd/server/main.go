package main

import (
	"net/http"
	"time"

	"github.com/openware-io/open-green-pass/internal/gateway"
	"github.com/openware-io/open-green-pass/internal/platform/config"
	"github.com/openware-io/open-green-pass/internal/platform/observability"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.Env)
	addr := cfg.Addr
	srv := &http.Server{
		Addr:              addr,
		Handler:           gateway.NewRouter(log),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("greenpass server starting", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("server exited", "err", err)
	}
}
