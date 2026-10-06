package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	eapi "github.com/openware-io/open-green-pass/internal/execution/api"
	eapp "github.com/openware-io/open-green-pass/internal/execution/application"
	edomain "github.com/openware-io/open-green-pass/internal/execution/domain"
	einfra "github.com/openware-io/open-green-pass/internal/execution/infra"
	gpPool "github.com/openware-io/open-green-pass/internal/execution/infra/pool"
	gpRunner "github.com/openware-io/open-green-pass/internal/execution/infra/runner"
	"github.com/openware-io/open-green-pass/internal/gateway"
	"github.com/openware-io/open-green-pass/internal/gateway/middleware"
	gapi "github.com/openware-io/open-green-pass/internal/governance/api"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/infra"
	iamapp "github.com/openware-io/open-green-pass/internal/iam/application"
	iaminfra "github.com/openware-io/open-green-pass/internal/iam/infra"
	"github.com/openware-io/open-green-pass/internal/platform/config"
	"github.com/openware-io/open-green-pass/internal/platform/observability"
	tapi "github.com/openware-io/open-green-pass/internal/trusted/api"
	tapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	tinfra "github.com/openware-io/open-green-pass/internal/trusted/infra"
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
	// GP3-05 reference policy is intentionally empty until the persistent
	// approval/whitelist store is configured; it can be injected here later.
	caseStore := infra.NewCaseStore(db, gen)
	caseSvc := application.NewCaseService(caseStore, gen)

	// 执行域依赖组装（版本校验 + 运行编排）
	envStore := einfra.NewEnvStore(db, gen)
	envSvc := eapp.NewEnvService(envStore, gen)
	runStore := einfra.NewRunStore(db, gen)
	caseReader := einfra.NewCaseReader(db)
	policyStore := einfra.NewPolicyStore(db, gen)
	policySvc := eapp.NewPolicyService(policyStore, gen)
	var runner edomain.RunnerPort
	switch os.Getenv("GP_RUNNER_TYPE") {
	case "scenario":
		runner = gpRunner.NewScenarioRunner(gen, gpRunner.DefaultRegistry(), nil)
		log.Info("runner", "type", "scenario")
	case "k8s":
		k8sRunner, err := einfra.NewK8sRunner(gen, einfra.K8sRunnerConfig{
			Namespace: os.Getenv("GP_RUNNER_NS"),
			Image:     os.Getenv("GP_RUNNER_IMAGE"),
			Timeout:   5 * time.Minute,
		})
		if err != nil {
			log.Error("init k8s runner", "error", err)
			os.Exit(1)
		}
		runner = k8sRunner
		log.Info("runner", "type", "k8s")
	default:
		runner = einfra.NewMockRunner(gen)
		log.Info("runner", "type", "mock")
	}
	runSvc := eapp.NewRunService(runStore, envSvc, caseReader, policySvc, runner, gen)
	// Reference-only in-process exclusion. Replace with shared Redis/DB storage
	// before running multiple server replicas.
	runSvc.SetConflictPort(gpPool.NewConflictRegistry())

	// 可信域依赖组装（审计哈希链 + 门禁判定 + 成本明细）
	gateStore := tinfra.NewGateStore(db, gen)
	auditStore := tinfra.NewAuditStore(db, gen)
	auditSvc := tapp.NewAuditService(auditStore, gen)
	runSvc.SetAuditPort(einfra.NewTrustedAuditPort(auditSvc))
	statReader := tinfra.NewRunStatReader(db)
	gateSvc := tapp.NewGateService(gateStore, auditSvc, statReader, gen)
	costStore := tinfra.NewCostStore(db, gen)
	costSvc := tapp.NewCostService(costStore, auditSvc, gen)
	reportStore := tinfra.NewReportStore(db, gen)
	reportSvc, err := tapp.NewReportService(statReader, costStore, gateStore, reportStore, gen)
	if err != nil {
		log.Error("init report service", "error", err)
		panic(err)
	}
	runSvc.SetGatePort(einfra.NewTrustedGatePort(gateSvc))
	runSvc.SetReportPort(einfra.NewTrustedReportPort(reportSvc))
	iamAuthorizer := iamapp.NewAuthorizer(iaminfra.NewRBACStore(db, gen))

	addr := cfg.Addr
	// Header identity is a development-only compatibility path. Production
	// must wire a verified AuthenticationProvider before serving business APIs.
	allowHeaderFallback := cfg.Env != "prod" && os.Getenv("GP_ALLOW_HEADER_AUTH") == "true"
	handler := gateway.NewRouterWithAuth(log, nil, allowHeaderFallback,
		func(mux *http.ServeMux) {
			gateway.RegisterSystem(mux, gateway.SystemStatus{Ready: func(ctx context.Context) error { return pool.Ping(ctx) }})
		},
		func(mux *http.ServeMux) { gapi.RegisterAuthorized(mux, treeSvc, iamAuthorizer) },
		func(mux *http.ServeMux) { gapi.RegisterCasesAuthorized(mux, caseSvc, iamAuthorizer) },
		func(mux *http.ServeMux) { eapi.RegisterAuthorized(mux, envSvc, runSvc, policySvc, iamAuthorizer) },
		func(mux *http.ServeMux) { tapi.Register(mux, gateSvc, auditSvc, costSvc, reportSvc) },
	)
	srv := &http.Server{
		Addr:              addr,
		Handler:           middleware.CORS(cfg.CORSAllowedOrigins)(handler),
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
