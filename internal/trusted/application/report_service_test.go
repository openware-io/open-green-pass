package application

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type existingReportRepo struct{ report *domain.Report }

func (r *existingReportRepo) Save(context.Context, *domain.Report) error {
	panic("Save must not be called")
}
func (r *existingReportRepo) Find(context.Context, int64, int64) (*domain.Report, error) {
	return nil, domain.ErrReportNotFound
}
func (r *existingReportRepo) FindByRunKind(_ context.Context, teamID, runID int64, kind string) (*domain.Report, error) {
	if r.report != nil && r.report.TeamID == teamID && r.report.RunID == runID && r.report.Kind == kind {
		return r.report, nil
	}
	return nil, domain.ErrReportNotFound
}

func TestGenerateReturnsExistingRunReport(t *testing.T) {
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	existing := &domain.Report{ID: 88, TeamID: 100, RunID: 9, Kind: "service", Status: "pass"}
	svc, err := NewReportService(nil, nil, nil, &existingReportRepo{report: existing}, gen)
	if err != nil {
		t.Fatal(err)
	}
	report, err := svc.Generate(rls.WithTenant(context.Background(), 100), GenerateRequest{RunID: 9, Kind: "service"})
	if err != nil || report != existing {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}
