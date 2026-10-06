package infra

import (
	"context"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	trusteddomain "github.com/openware-io/open-green-pass/internal/trusted/domain"
)

type trustedGateEvaluator interface {
	Evaluate(context.Context, int64) (*trusteddomain.GateResult, error)
}

// TrustedGatePort maps the trusted decision into the execution boundary.
type TrustedGatePort struct{ service trustedGateEvaluator }

func NewTrustedGatePort(service *trustedapp.GateService) *TrustedGatePort {
	return &TrustedGatePort{service: service}
}

func (p *TrustedGatePort) Evaluate(ctx context.Context, runID int64) (domain.GateDecision, error) {
	result, err := p.service.Evaluate(ctx, runID)
	if err != nil {
		return "", err
	}
	return domain.GateDecision(result.Result), nil
}

type trustedReportGenerator interface {
	Generate(context.Context, trustedapp.GenerateRequest) (*trusteddomain.Report, error)
}

// TrustedReportPort creates the normal P1 service report after every Run.
type TrustedReportPort struct{ service trustedReportGenerator }

func NewTrustedReportPort(service *trustedapp.ReportService) *TrustedReportPort {
	return &TrustedReportPort{service: service}
}

func (p *TrustedReportPort) Generate(ctx context.Context, runID int64) error {
	_, err := p.service.Generate(ctx, trustedapp.GenerateRequest{RunID: runID, Kind: "service"})
	return err
}

type trustedCostRecorder interface {
	Record(context.Context, trustedapp.RecordRequest) (bool, error)
}

// TrustedCostPort persists exact runner-provided measurements. In particular,
// zero tokens are retained rather than invented as an AI charge.
type TrustedCostPort struct{ service trustedCostRecorder }

func NewTrustedCostPort(service *trustedapp.CostService) *TrustedCostPort {
	return &TrustedCostPort{service: service}
}

func (p *TrustedCostPort) Record(ctx context.Context, record domain.CostRecord) error {
	_, err := p.service.Record(ctx, trustedapp.RecordRequest{
		RequestID: record.RequestID,
		BizPoint:  "case_execute",
		TargetID:  record.TargetID,
		RunID:     record.RunID,
		CaseID:    record.CaseID,
		Model:     record.Model,
		TokensIn:  record.TokensIn,
		TokensOut: record.TokensOut,
		UnitPrice: record.UnitPrice,
	})
	return err
}

var _ domain.GatePort = (*TrustedGatePort)(nil)
var _ domain.ReportPort = (*TrustedReportPort)(nil)
var _ domain.CostPort = (*TrustedCostPort)(nil)
