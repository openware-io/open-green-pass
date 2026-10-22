// CoveragePort adapter: execution result -> governance coverage ledger.
package infra

import (
	"context"

	governanceapp "github.com/openware-io/open-green-pass/internal/governance/application"
)

// GovernanceCoveragePort implements execution's CoveragePort.
type GovernanceCoveragePort struct {
	svc *governanceapp.CoverageService
}

// NewGovernanceCoveragePort creates the adapter.
func NewGovernanceCoveragePort(svc *governanceapp.CoverageService) *GovernanceCoveragePort {
	return &GovernanceCoveragePort{svc: svc}
}

// MarkCaseResult updates the coverage cell bound to the case.
func (p *GovernanceCoveragePort) MarkCaseResult(ctx context.Context, caseID int64, status string) error {
	return p.svc.MarkCaseResult(ctx, caseID, status)
}
