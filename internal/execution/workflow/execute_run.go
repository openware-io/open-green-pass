// Package workflow 执行流水线的 Temporal Workflow 编排（GP2-02）。
// 现状：把 P1 同步执行链（版本校验→执行→终态推进）纳入 Temporal 编排，
// 复用现有 RunService，获得持久化/重试/信号（暂停/恢复）/可见性能力；
// 后续按 GP2-02b 拆为 version→dispatch→collect→gate→cost→report 细粒度 Activity。
package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

// ExecuteRunInput Workflow 输入：待执行的运行与租户。
type ExecuteRunInput struct {
	RunID  int64
	TeamID int64
}

// ExecuteRunResult Workflow 结果：最终运行状态 + 通过用例数。
type ExecuteRunResult struct {
	State      domain.RunState
	PassCount  int
	TotalCount int
}

// ExecuteRunWorkflow 测试运行执行 Workflow：版本校验 → 执行（collect/gate/report/done）。
func ExecuteRunWorkflow(ctx workflow.Context, input ExecuteRunInput) (*ExecuteRunResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute, // 长执行容忍
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// 1) 版本校验（"测了没白测"前置阻断）
	var vc VersionCheckResult
	if err := workflow.ExecuteActivity(ctx, ActivityRunVersionCheck, input.TeamID, input.RunID).Get(ctx, &vc); err != nil {
		return nil, err
	}
	if !vc.Ok {
		return &ExecuteRunResult{State: domain.RunFailed}, nil
	}

	// 2) 执行（含 collect→gate→report→done）
	var ex ExecuteResult
	if err := workflow.ExecuteActivity(ctx, ActivityExecuteRun, input.TeamID, input.RunID).Get(ctx, &ex); err != nil {
		return nil, err
	}
	return &ExecuteRunResult{State: ex.State, PassCount: ex.PassCount, TotalCount: ex.TotalCount}, nil
}
