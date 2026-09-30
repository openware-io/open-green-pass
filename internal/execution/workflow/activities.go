// Activities：执行 Workflow 的步骤实现（包级命名函数，经 SetDeps 注入依赖，供 Temporal 确定性反射）。
package workflow

import (
	"context"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"

	"github.com/openware-io/open-green-pass/internal/execution/application"
	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

// deps 执行链依赖（由 worker 经 SetDeps 注入；包级，保证 Workflow 反射到稳定函数名）。
var deps *Activities

// SetDeps 注入执行链依赖（worker 启动时调用一次）。
func SetDeps(runSvc *application.RunService) { deps = &Activities{runSvc: runSvc} }

// TemporalController signals the long-lived run workflow by its stable ID.
type TemporalController struct{ client client.Client }

func NewTemporalController(c client.Client) *TemporalController {
	return &TemporalController{client: c}
}

func (c *TemporalController) Signal(ctx context.Context, runID, _ int64, signal string) error {
	return c.client.SignalWorkflow(ctx, workflowID(runID), "", signal, struct{}{})
}

func workflowID(runID int64) string {
	return "gp-run-" + itoaWorkflow(runID)
}

func itoaWorkflow(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// Activities 持有执行链依赖。
type Activities struct {
	runSvc *application.RunService
}

// VersionCheckResult 版本校验 Activity 结果。
type VersionCheckResult struct{ Ok bool }

// ExecuteResult 执行 Activity 结果。
type ExecuteResult struct {
	State      domain.RunState
	PassCount  int
	TotalCount int
}

// ActivityRunVersionCheck 版本校验（team 注入后调用 RunService.StartVersionCheck）。
// DB 操作用独立 context：避免 Activity ctx 的心跳/ScheduleToClose deadline 截断长事务（优雅取消后续 GP2-04 接入）。
func ActivityRunVersionCheck(ctx context.Context, teamID, runID int64) (*VersionCheckResult, error) {
	actCtx := rls.WithTenant(context.Background(), teamID)
	run, err := deps.runSvc.StartVersionCheck(actCtx, runID)
	if err != nil {
		activity.GetLogger(ctx).Error("version check failed", "run_id", runID, "err", err)
		return &VersionCheckResult{Ok: false}, nil
	}
	return &VersionCheckResult{Ok: run.State == domain.RunScheduled}, nil
}

// ActivityExecuteRun 执行（collect→gate→report→done）。
func ActivityExecuteRun(ctx context.Context, teamID, runID int64) (*ExecuteResult, error) {
	actCtx := rls.WithTenant(context.Background(), teamID)
	run, results, err := deps.runSvc.ExecuteRun(actCtx, runID)
	if err != nil {
		return nil, err
	}
	pass := 0
	for _, c := range results {
		if c.Status == domain.CasePass {
			pass++
		}
	}
	return &ExecuteResult{State: run.State, PassCount: pass, TotalCount: len(results)}, nil
}
