package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/gateway"
	gapi "github.com/openware-io/open-green-pass/internal/governance/api"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/infra"
	eapi "github.com/openware-io/open-green-pass/internal/execution/api"
	eapp "github.com/openware-io/open-green-pass/internal/execution/application"
	einfra "github.com/openware-io/open-green-pass/internal/execution/infra"
	"github.com/openware-io/open-green-pass/internal/platform/config"
	"github.com/openware-io/open-green-pass/internal/platform/observability"
	"github.com/openware-io/open-green-pass/pkg/id"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.Env)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		log.Error("pgxpool init failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 治理域依赖组装（被测对象树 + 用例版本化）
	gen, err := id.New(1, nil)
	if err != nil {
		log.Error("id generator init failed", "err", err)
		os.Exit(1)
	}
	db := infra.NewDB(pool)
	store := infra.NewTargetStore(db, gen)
	treeSvc := application.NewTargetTreeService(store, gen)
	caseStore := infra.NewCaseStore(db, gen)
	caseSvc := application.NewCaseService(caseStore, gen)

	// 执行域依赖组装（测试环境版本校验）
	envStore := einfra.NewEnvStore(db, gen)
	envSvc := eapp.NewEnvService(envStore, gen)

	addr := cfg.Addr
	srv := &http.Server{
		Addr: addr,
		Handler: gateway.NewRouter(log,
			func(mux *http.ServeMux) { gapi.Register(mux, treeSvc) },
			func(mux *http.ServeMux) { gapi.RegisterCases(mux, caseSvc) },
			func(mux *http.ServeMux) { eapi.Register(mux, envSvc) },
		),
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
