// Package application 覆盖/缺陷门禁：以账本数据计算，不接受 Agent 自报。
package application

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

// CoverageGateModule 是风险模式下按模块聚合的门禁数据。
type CoverageGateModule struct {
	Module            string  `json:"module"`
	Required          int     `json:"required"`
	Mapped            int     `json:"mapped"`
	Passed            int     `json:"passed"`
	DesignCoverage    float64 `json:"design_coverage"`
	ExecutionCoverage float64 `json:"execution_coverage"`
}

// CoverageGateResult 门禁结果。
type CoverageGateResult struct {
	ProjectID         int64                `json:"project_id"`
	Surface           string               `json:"surface"`
	Module            string               `json:"module"`
	Mode              string               `json:"mode"`
	TotalRequired     int                  `json:"total_required"`
	DesignCovered     int                  `json:"design_covered"`
	Executed          int                  `json:"executed"`
	Passed            int                  `json:"passed"`
	DesignCoverage    float64              `json:"design_coverage"`
	ExecutionCoverage float64              `json:"execution_coverage"`
	OpenP0            int                  `json:"open_p0"`
	OpenP1            int                  `json:"open_p1"`
	PassedGate        bool                 `json:"gate_passed"`
	Blockers          []string             `json:"blockers"`
	Modules           []CoverageGateModule `json:"modules,omitempty"`
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

// Evaluate 保持严格模式兼容：design 100%、execution≥90%、未关闭 P0/P1=0。
func (s *CoverageGateService) Evaluate(ctx context.Context, projectID int64, module string) (*CoverageGateResult, error) {
	return s.EvaluateMode(ctx, projectID, module, "strict")
}

// EvaluateMode 支持 strict 与 risk 两种验收口径。
//
// strict: 原有全量 100%/90%/P0P1=0。
// risk:   18 模块 smoke + 高风险模块 design/execution≥90% + P0/P1=0。
// 风险模式只改变验收口径，不修改 Cell 的 required/passed 事实。
func (s *CoverageGateService) EvaluateMode(ctx context.Context, projectID int64, module, mode string) (*CoverageGateResult, error) {
	switch mode {
	case "", "strict":
		return s.evaluateStrict(ctx, projectID, module)
	case "risk":
		return s.evaluateRisk(ctx, projectID, module)
	default:
		return nil, gperr.Validation("invalid mode: use strict or risk")
	}
}

func (s *CoverageGateService) evaluateStrict(ctx context.Context, projectID int64, module string) (*CoverageGateResult, error) {
	summary, p0, p1, err := s.load(ctx, projectID, module)
	if err != nil {
		return nil, err
	}
	result := newGateResult(projectID, module, "strict", summary, p0, p1)
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

// highRiskModules 是风险模式必须达到 90% 的模块。
var highRiskModules = map[string]bool{
	"auth": true, "user": true, "security": true, "config": true,
	"openPlatform": true, "clientRelease": true, "execution": true,
}

//nolint:gocyclo // legacy complexity; refactor tracked in lint backlog
func (s *CoverageGateService) evaluateRisk(ctx context.Context, projectID int64, module string) (*CoverageGateResult, error) {
	summary, p0, p1, err := s.load(ctx, projectID, module)
	if err != nil {
		return nil, err
	}
	teamID, _ := rls.TenantFrom(ctx)
	cells, err := s.coverage.ListCells(ctx, teamID, domain.CoverageFilter{ProjectID: projectID, Module: module})
	if err != nil {
		return nil, err
	}
	result := newGateResult(projectID, module, "risk", summary, p0, p1)
	if summary.TotalRequired == 0 {
		result.Blockers = append(result.Blockers, "no required coverage cells")
	}
	now := time.Now().UTC()
	stats := map[string]*CoverageGateModule{}
	for _, cell := range cells {
		if !cell.Required || cell.WaivedAt(now) {
			continue
		}
		row := stats[cell.Module]
		if row == nil {
			row = &CoverageGateModule{Module: cell.Module}
			stats[cell.Module] = row
		}
		row.Required++
		if cell.CaseID != nil {
			row.Mapped++
		}
		if cell.Status == "passed" {
			row.Passed++
		}
	}
	names := make([]string, 0, len(stats))
	for name := range stats {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		row := stats[name]
		row.DesignCoverage = ratio(row.Mapped, row.Required)
		row.ExecutionCoverage = ratio(row.Mapped, row.Required)
		result.Modules = append(result.Modules, *row)
		if row.Passed == 0 {
			result.Blockers = append(result.Blockers, fmt.Sprintf("module %s smoke: no passed cells", name))
		}
		if highRiskModules[name] {
			if row.DesignCoverage < 0.9 {
				result.Blockers = append(result.Blockers, fmt.Sprintf("high-risk module %s design coverage %.1f%% < 90%%", name, row.DesignCoverage*100))
			}
			if row.ExecutionCoverage < 0.9 {
				result.Blockers = append(result.Blockers, fmt.Sprintf("high-risk module %s execution coverage %.1f%% < 90%%", name, row.ExecutionCoverage*100))
			}
		}
	}
	if p0 > 0 || p1 > 0 {
		result.Blockers = append(result.Blockers, fmt.Sprintf("open P0/P1 findings: %d/%d", p0, p1))
	}
	result.PassedGate = len(result.Blockers) == 0
	return result, nil
}

func (s *CoverageGateService) load(ctx context.Context, projectID int64, module string) (domain.CoverageSummary, int, int, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return domain.CoverageSummary{}, 0, 0, gperr.Unauthorized("tenant required")
	}
	if projectID <= 0 {
		return domain.CoverageSummary{}, 0, 0, gperr.Validation("project_id required")
	}
	summary, err := s.coverage.Summary(ctx, teamID, domain.CoverageFilter{ProjectID: projectID, Module: module})
	if err != nil {
		return domain.CoverageSummary{}, 0, 0, err
	}
	p0, p1, err := s.findings.CountOpen(ctx, teamID, projectID, module)
	if err != nil {
		return domain.CoverageSummary{}, 0, 0, err
	}
	return summary, p0, p1, nil
}

func newGateResult(projectID int64, module, mode string, summary domain.CoverageSummary, p0, p1 int) *CoverageGateResult {
	result := &CoverageGateResult{
		ProjectID: projectID, Surface: "api", Module: module, Mode: mode,
		TotalRequired: summary.TotalRequired, DesignCovered: summary.DesignCovered,
		Executed: summary.Executed, Passed: summary.Passed, OpenP0: p0, OpenP1: p1,
		Blockers: []string{},
	}
	if summary.TotalRequired > 0 {
		result.DesignCoverage = round3(float64(summary.DesignCovered) / float64(summary.TotalRequired))
		result.ExecutionCoverage = round3(float64(summary.Executed) / float64(summary.TotalRequired))
	}
	return result
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return round3(float64(n) / float64(d))
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
