// GreenPass Temporal Worker（GP2-02）：执行测试运行 Workflow 的 worker 实例。
// 连接 Temporal frontend，注册 ExecuteRunWorkflow 与其 Activities。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	redis "github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

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

	gen, err := id.New(1, nil)
	if err != nil {
		log.Error("id generator init failed", "err", err)
		os.Exit(1)
	}
	dbb := db.NewDB(pool)
	trustedDB := db.NewDB(trustedPool)

	// 执行域依赖（与 cmd/server 一致）
	envStore := einfra.NewEnvStore(dbb, gen)
	envSvc := eapp.NewEnvService(envStore, gen)
	runStore := einfra.NewRunStore(dbb, gen)
	caseReader := einfra.NewCaseReader(dbb)
	policyStore := einfra.NewPolicyStore(dbb, gen)
	policySvc := eapp.NewPolicyService(policyStore, gen)
	var runner edomain.RunnerPort
	switch os.Getenv("GP_RUNNER_TYPE") {
	case "scenario":
		runner = gpRunner.NewScenarioRunner(gen, gpRunner.DefaultRegistry(), nil)
	case "k8s":
		kr, e := einfra.NewK8sRunner(gen, einfra.K8sRunnerConfig{Namespace: os.Getenv("GP_RUNNER_NS"), Image: os.Getenv("GP_RUNNER_IMAGE"), Timeout: 5 * time.Minute})
		if e != nil {
			log.Error("init k8s runner", "error", e)
			os.Exit(1)
		}
		runner = kr
	default:
		runner = einfra.NewMockRunner(gen)
	}
	runSvc := eapp.NewRunService(runStore, envSvc, caseReader, policySvc, runner, gen)
	if os.Getenv("GP_EXECUTION_POOL_ENABLED") == "true" {
		registry, e := pgpool.New(pool)
		if e != nil {
			log.Error("init execution pool registry", "error", e)
			os.Exit(1)
		}
		selector, e := pgpool.NewSelector(registry, pgpool.SelectorConfig{HeartbeatTTL: time.Duration(config.MustInt("GP_EXECUTION_POOL_HEARTBEAT_TTL_SECONDS", 30)) * time.Second, LeaseTTL: time.Duration(config.MustInt("GP_EXECUTION_POOL_LEASE_TTL_SECONDS", 600)) * time.Second})
		if e != nil {
			log.Error("init execution pool selector", "error", e)
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
	auditSvc := tapp.NewAuditService(tinfra.NewAuditStore(trustedDB, gen), gen)
	runSvc.SetAuditPort(einfra.NewTrustedAuditPort(auditSvc))
	gateStore := tinfra.NewGateStore(trustedDB, gen)
	statReader := tinfra.NewRunStatReader(dbb)
	gateSvc := tapp.NewGateService(gateStore, auditSvc, statReader, gen)
	costStore := tinfra.NewCostStore(trustedDB, gen)
	costSvc := tapp.NewCostService(costStore, auditSvc, gen)
	reportSvc, err := tapp.NewReportService(statReader, costStore, gateStore, tinfra.NewReportStore(dbb, gen), gen)
	if err != nil {
		log.Error("init report service", "error", err)
		os.Exit(1)
	}
	runSvc.SetGatePort(einfra.NewTrustedGatePort(gateSvc))
	runSvc.SetReportPort(einfra.NewTrustedReportPort(reportSvc))
	runSvc.SetCostPort(einfra.NewTrustedCostPort(costSvc))

	// Temporal client + worker
	addr := os.Getenv("GP_TEMPORAL_ADDR")
	if addr == "" {
		addr = "127.0.0.1:7233"
	}
	c, err := client.Dial(client.Options{HostPort: addr})
	if err != nil {
		log.Error("temporal client dial failed", "addr", addr, "err", err)
		os.Exit(1)
	}
	defer c.Close()
	temporalController := workflow.NewTemporalController(c)
	runSvc.SetWorkflowController(temporalController)

	w := worker.New(c, "gp-execution", worker.Options{})
	workflow.SetDeps(runSvc)
	w.RegisterWorkflow(workflow.ExecuteRunWorkflow)
	w.RegisterActivity(workflow.ActivityRunVersionCheck)
	w.RegisterActivity(workflow.ActivityExecuteRun)

	if err := w.Start(); err != nil {
		log.Error("temporal worker start failed", "err", err)
		os.Exit(1)
	}
	defer w.Stop()

	scheduleClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: os.Getenv("GP_REDIS_PASSWORD")})
	defer scheduleClient.Close()
	scheduleQueue, err := gpRedisQueue.New(ctx, scheduleClient, "gp:execution:schedule", "gp-execution")
	if err != nil {
		log.Error("init redis schedule queue", "error", err)
		os.Exit(1)
	}
	workerName := os.Getenv("GP_WORKER_NAME")
	if workerName == "" {
		workerName = os.Getenv("HOSTNAME")
	}
	if workerName == "" {
		workerName = "worker"
	}
	consumer := fmt.Sprintf("%s-%d", workerName, os.Getpid())
	dispatcher, err := eapp.NewScheduleDispatcher(scheduleQueue, temporalController, consumer, log)
	if err != nil {
		log.Error("init schedule dispatcher", "error", err)
		os.Exit(1)
	}
	workerCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		if err := dispatcher.Run(workerCtx); err != nil {
			log.Error("schedule dispatcher stopped", "error", err)
			stop()
		}
	}()

	log.Info("greenpass temporal worker started", "task_queue", "gp-execution", "temporal", addr)
	<-workerCtx.Done()
}
