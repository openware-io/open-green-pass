// 可信域门禁核心概念：门禁规则（策略即代码）+ 判定结果 + 执行→可信端口。
// 策略以 Rego 形式进 git（deploy/rego/），P1 用声明式策略评估（字段级阈值），完整 OPA 引擎留 P2。
package domain

import (
	"context"
	"errors"
	"time"
)

// ErrRunNotFound 运行不存在或租户不可见（RLS 过滤为空）。
var ErrRunNotFound = errors.New("run not found")

// GateDecision 门禁判定结果。
type GateDecision string

const (
	GatePass    GateDecision = "pass"    // 通过
	GateFail    GateDecision = "fail"    // 未达标
	GateBlocked GateDecision = "blocked" // 阻断（策略未加载/上下文缺失）
)

// GateRule 门禁规则（策略即代码；Rego 装载，字段级声明式策略见 Policy）。
type GateRule struct {
	ID         int64
	TeamID     int64
	TargetID   int64
	ScenarioID int64
	Rego       string // 策略体（Rego 或声明式策略 JSON）
	Version    int
	Enabled    bool
}

// GatePolicy 声明式门禁策略（P1 评估口径；完整 Rego/OPA P2 接入）。
type GatePolicy struct {
	MinPassRate float64 `json:"min_pass_rate"` // 通过率下限（0~1）
	MaxFailed   int64   `json:"max_failed"`    // 最大失败用例数
	MinCoverage float64 `json:"min_coverage"`  // 覆盖率下限
}

// GateResult 门禁判定结果。
type GateResult struct {
	ID        int64
	TeamID    int64
	RunID     int64
	RuleID    int64
	Result    GateDecision
	Detail    map[string]any
	DecidedAt time.Time
}

// GateRepository 门禁仓储端口。
type GateRepository interface {
	UpsertRule(ctx context.Context, r *GateRule) error
	ActiveRule(ctx context.Context, teamID, targetID, scenarioID int64) (*GateRule, error)
	SaveResult(ctx context.Context, r *GateResult) error
	ResultsByRun(ctx context.Context, teamID, runID int64) ([]*GateResult, error)
}

// GatePort 执行域→可信域门禁端口（RunService 在 gate 阶段调用）。
type GatePort interface {
	Evaluate(ctx context.Context, runID int64) (*GateResult, error)
}

// RunStatPort 读取执行域运行元信息与用例统计（trusted 只读 run_*）。
type RunStatPort interface {
	RunMeta(ctx context.Context, teamID, runID int64) (targetID, scenarioID int64, err error)
	CaseStats(ctx context.Context, teamID, runID int64) (total, pass, fail int64, err error)
	RunHead(ctx context.Context, teamID, runID int64) (*RunHead, error)
	CaseResults(ctx context.Context, teamID, runID int64) ([]*CaseResult, error)
}

// RunHead 运行头信息（报告聚合用）。
type RunHead struct {
	TargetID, ScenarioID int64
	Env, Version, Branch, RunMode, State string
	StartedAt, EndedAt                   time.Time
}

// CaseResult 用例执行结果（报告聚合用，最新 attempt）。
type CaseResult struct {
	CaseID   int64
	CaseCode string
	Kind     string
	Status   string
	Attempt  int
	Evidence map[string]any
	TokensIn int64
	TokensOut int64
	Cost     float64
	StartedAt, EndedAt time.Time
}
