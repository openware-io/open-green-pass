// GreenPass Temporal Worker（GP2-02）：执行测试运行 Workflow 的 worker 实例。
// 连接 Temporal frontend，注册 ExecuteRunWorkflow 与其 Activities。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	redis "github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	aiapp "github.com/openware-io/open-green-pass/internal/ai/application"
	aiinfra "github.com/openware-io/open-green-pass/internal/ai/infra"
	aiaudit "github.com/openware-io/open-green-pass/internal/ai/trusted_audit"
	aiworkflow "github.com/openware-io/open-green-pass/internal/ai/workflow"
	cicdapp "github.com/openware-io/open-green-pass/internal/cicd/application"
	cicdinfra "github.com/openware-io/open-green-pass/internal/cicd/infra"
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
	"github.com/openware-io/open-green-pass/internal/execution/workflow"
	"github.com/openware-io/open-green-pass/internal/platform/config"
	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/platform/observability"
	tapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	tinfra "github.com/openware-io/open-green-pass/internal/trusted/infra"
	"github.com/openware-io/open-green-pass/pkg/id"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.Env)
	// Background workers record the same low-cardinality metrics contract as the
	// API process; plan GC counters are emitted from GenerationPlanStore.SweepOnce.
	metricsRecorder := observability.NewRecorder()
	startWorkerMetrics(metricsRecorder, log)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DBDSN)
	mustStart(err, log, "pgxpool init failed", "err")
	defer pool.Close()
	trustedDSN, err := cfg.TrustedDSN()
	mustStart(err, log, "trusted database configuration", "error")
	trustedPool, err := pgxpool.New(ctx, trustedDSN)
	mustStart(err, log, "trusted pgxpool init failed", "err")
	defer trustedPool.Close()

	gen, err := id.New(1, nil)
	mustStart(err, log, "id generator init failed", "err")
	dbb := db.NewDB(pool)
	trustedDB := db.NewDB(trustedPool)

	// 执行域依赖（与 cmd/server 一致）
	envStore := einfra.NewEnvStore(dbb, gen)
	envSvc := eapp.NewEnvService(envStore, gen)
	runStore := einfra.NewRunStore(dbb, gen)
	caseReader := einfra.NewCaseReader(dbb)
	policyStore := einfra.NewPolicyStore(dbb, gen)
	policySvc := eapp.NewPolicyService(policyStore, gen)
	runner := buildWorkerRunner(gen, log)
	runSvc := eapp.NewRunService(runStore, envSvc, caseReader, policySvc, runner, gen)
	enableExecutionPool(runSvc, pool, log)
	quotaClient, err := gpRedisQuota.NewClient(ctx, gpRedisQuota.ClientOptions{Address: cfg.RedisAddr, Password: os.Getenv("GP_REDIS_PASSWORD")})
	mustStart(err, log, "init redis quota client", "error")
	defer quotaClient.Close()
	runQuota, err := gpRedisQuota.New(quotaClient, map[string]gpQuota.Limits{"run": {
		Global: config.MustInt("GP_QUOTA_RUN_GLOBAL", 50), Team: config.MustInt("GP_QUOTA_RUN_TEAM", 20),
		Target: config.MustInt("GP_QUOTA_RUN_TARGET", 5), Owner: config.MustInt("GP_QUOTA_RUN_OWNER", 3),
	}})
	mustStart(err, log, "init redis quota", "error")
	runSvc.SetQuotaPort(runQuota)
	runEventBus, err := gpRedisRunBus.New(ctx, cfg.RedisAddr, os.Getenv("GP_REDIS_PASSWORD"))
	mustStart(err, log, "init redis run event bus", "error")
	defer runEventBus.Close()
	runSvc.SetRunEventBus(runEventBus)
	conflicts, err := pgconflict.New(pool)
	mustStart(err, log, "init postgres conflict registry", "error")
	runSvc.SetConflictPort(conflicts)
	auditSvc := tapp.NewAuditService(tinfra.NewAuditStore(trustedDB, gen), gen)
	runSvc.SetAuditPort(einfra.NewTrustedAuditPort(auditSvc))
	gateStore := tinfra.NewGateStore(trustedDB, gen)
	statReader := tinfra.NewRunStatReader(dbb)
	gateSvc := tapp.NewGateService(gateStore, auditSvc, statReader, gen)
	costStore := tinfra.NewCostStore(trustedDB, gen)
	costSvc := tapp.NewCostService(costStore, auditSvc, gen)
	costPriceStore := tinfra.NewCostPriceStore(dbb)
	costPriceSvc := tapp.NewCostPriceService(costPriceStore, auditSvc, gen)
	reportSvc, err := tapp.NewReportService(statReader, costStore, gateStore, tinfra.NewReportStore(dbb, gen), gen)
	mustStart(err, log, "init report service", "error")
	runSvc.SetGatePort(einfra.NewTrustedGatePort(gateSvc))
	runSvc.SetReportPort(einfra.NewTrustedReportPort(reportSvc))
	runSvc.SetCostPort(einfra.NewTrustedCostPort(costSvc))

	// Temporal client + worker
	addr := os.Getenv("GP_TEMPORAL_ADDR")
	if addr == "" {
		addr = "127.0.0.1:7233"
	}
	c, err := client.Dial(client.Options{HostPort: addr})
	mustStart(err, log, "temporal client dial failed", "err", "addr", addr)
	defer c.Close()
	generationStore := aiinfra.NewGenerationBatchStore(dbb, gen)
	generationStore.SetCostPort(aiinfra.NewGenerationCostAdapter(costSvc, costPriceSvc))
	if gatewayURL := os.Getenv("GP_DSH_GATEWAY_URL"); gatewayURL != "" {
		generationStore.SetDSHClient(aiinfra.NewDSHGatewayClient(gatewayURL, os.Getenv("GP_DSH_INTERNAL_TOKEN")))
	}
	go purgeGenerationArtifacts(ctx, dbb, log)

	generationSvc := aiapp.NewGenerationBatchService(generationStore)
	generationSvc.SetScopePort(generationStore)
	generationController := aiworkflow.NewTemporalController(c)
	generationSvc.SetReviewPorts(generationController, aiaudit.New(auditSvc))
	aiworkflow.SetGenerationDeps(generationSvc, generationStore, gen)

	// Run aggregation: the worker owns the 5s batch->unit/run reconciler so GET
	// stays a pure read. The reconciler scans non-terminal runs across tenants
	// through a bounded definer function and writes each projection inside its
	// own tenant transaction.
	generationRunStore := aiinfra.NewGenerationRunStore(dbb)
	generationRunStore.SetCostReadPort(aiinfra.NewGenerationCostReadAdapter(costSvc))
	generationRunEventStore := aiinfra.NewGenerationRunEventStore(dbb)
	generationPlanStore := aiinfra.NewGenerationPlanStore(dbb)
	generationPlanStore.SetMetricsRecorder(metricsRecorder)
	generationRunSvc := aiapp.NewGenerationRunService(generationRunStore, generationSvc, gen)
	generationRunSvc.SetPlanPort(generationPlanStore)
	generationRunSvc.SetEventPort(generationRunEventStore)
	generationReconciler := aiapp.NewGenerationRunReconciler(
		generationRunStore, generationRunSvc,
		time.Duration(config.MustInt("GP_GENERATION_RECONCILE_INTERVAL_SECONDS", 5))*time.Second,
		log,
	)
	temporalController := workflow.NewTemporalController(c)
	runSvc.SetWorkflowController(temporalController)

	w := worker.New(c, "gp-execution", worker.Options{})
	workflow.SetDeps(runSvc)
	w.RegisterWorkflow(workflow.ExecuteRunWorkflow)
	w.RegisterActivity(workflow.ActivityRunVersionCheck)
	w.RegisterActivity(workflow.ActivityExecuteRun)
	w.RegisterActivity(workflow.ActivityMarkRunFailed)

	gw := worker.New(c, "gp-generation", worker.Options{})
	gw.RegisterWorkflow(aiworkflow.GenerationPipelineWorkflow)
	gw.RegisterActivity(aiworkflow.FetchSource)
	gw.RegisterActivity(aiworkflow.ParseAndNormalize)
	gw.RegisterActivity(aiworkflow.GenerateSeeds)
	gw.RegisterActivity(aiworkflow.QualityGate)
	gw.RegisterActivity(aiworkflow.FailGeneration)
	gw.RegisterActivity(aiworkflow.RecordReview)
	gw.RegisterActivity(aiworkflow.CommitVersions)
	gw.RegisterActivity(aiworkflow.ApplyRollback)
	gw.RegisterActivity(aiworkflow.EmitAudit)

	mustStart(w.Start(), log, "temporal worker start failed", "err")
	defer w.Stop()
	mustStart(gw.Start(), log, "generation temporal worker start failed", "err")
	defer gw.Stop()

	scheduleClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: os.Getenv("GP_REDIS_PASSWORD")})
	defer scheduleClient.Close()
	scheduleQueue, err := gpRedisQueue.New(ctx, scheduleClient, "gp:execution:schedule", "gp-execution")
	mustStart(err, log, "init redis schedule queue", "error")
	_, consumer := resolveWorkerIdentity()
	dispatcher, err := eapp.NewScheduleDispatcher(scheduleQueue, temporalController, consumer, log)
	mustStart(err, log, "init schedule dispatcher", "error")
	workerCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go generationReconciler.Run(workerCtx)
	go generationPlanStore.RunGC(
		workerCtx,
		time.Duration(config.MustInt("GP_GENERATION_PLAN_GC_INTERVAL_SECONDS", 600))*time.Second,
		log,
	)
	go runScheduleDispatcher(workerCtx, dispatcher, log, stop)

	// CI outbox delivery is enabled only when an explicit webhook endpoint is
	// configured. Missing configuration is a real unavailable-provider state;
	// it is never treated as a successful delivery.
	startCICDOutboxDispatcher(workerCtx, dbb, log)

	log.Info("greenpass temporal worker started", "task_queue", "gp-execution", "temporal", addr)
	<-workerCtx.Done()
}

func purgeGenerationArtifacts(ctx context.Context, database *db.DB, log interface{ Info(string, ...any) }) {
	retentionHours := config.MustInt("GP_GENERATION_RETENTION_HOURS", 24)
	if retentionHours < 1 {
		retentionHours = 24
	}
	run := func() {
		cutoff := time.Now().UTC().Add(-time.Duration(retentionHours) * time.Hour)
		var removed int64
		if err := database.Pool().QueryRow(context.Background(), "SELECT gp.purge_generation_artifacts($1)", cutoff).Scan(&removed); err != nil {
			log.Info("generation artifact janitor failed", "error", err)
			return
		}
		if removed > 0 {
			log.Info("generation artifact janitor removed rows", "rows", removed, "cutoff", cutoff)
		}
	}
	run()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func startWorkerMetrics(recorder *observability.Recorder, log *slog.Logger) {
	metricsAddr := os.Getenv("GP_WORKER_METRICS_ADDR")
	if metricsAddr == "" {
		return
	}
	// Optional scrape endpoint for background-worker metrics (disabled unless
	// explicitly configured, so existing deployments are unchanged).
	go func() {
		server := &http.Server{Addr: metricsAddr, Handler: recorder.Handler(), ReadHeaderTimeout: 5 * time.Second}
		log.Info("worker metrics server listening", "addr", metricsAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("worker metrics server stopped", "error", err)
		}
	}()
}

func buildWorkerRunner(gen *id.Generator, log *slog.Logger) edomain.RunnerPort {
	switch os.Getenv("GP_RUNNER_TYPE") {
	case "scenario":
		return gpRunner.NewScenarioRunner(gen, gpRunner.DefaultRegistry(), nil)
	case "http":
		return einfra.NewHTTPRunner(gen, os.Getenv("GP_EXECUTION_BEARER_TOKEN"))
	case "hybrid":
		return einfra.NewHybridRunner(gen, os.Getenv("GP_EXECUTION_BEARER_TOKEN"), os.Getenv("GP_BROWSER_RUNNER_URL"))
	case "k8s":
		kr, err := einfra.NewK8sRunner(gen, einfra.K8sRunnerConfig{Namespace: os.Getenv("GP_RUNNER_NS"), Image: os.Getenv("GP_RUNNER_IMAGE"), Timeout: 5 * time.Minute})
		mustStart(err, log, "init k8s runner", "error")
		return kr
	default:
		return einfra.NewMockRunner(gen)
	}
}

func enableExecutionPool(runSvc *eapp.RunService, pool *pgxpool.Pool, log *slog.Logger) {
	if os.Getenv("GP_EXECUTION_POOL_ENABLED") != "true" {
		return
	}
	registry, err := pgpool.New(pool)
	mustStart(err, log, "init execution pool registry", "error")
	selector, err := pgpool.NewSelector(registry, pgpool.SelectorConfig{
		HeartbeatTTL: time.Duration(config.MustInt("GP_EXECUTION_POOL_HEARTBEAT_TTL_SECONDS", 30)) * time.Second,
		LeaseTTL:     time.Duration(config.MustInt("GP_EXECUTION_POOL_LEASE_TTL_SECONDS", 600)) * time.Second,
	})
	mustStart(err, log, "init execution pool selector", "error")
	runSvc.SetExecutionPool(selector)
}

func resolveWorkerIdentity() (workerName, consumer string) {
	workerName = os.Getenv("GP_WORKER_NAME")
	if workerName == "" {
		workerName = os.Getenv("HOSTNAME")
	}
	if workerName == "" {
		workerName = "worker"
	}
	return workerName, fmt.Sprintf("%s-%d", workerName, os.Getpid())
}

func runScheduleDispatcher(ctx context.Context, dispatcher *eapp.ScheduleDispatcher, log *slog.Logger, stop func()) {
	if err := dispatcher.Run(ctx); err != nil {
		log.Error("schedule dispatcher stopped", "error", err)
		stop()
	}
}

func startCICDOutboxDispatcher(ctx context.Context, database *db.DB, log *slog.Logger) {
	cicdStore := cicdinfra.NewOutboxStore(database)
	connectors := map[string]cicdapp.Connector{}
	if webhookURL := os.Getenv("GP_CICD_WEBHOOK_URL"); webhookURL != "" {
		connectors[os.Getenv("GP_CICD_PROVIDER")] = cicdinfra.WebhookConnector{URL: webhookURL}
	}
	dispatcher := &cicdapp.Dispatcher{
		Outbox: cicdStore, Connectors: connectors,
		Retry:        cicdapp.RetryPolicy{MaxAttempts: config.MustInt("GP_CICD_MAX_ATTEMPTS", 5), BaseDelay: time.Duration(config.MustInt("GP_CICD_RETRY_BASE_SECONDS", 5)) * time.Second},
		ClaimTimeout: time.Duration(config.MustInt("GP_CICD_CLAIM_TIMEOUT_SECONDS", 120)) * time.Second,
	}
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := dispatcher.DispatchOne(ctx); err != nil {
					log.Error("cicd outbox dispatch failed", "error", err)
				}
			}
		}
	}()
}

// mustStart keeps startup failure handling in one place: log the real cause and
// exit before serving. It preserves the original log message/key fields.
func mustStart(err error, log *slog.Logger, message, key string, attrs ...any) {
	if err == nil {
		return
	}
	args := append(append([]any{}, attrs...), key, err)
	log.Error(message, args...)
	os.Exit(1)
}
