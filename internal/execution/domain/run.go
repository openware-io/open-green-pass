// Run 聚合：测试运行（时间维主键）与用例执行结果。
// 状态机（合法迁移见 validTransitions）：
//   queued→version_check→scheduled→running→collect→gate→report→done
//   version_check 校验 mismatch→failed；任意阶段可 Fail；running 可 Paused/Resumed。
package domain

import (
	"context"
	"errors"
	"time"
)

// RunState 运行状态。
type RunState string

const (
	RunQueued      RunState = "queued"       // 已创建待校验
	RunVersionCheck RunState = "version_check" // 版本校验中（"测了没白测"）
	RunScheduled   RunState = "scheduled"    // 校验通过已入调度
	RunRunning     RunState = "running"      // 执行中
	RunPaused      RunState = "paused"       // 暂停
	RunCollect     RunState = "collect"      // 证据收集
	RunGate        RunState = "gate"         // 门禁判定
	RunReport      RunState = "report"       // 报告生成
	RunDone        RunState = "done"         // 完成
	RunFailed      RunState = "failed"       // 失败/阻断（如版本 mismatch）
)

// CaseStatus 用例执行结果状态。
type CaseStatus string

const (
	CasePass    CaseStatus = "pass"
	CaseFail    CaseStatus = "fail"
	CaseBlocked CaseStatus = "blocked"
	CaseSkipped CaseStatus = "skipped"
	CaseRetried CaseStatus = "retried"
)

// EvidenceRef 证据引用（截图/日志 → S3 + 哈希）。
type EvidenceRef struct {
	Screenshots []string `json:"screenshots"`
	Logs        []string `json:"logs"`
	Hash        string   `json:"hash,omitempty"`
}

// CaseResult 用例执行结果（run_case_result 明细粒）。
type CaseResult struct {
	ID           int64
	TeamID       int64
	RunID        int64
	CaseID       int64
	CaseVersion  int
	Status       CaseStatus
	ResultText   string
	Evidence     *EvidenceRef
	AITokensIn   int64
	AITokensOut  int64
	CostAmount   float64
	AttemptSeq   int
	StartedAt    time.Time
	EndedAt      *time.Time
}

// Run 测试运行聚合。
type Run struct {
	ID            int64
	TeamID        int64
	ScenarioID    int64
	TargetID      int64
	Env           string // 校验/执行环境
	TargetVersion string
	TargetBranch  string
	EnvVersion    string
	EnvCheckID    *int64
	RunMode       string // manual / ci / trigger
	State         RunState
	SelectedCases []int64 // 当次勾选用例 id；空=全量
	StartedAt     *time.Time
	EndedAt       *time.Time
}

// NewRun 创建运行（初始 queued）。
func NewRun(id, teamID, scenarioID, targetID int64, env, targetVersion, targetBranch, runMode string, selectedCases []int64) *Run {
	return &Run{
		ID: id, TeamID: teamID, ScenarioID: scenarioID, TargetID: targetID,
		Env: env, TargetVersion: targetVersion, TargetBranch: targetBranch,
		RunMode: runMode, State: RunQueued, SelectedCases: selectedCases,
	}
}

// validTransitions 状态机合法迁移表。
var validTransitions = map[RunState][]RunState{
	RunQueued:      {RunVersionCheck, RunFailed},
	RunVersionCheck: {RunScheduled, RunFailed},
	RunScheduled:   {RunRunning, RunFailed},
	RunRunning:     {RunPaused, RunCollect, RunFailed},
	RunPaused:      {RunRunning, RunFailed},
	RunCollect:     {RunGate, RunFailed},
	RunGate:        {RunReport, RunFailed},
	RunReport:      {RunDone, RunFailed},
}

// ErrInvalidTransition 非法状态迁移。
var ErrInvalidTransition = errors.New("invalid run state transition")

// ErrRunFinished 运行已终态，不可再迁移。
var ErrRunFinished = errors.New("run already finished")

// ErrRunNotFound 运行不存在或租户不可见（RLS 过滤为空）。
var ErrRunNotFound = errors.New("run not found")

func (r *Run) transition(next RunState) error {
	if r.State == RunDone || r.State == RunFailed {
		return ErrRunFinished
	}
	for _, ok := range validTransitions[r.State] {
		if ok == next {
			r.State = next
			return nil
		}
	}
	return ErrInvalidTransition
}

// StartVersionCheck queued→version_check（开始版本校验）。
func (r *Run) StartVersionCheck() error { return r.transition(RunVersionCheck) }

// MarkVersionChecked version_check→scheduled，回填校验结果；ok=false 记为 failed。
func (r *Run) MarkVersionChecked(ok bool, envVersion string, envCheckID *int64) error {
	r.EnvVersion = envVersion
	r.EnvCheckID = envCheckID
	if !ok {
		return r.transition(RunFailed)
	}
	return r.transition(RunScheduled)
}

// Schedule scheduled→running。
func (r *Run) Schedule() error { return r.transition(RunRunning) }

// Pause running→paused。
func (r *Run) Pause() error { return r.transition(RunPaused) }

// Resume paused→running。
func (r *Run) Resume() error { return r.transition(RunRunning) }

// StartCollect running/collect→collect（开始证据收集）。
func (r *Run) StartCollect() error { return r.transition(RunCollect) }

// StartGate collect→gate。
func (r *Run) StartGate() error { return r.transition(RunGate) }

// StartReport gate→report。
func (r *Run) StartReport() error { return r.transition(RunReport) }

// Finish report→done。
func (r *Run) Finish() error { return r.transition(RunDone) }

// Fail 置为 failed。
func (r *Run) Fail() error { return r.transition(RunFailed) }

// RunRepository 运行仓储端口。
type RunRepository interface {
	Save(ctx context.Context, r *Run) error
	Find(ctx context.Context, teamID, runID int64) (*Run, error)
	List(ctx context.Context, teamID int64, targetID *int64, state string, limit int) ([]*Run, error)
	SaveCaseResult(ctx context.Context, c *CaseResult) error
	CaseResults(ctx context.Context, teamID, runID int64) ([]*CaseResult, error)
	MaxAttempt(ctx context.Context, teamID, runID, caseID int64) (int, error)
}
