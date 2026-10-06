package domain

import "context"

// GatePort is the execution-to-trusted boundary. Execution only requires the
// durable decision; policy parsing and trusted persistence remain outside this
// domain.
type GatePort interface {
	Evaluate(context.Context, int64) (GateDecision, error)
}

// GateDecision is intentionally small: a Run records operational completion
// independently of whether its quality gate passed, failed, or was blocked.
type GateDecision string

const (
	GatePass    GateDecision = "pass"
	GateFail    GateDecision = "fail"
	GateBlocked GateDecision = "blocked"
)

// ReportPort generates the immutable trusted report for an executed Run.
type ReportPort interface {
	Generate(context.Context, int64) error
}

// CostRecord is an execution-side measurement. Tokens and unit price are
// factual inputs supplied by the runner; zero is meaningful for a real API
// probe that did not call an AI provider.
type CostRecord struct {
	RequestID string
	TargetID  int64
	RunID     int64
	CaseID    int64
	Attempt   int
	Model     string
	TokensIn  int64
	TokensOut int64
	UnitPrice float64
}

// CostPort is the only execution-to-trusted measurement boundary.
type CostPort interface {
	Record(context.Context, CostRecord) error
}

type NoopGatePort struct{}

func (NoopGatePort) Evaluate(context.Context, int64) (GateDecision, error) { return GatePass, nil }

type NoopReportPort struct{}

func (NoopReportPort) Generate(context.Context, int64) error { return nil }

type NoopCostPort struct{}

func (NoopCostPort) Record(context.Context, CostRecord) error { return nil }
