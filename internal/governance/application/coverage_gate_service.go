// Package application 覆盖/缺陷门禁：以账本数据计算，不接受 Agent 自报。
package application

import (
	"context"
	"fmt"
	"math"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

// CoverageGateResult 门禁结果。
type CoverageGateResult struct {
	ProjectID         int64    `json:"project_id"`
	Module            string   `json:"module"`
	TotalRequired     int      `json:"total_required"`
	DesignCovered     int      `json:"design_covered"`
	Executed          int      `json:"executed"`
	Passed            int      `json:"passed"`
	DesignCoverage    float64  `json:"design_coverage"`
	ExecutionCoverage float64  `json:"execution_coverage"`
	OpenP0            int      `json:"open_p0"`
	OpenP1            int      `json:"open_p1"`
	PassedGate        bool     `json:"passed"`
	Blockers          []string `json:"blockers"`
}

// CoverageGateService 计算覆盖 + 缺陷门禁。
type CoverageGateService struct {
	coverage domain.CoverageRepository
	findings domain.FindingRepository
}

// NewCoverageGateService 创建服务。
func NewCoverageGateService(coverage domain.CoverageRepository, findings domain.FindingRepository) *CoverageGateService {
	return &CoverageGateService{coverage: coverage, findings: findings}
}

// Evaluate 计算门禁：design 100%、execution≥90%、未关闭 P0/P1=0。
func (s *CoverageGateService) Evaluate(ctx context.Context, projectID int64, module string) (*CoverageGateResult, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	if projectID <= 0 {
		return nil, gperr.Validation("project_id required")
	}
	summary, err := s.coverage.Summary(ctx, teamID, domain.CoverageFilter{ProjectID: projectID, Module: module})
	if err != nil {
		return nil, err
	}
	p0, p1, err := s.findings.CountOpen(ctx, teamID, projectID, module)
	if err != nil {
		return nil, err
	}
	result := &CoverageGateResult{
		ProjectID: projectID, Module: module, TotalRequired: summary.TotalRequired,
		DesignCovered: summary.DesignCovered, Executed: summary.Executed, Passed: summary.Passed,
		OpenP0: p0, OpenP1: p1, Blockers: []string{},
	}
	if summary.TotalRequired > 0 {
		result.DesignCoverage = round3(float64(summary.DesignCovered) / float64(summary.TotalRequired))
		result.ExecutionCoverage = round3(float64(summary.Executed) / float64(summary.TotalRequired))
	}
	if summary.TotalRequired == 0 {
		result.Blockers = append(result.Blockers, "no required coverage cells")
	}
	if result.DesignCoverage < 1 {
		result.Blockers = append(result.Blockers, fmt.Sprintf("design coverage %.1f%% < 100%%", result.DesignCoverage*100))
	}
	if result.ExecutionCoverage < 0.9 {
		result.Blockers = append(result.Blockers, fmt.Sprintf("execution coverage %.1f%% < 90%%", result.ExecutionCoverage*100))
	}
	if p0 > 0 || p1 > 0 {
		result.Blockers = append(result.Blockers, fmt.Sprintf("open P0/P1 findings: %d/%d", p0, p1))
	}
	result.PassedGate = len(result.Blockers) == 0
	return result, nil
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
