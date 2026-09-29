//go:build integration

// GP2-02 集成测试：执行链纳入 Temporal Workflow（版本校验→执行→done）。
// 前提：Temporal frontend 已透传 127.0.0.1:7233；测试内自起 worker。
package workflow_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	eapp "github.com/openware-io/open-green-pass/internal/execution/application"
	einfra "github.com/openware-io/open-green-pass/internal/execution/infra"
	"github.com/openware-io/open-green-pass/internal/execution/workflow"
	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

const testDSN = "postgres://gp:gp@127.0.0.1:5433/gp?sslmode=disable"

// TestWorkflowExecuteRun Temporal 编排闭环：创建 run→StartWorkflow→done。
func TestWorkflowExecuteRun(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), testDSN)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)

	// 清理 team100 历史 run（避免唯一约束冲突）
	ctx := rls.WithTenant(context.Background(), 100)
	tx, _ := pool.Begin(ctx)
	_, _ = tx.Exec(ctx, `SELECT set_config('gp.team_id','100',false)`)
	_, _ = tx.Exec(ctx, `DELETE FROM run_case_result WHERE team_id=100`)
	_, _ = tx.Exec(ctx, `DELETE FROM run_run WHERE team_id=100`)
	_, _ = tx.Exec(ctx, `DELETE FROM env_check WHERE team_id=100`)
	_, _ = tx.Exec(ctx, `DELETE FROM env_runtime WHERE team_id=100`)
	_ = tx.Commit(ctx)

	gen, _ := id.New(1, nil)
	dbb := db.NewDB(pool)
	envStore := einfra.NewEnvStore(dbb, gen)
	envSvc := eapp.NewEnvService(envStore, gen)
	runStore := einfra.NewRunStore(dbb, gen)
	caseReader := einfra.NewCaseReader(dbb)
	policyStore := einfra.NewPolicyStore(dbb, gen)
	policySvc := eapp.NewPolicyService(policyStore, gen)
	runner := einfra.NewMockRunner(gen)
	runSvc := eapp.NewRunService(runStore, envSvc, caseReader, policySvc, runner, gen)

	// 创建运行（target 1003 im-saas-gateway，用例 4001）
	ctx2 := rls.WithTenant(context.Background(), 100)
	run, err := runSvc.CreateRun(ctx2, eapp.CreateRunRequest{
		ScenarioID: 1, TargetID: 1003, Env: "test", TargetVersion: "v1.0.0",
		TargetBranch: "main", RunMode: "manual",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := run.ID

	// Temporal client + worker
	c, err := client.Dial(client.Options{HostPort: "127.0.0.1:7233"})
	if err != nil {
		t.Fatalf("temporal dial: %v", err)
	}
	t.Cleanup(c.Close)
	workflow.SetDeps(runSvc)
	w := worker.New(c, "gp-execution", worker.Options{})
	w.RegisterWorkflow(workflow.ExecuteRunWorkflow)
	w.RegisterActivity(workflow.ActivityRunVersionCheck)
	w.RegisterActivity(workflow.ActivityExecuteRun)
	if err := w.Start(); err != nil {
		t.Fatalf("worker start: %v", err)
	}
	t.Cleanup(w.Stop)
	slog.SetLogLoggerLevel(slog.LevelWarn)

	// 启动 workflow
	we, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		TaskQueue: "gp-execution",
		ID:        "gp-run-" + itoa(runID),
	}, workflow.ExecuteRunWorkflow, workflow.ExecuteRunInput{RunID: runID, TeamID: 100})
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	var res workflow.ExecuteRunResult
	if err := we.Get(context.Background(), &res); err != nil {
		t.Fatalf("workflow result: %v", err)
	}
	if res.State != "done" {
		t.Fatalf("workflow state want done got %s", res.State)
	}
	if res.PassCount != 1 || res.TotalCount != 1 {
		t.Fatalf("pass/total want 1/1 got %d/%d", res.PassCount, res.TotalCount)
	}

	// 断言 DB 终态
	gotten, err := runSvc.GetRun(rls.WithTenant(context.Background(), 100), runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if gotten.State != "done" {
		t.Fatalf("db run state want done got %s", gotten.State)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
