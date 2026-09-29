// Package domain 定义执行域核心概念：测试执行 / 用例执行 / 批量执行。
// 本域负责执行编排与调度语义（工作流实现在 infra/workflow，见 TECH-DESIGN §D3）。
package domain

// RunStatus 测试执行状态。
type RunStatus string

const (
	RunPending  RunStatus = "pending"
	RunRunning  RunStatus = "running"
	RunPassed   RunStatus = "passed"
	RunFailed   RunStatus = "failed"
	RunSkipped  RunStatus = "skipped"
	RunPaused   RunStatus = "paused"
	RunTimedOut RunStatus = "timed_out"
)

// Run 一次测试执行批次。
type Run struct {
	ID         int64
	TeamID     int64
	TargetID   int64 // 被测对象
	Status     RunStatus
	Attempt    int // 尝试序号（幂等/重试语义）
	StartedAt  *int64
	FinishedAt *int64
}

// CaseResult 单个用例执行结果。
type CaseResult struct {
	ID          int64
	TeamID      int64
	RunID       int64
	CaseID      int64
	AttemptSeq  int // 该用例的尝试序号（幂等键组成部分）
	Status      RunStatus
	EvidenceRef *string // 截图/证据引用
	CostRef     *string // 关联成本行
}
