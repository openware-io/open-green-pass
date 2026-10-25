package application

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

type gateRepoStub struct {
	gapRepoStub
	summary domain.CoverageSummary
}

func (s *gateRepoStub) Summary(context.Context, int64, domain.CoverageFilter) (domain.CoverageSummary, error) {
	return s.summary, nil
}

type gateFindingsStub struct{}

func (gateFindingsStub) Create(context.Context, *domain.Finding, *domain.FindingEvent) (bool, error) {
	return false, nil
}
func (gateFindingsStub) Find(context.Context, int64, int64) (*domain.Finding, error) {
	return nil, nil
}
func (gateFindingsStub) FindByDedupe(context.Context, int64, string) (*domain.Finding, error) {
	return nil, nil
}
func (gateFindingsStub) List(context.Context, int64, domain.FindingFilter) ([]*domain.Finding, error) {
	return nil, nil
}
func (gateFindingsStub) UpdateStatus(context.Context, *domain.Finding, *domain.FindingEvent) error {
	return nil
}
func (gateFindingsStub) Events(context.Context, int64, int64) ([]domain.FindingEvent, error) {
	return nil, nil
}
func (gateFindingsStub) CountOpen(context.Context, int64, int64, string) (int, int, error) {
	return 0, 0, nil
}

func TestCoverageGateRiskRequiresSmokeAndHighRiskCoverage(t *testing.T) {
	repo := &gateRepoStub{summary: domain.CoverageSummary{TotalRequired: 6, DesignCovered: 5, Executed: 5, Passed: 3}}
	repo.cells = []domain.CoverageCell{
		{Module: "auth", Dimension: "happy", Required: true, Status: "passed"},
		{Module: "user", Dimension: "happy", Required: true, Status: "passed"},
		{Module: "server", Dimension: "happy", Required: true, Status: "planned"},
		{Module: "config", Dimension: "happy", Required: true, Status: "planned"},
		{Module: "media", Dimension: "happy", Required: true, Status: "blocked"},
		{Module: "clientRelease", Dimension: "happy", Required: true, Status: "passed"},
	}
	svc := NewCoverageGateService(repo, gateFindingsStub{})
	result, err := svc.EvaluateMode(rls.WithTenant(context.Background(), 1), 7, "", "risk")
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != "risk" || result.PassedGate {
		t.Fatalf("expected blocking risk result, got %+v", result)
	}
	if len(result.Blockers) < 2 {
		t.Fatalf("expected smoke blockers, got %+v", result.Blockers)
	}
	if result.Modules == nil {
		t.Fatal("expected module aggregation")
	}
}

func TestCoverageGateRejectsUnknownMode(t *testing.T) {
	svc := NewCoverageGateService(&gateRepoStub{}, gateFindingsStub{})
	if _, err := svc.EvaluateMode(rls.WithTenant(context.Background(), 1), 7, "", "loose"); err == nil {
		t.Fatal("unknown mode must be rejected")
	}
}
