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

var _ domain.GatePort = (*TrustedGatePort)(nil)
var _ domain.ReportPort = (*TrustedReportPort)(nil)
