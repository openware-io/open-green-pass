// GreenPass Temporal Worker（GP2-02）：执行测试运行 Workflow 的 worker 实例。
// 连接 Temporal frontend，注册 ExecuteRunWorkflow 与其 Activities。
package main

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	eapp "github.com/openware-io/open-green-pass/internal/execution/application"
	edomain "github.com/openware-io/open-green-pass/internal/execution/domain"
	einfra "github.com/openware-io/open-green-pass/internal/execution/infra"
	gpPool "github.com/openware-io/open-green-pass/internal/execution/infra/pool"
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

	gen, err := id.New(1, nil)
	if err != nil {
		log.Error("id generator init failed", "err", err)
		os.Exit(1)
	}
	dbb := db.NewDB(pool)

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
	// Reference-only in-process exclusion. Replace with shared Redis/DB storage
	// before running multiple worker replicas.
	runSvc.SetConflictPort(gpPool.NewConflictRegistry())
	auditSvc := tapp.NewAuditService(tinfra.NewAuditStore(dbb, gen), gen)
	runSvc.SetAuditPort(einfra.NewTrustedAuditPort(auditSvc))
	gateStore := tinfra.NewGateStore(dbb, gen)
	statReader := tinfra.NewRunStatReader(dbb)
	gateSvc := tapp.NewGateService(gateStore, auditSvc, statReader, gen)
	costStore := tinfra.NewCostStore(dbb, gen)
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
	runSvc.SetWorkflowController(workflow.NewTemporalController(c))

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

	log.Info("greenpass temporal worker started", "task_queue", "gp-execution", "temporal", addr)
	select {}
}
