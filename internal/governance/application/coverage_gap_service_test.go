package application

import (
	"context"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

type gapRepoStub struct{ cells []domain.CoverageCell }

func (s *gapRepoStub) SaveEndpoints(context.Context, int64, []domain.Endpoint) ([]domain.Endpoint, error) {
	return nil, nil
}
func (s *gapRepoStub) SaveCells(context.Context, int64, []domain.CoverageCell) ([]domain.CoverageCell, error) {
	return nil, nil
}
func (s *gapRepoStub) ListCells(context.Context, int64, domain.CoverageFilter) ([]domain.CoverageCell, error) {
	return s.cells, nil
}
func (s *gapRepoStub) Summary(context.Context, int64, domain.CoverageFilter) (domain.CoverageSummary, error) {
	return domain.CoverageSummary{}, nil
}
func (s *gapRepoStub) AssignCase(context.Context, int64, []int64, int64) error { return nil }
func (s *gapRepoStub) MarkCaseResult(context.Context, int64, int64, string) error {
	return nil
}

func TestCoverageGapsAggregatesAndSkipsWaived(t *testing.T) {
	now := time.Now().UTC()
	expired := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	waiverID := int64(1)
	expiredWaiverID := int64(2)
	caseID := int64(10)
	repo := &gapRepoStub{cells: []domain.CoverageCell{
		{Module: "alpha", Dimension: "happy", Required: true, Status: "passed", CaseID: &caseID},
		{Module: "alpha", Dimension: "authn", Required: true, Status: "planned"},
		{Module: "beta", Dimension: "happy", Required: true, Status: "waived", WaiverID: &waiverID, WaiverExpiresAt: &future},
		{Module: "beta", Dimension: "happy", Required: true, Status: "waived", WaiverID: &expiredWaiverID, WaiverExpiresAt: &expired},
		{Module: "beta", Dimension: "happy", Required: false, Status: "planned"},
	}}
	svc := NewCoverageService(repo)
	report, err := svc.Gaps(rls.WithTenant(context.Background(), 1), domain.CoverageFilter{ProjectID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if report.Total.Waived != 1 {
		t.Fatalf("unexpected waived: %+v", report.Total)
	}
	if report.Total.Required != 3 || report.Total.Mapped != 1 || report.Total.Passed != 1 || report.Total.Unmapped != 2 {
		t.Fatalf("unexpected total: %+v", report.Total)
	}
	if len(report.Modules) != 2 || report.Modules[0].Module != "alpha" || report.Modules[1].Module != "beta" {
		t.Fatalf("unexpected modules: %+v", report.Modules)
	}
}
