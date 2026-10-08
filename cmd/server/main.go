package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	redis "github.com/redis/go-redis/v9"
	temporalclient "go.temporal.io/sdk/client"

	eapi "github.com/openware-io/open-green-pass/internal/execution/api"
	eapp "github.com/openware-io/open-green-pass/internal/execution/application"
	edomain "github.com/openware-io/open-green-pass/internal/execution/domain"
	einfra "github.com/openware-io/open-green-pass/internal/execution/infra"
	"github.com/openware-io/open-green-pass/internal/execution/infra/pgconflict"
	"github.com/openware-io/open-green-pass/internal/execution/infra/pgpool"
	gpQuota "github.com/openware-io/open-green-pass/internal/execution/infra/quota"
	gpRedisQueue "github.com/openware-io/open-green-pass/internal/execution/infra/redisqueue"
	gpRedisQuota "github.com/openware-io/open-green-pass/internal/execution/infra/redisquota"
	gpRedisRunBus "github.com/openware-io/open-green-pass/internal/execution/infra/redisrunbus"
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
	metricsRecorder := observability.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		log.Error("pgxpool init failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	trustedDSN, err := cfg.TrustedDSN()
	if err != nil {
		log.Error("trusted database configuration", "error", err)
		os.Exit(1)
	}
	trustedPool, err := pgxpool.New(ctx, trustedDSN)
	if err != nil {
		log.Error("trusted pgxpool init failed", "err", err)
		os.Exit(1)
	}
	defer trustedPool.Close()

	// 治理域依赖组装（被测对象树 + 用例版本化）
	gen, err := id.New(1, nil)
	if err != nil {
		log.Error("id generator init failed", "err", err)
		os.Exit(1)
	}
	db := infra.NewDB(pool)
	trustedDB := infra.NewDB(trustedPool)
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
	if os.Getenv("GP_EXECUTION_POOL_ENABLED") == "true" {
		registry, poolErr := pgpool.New(pool)
		if poolErr != nil {
			log.Error("init execution pool registry", "error", poolErr)
			os.Exit(1)
		}
		selector, poolErr := pgpool.NewSelector(registry, pgpool.SelectorConfig{HeartbeatTTL: time.Duration(config.MustInt("GP_EXECUTION_POOL_HEARTBEAT_TTL_SECONDS", 30)) * time.Second, LeaseTTL: time.Duration(config.MustInt("GP_EXECUTION_POOL_LEASE_TTL_SECONDS", 600)) * time.Second})
		if poolErr != nil {
			log.Error("init execution pool selector", "error", poolErr)
			os.Exit(1)
		}
		runSvc.SetExecutionPool(selector)
	}
	quotaClient, err := gpRedisQuota.NewClient(ctx, gpRedisQuota.ClientOptions{Address: cfg.RedisAddr, Password: os.Getenv("GP_REDIS_PASSWORD")})
	if err != nil {
		log.Error("init redis quota client", "error", err)
		os.Exit(1)
	}
	defer quotaClient.Close()
	runQuota, err := gpRedisQuota.New(quotaClient, map[string]gpQuota.Limits{"run": {
		Global: config.MustInt("GP_QUOTA_RUN_GLOBAL", 50), Team: config.MustInt("GP_QUOTA_RUN_TEAM", 20),
		Target: config.MustInt("GP_QUOTA_RUN_TARGET", 5), Owner: config.MustInt("GP_QUOTA_RUN_OWNER", 3),
	}})
	if err != nil {
		log.Error("init redis quota", "error", err)
		os.Exit(1)
	}
	runSvc.SetQuotaPort(runQuota)
	scheduleClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: os.Getenv("GP_REDIS_PASSWORD")})
	defer scheduleClient.Close()
	scheduleQueue, err := gpRedisQueue.New(ctx, scheduleClient, "gp:execution:schedule", "gp-execution")
	if err != nil {
		log.Error("init redis schedule queue", "error", err)
		os.Exit(1)
	}
	runSvc.SetScheduleQueue(scheduleQueue)
	temporalCtx, temporalCancel := context.WithTimeout(context.Background(), 10*time.Second)
	temporalClient, err := temporalclient.DialContext(temporalCtx, temporalclient.Options{HostPort: cfg.TemporalAddr})
	temporalCancel()
	if err != nil {
		log.Error("init temporal client", "error", err)
		os.Exit(1)
	}
	defer temporalClient.Close()
	runEventBus, err := gpRedisRunBus.New(ctx, cfg.RedisAddr, os.Getenv("GP_REDIS_PASSWORD"))
	if err != nil {
		log.Error("init redis run event bus", "error", err)
		os.Exit(1)
	}
	defer runEventBus.Close()
	runSvc.SetRunEventBus(runEventBus)
	conflicts, err := pgconflict.New(pool)
	if err != nil {
		log.Error("init postgres conflict registry", "error", err)
		os.Exit(1)
	}
	runSvc.SetConflictPort(conflicts)

	// 可信域依赖组装（审计哈希链 + 门禁判定 + 成本明细）
	gateStore := tinfra.NewGateStore(trustedDB, gen)
	auditStore := tinfra.NewAuditStore(trustedDB, gen)
	auditSvc := tapp.NewAuditService(auditStore, gen)
	runSvc.SetAuditPort(einfra.NewTrustedAuditPort(auditSvc))
	// Execution/run and report tables remain owned by their respective domains;
	// the trusted connection is restricted to trusted writes only.
	statReader := tinfra.NewRunStatReader(db)
	gateSvc := tapp.NewGateService(gateStore, auditSvc, statReader, gen)
	costStore := tinfra.NewCostStore(trustedDB, gen)
	costSvc := tapp.NewCostService(costStore, auditSvc, gen)
	reportStore := tinfra.NewReportStore(db, gen)
	reportSvc, err := tapp.NewReportService(statReader, costStore, gateStore, reportStore, gen)
	if err != nil {
		log.Error("init report service", "error", err)
		panic(err)
	}
	runSvc.SetGatePort(einfra.NewTrustedGatePort(gateSvc))
	runSvc.SetReportPort(einfra.NewTrustedReportPort(reportSvc))
	runSvc.SetCostPort(einfra.NewTrustedCostPort(costSvc))
	iamAuthorizer := iamapp.NewAuthorizer(iaminfra.NewRBACStore(db, gen))
	iamAuthorizer.SetAuditPort(iaminfra.NewTrustedAuditPort(auditSvc))

	addr := cfg.Addr
	// Header identity is a development-only compatibility path. Production
	// must wire a verified AuthenticationProvider before serving business APIs.
	allowHeaderFallback := cfg.Env != "prod" && os.Getenv("GP_ALLOW_HEADER_AUTH") == "true"
	handler := gateway.NewRouterWithAuth(log, nil, allowHeaderFallback,
		func(mux *http.ServeMux) {
			gateway.RegisterSystem(mux, gateway.SystemStatus{
				Ready: func(ctx context.Context) error { return pool.Ping(ctx) },
				Checks: map[string]gateway.SystemChecker{
					"redis":    func(ctx context.Context) error { return scheduleClient.Ping(ctx).Err() },
					"temporal": func(ctx context.Context) error { _, err := temporalClient.CheckHealth(ctx, nil); return err },
				},
				Metrics: metricsRecorder,
			})
		},
		func(mux *http.ServeMux) { gapi.RegisterAuthorized(mux, treeSvc, iamAuthorizer) },
		func(mux *http.ServeMux) { gapi.RegisterCasesAuthorized(mux, caseSvc, iamAuthorizer) },
		func(mux *http.ServeMux) { eapi.RegisterAuthorized(mux, envSvc, runSvc, policySvc, iamAuthorizer) },
		func(mux *http.ServeMux) { tapi.Register(mux, gateSvc, auditSvc, costSvc, reportSvc) },
	)
	srv := &http.Server{
		Addr:              addr,
		Handler:           middleware.CORS(cfg.CORSAllowedOrigins)(middleware.Metrics(metricsRecorder)(handler)),
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
