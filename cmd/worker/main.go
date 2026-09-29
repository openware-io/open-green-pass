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
	einfra "github.com/openware-io/open-green-pass/internal/execution/infra"
	"github.com/openware-io/open-green-pass/internal/execution/workflow"
	"github.com/openware-io/open-green-pass/internal/platform/config"
	"github.com/openware-io/open-green-pass/internal/platform/db"
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
	runner := einfra.NewMockRunner(gen)
	runSvc := eapp.NewRunService(runStore, envSvc, caseReader, policySvc, runner, gen)

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
