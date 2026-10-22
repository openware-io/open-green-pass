// Package application 执行结果回写覆盖账本。
package application

import (
	"context"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

// CoveragePort marks coverage cells from execution results.
type CoveragePort interface {
	MarkCaseResult(ctx context.Context, caseID int64, status string) error
}

// SyncCoverage reads persisted run results and updates coverage cells.
func (s *RunService) SyncCoverage(ctx context.Context, runID int64) (int, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return 0, gperr.Unauthorized("tenant required")
	}
	if s.coverage == nil {
		return 0, gperr.Validation("coverage port not configured")
	}
	results, err := s.repo.CaseResults(ctx, teamID, runID)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, result := range results {
		status := string(result.Status)
		switch result.Status {
		case domain.CasePass:
			status = "passed"
		case domain.CaseFail:
			status = "failed"
		case domain.CaseBlocked:
			status = "blocked"
		}
		if err := s.coverage.MarkCaseResult(ctx, result.CaseID, status); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}
