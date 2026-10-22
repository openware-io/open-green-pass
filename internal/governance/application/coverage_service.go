// Package application 治理域应用服务：覆盖计划与覆盖账本。
package application

import (
	"context"
	"strings"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

// CoverageService 覆盖账本应用服务。
type CoverageService struct {
	repo domain.CoverageRepository
}

// NewCoverageService 创建覆盖账本服务。
func NewCoverageService(repo domain.CoverageRepository) *CoverageService {
	return &CoverageService{repo: repo}
}

// Plan 保存端点并生成 Coverage Cell，返回 Cell + 汇总。
func (s *CoverageService) Plan(ctx context.Context, projectID int64, serviceID *int64, module string, endpoints []domain.Endpoint) ([]domain.CoverageCell, domain.CoverageSummary, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, domain.CoverageSummary{}, gperr.Unauthorized("tenant required")
	}
	if projectID <= 0 {
		return nil, domain.CoverageSummary{}, gperr.Validation("project_id required")
	}
	if len(endpoints) == 0 {
		return nil, domain.CoverageSummary{}, gperr.Validation("endpoints required")
	}
	for i := range endpoints {
		endpoints[i].TeamID = teamID
		endpoints[i].ProjectID = projectID
		endpoints[i].Module = strings.TrimSpace(firstNonEmpty(endpoints[i].Module, module))
		endpoints[i].Method = strings.ToUpper(strings.TrimSpace(endpoints[i].Method))
		endpoints[i].Path = strings.TrimSpace(endpoints[i].Path)
		if endpoints[i].Method == "" || endpoints[i].Path == "" {
			return nil, domain.CoverageSummary{}, gperr.Validation("endpoint method/path required")
		}
		if endpoints[i].AuthMode == "" {
			endpoints[i].AuthMode = "bearer"
		}
	}
	saved, err := s.repo.SaveEndpoints(ctx, teamID, endpoints)
	if err != nil {
		return nil, domain.CoverageSummary{}, err
	}
	cells := domain.BuildCoveragePlan(projectID, saved)
	savedCells, err := s.repo.SaveCells(ctx, teamID, cells)
	if err != nil {
		return nil, domain.CoverageSummary{}, err
	}
	summary, err := s.repo.Summary(ctx, teamID, domain.CoverageFilter{ProjectID: projectID, Module: module})
	if err != nil {
		return nil, domain.CoverageSummary{}, err
	}
	return savedCells, summary, nil
}

// List 查询覆盖账本。
func (s *CoverageService) List(ctx context.Context, filter domain.CoverageFilter) ([]domain.CoverageCell, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	return s.repo.ListCells(ctx, teamID, filter)
}

// MarkCaseResult 回写用例执行状态到覆盖 Cell。
func (s *CoverageService) MarkCaseResult(ctx context.Context, caseID int64, status string) error {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return gperr.Unauthorized("tenant required")
	}
	return s.repo.MarkCaseResult(ctx, teamID, caseID, status)
}

// Summary 查询覆盖汇总。
func (s *CoverageService) Summary(ctx context.Context, filter domain.CoverageFilter) (domain.CoverageSummary, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return domain.CoverageSummary{}, gperr.Unauthorized("tenant required")
	}
	return s.repo.Summary(ctx, teamID, filter)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
