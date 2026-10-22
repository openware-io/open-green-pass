package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/clock"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type importCaseRepo struct {
	cases    []domain.Case
	versions []domain.CaseVersion
}

func (r *importCaseRepo) SaveCase(_ context.Context, c *domain.Case) error {
	r.cases = append(r.cases, *c)
	return nil
}
func (r *importCaseRepo) SaveVersion(_ context.Context, v *domain.CaseVersion) error {
	r.versions = append(r.versions, *v)
	return nil
}
func (r *importCaseRepo) FindCase(context.Context, int64, int64) (*domain.Case, error) {
	return nil, nil
}
func (r *importCaseRepo) FindVersion(context.Context, int64, int64, int) (*domain.CaseVersion, error) {
	return nil, nil
}
func (r *importCaseRepo) ListCases(context.Context, int64, domain.CaseFilter) ([]*domain.Case, error) {
	return nil, nil
}
func (r *importCaseRepo) History(context.Context, int64, int64) ([]*domain.CaseVersion, error) {
	return nil, nil
}

type importCoverageRepo struct{ assigned []int64 }

func (r *importCoverageRepo) SaveEndpoints(context.Context, int64, []domain.Endpoint) ([]domain.Endpoint, error) {
	return nil, nil
}
func (r *importCoverageRepo) SaveCells(context.Context, int64, []domain.CoverageCell) ([]domain.CoverageCell, error) {
	return nil, nil
}
func (r *importCoverageRepo) ListCells(context.Context, int64, domain.CoverageFilter) ([]domain.CoverageCell, error) {
	return nil, nil
}
func (r *importCoverageRepo) Summary(context.Context, int64, domain.CoverageFilter) (domain.CoverageSummary, error) {
	return domain.CoverageSummary{}, nil
}
func (r *importCoverageRepo) AssignCase(_ context.Context, _ int64, cellIDs []int64, _ int64) error {
	r.assigned = append(r.assigned, cellIDs...)
	return nil
}
func (r *importCoverageRepo) MarkCaseResult(context.Context, int64, int64, string) error {
	return nil
}

type importReader struct {
	state, output string
}

func (r importReader) ReadOutput(context.Context, int64, int64) (string, string, error) {
	return r.state, r.output, nil
}

func TestImportCasesParsesAndAssignsCoverage(t *testing.T) {
	repo := &importCaseRepo{}
	cov := &importCoverageRepo{}
	g, err := id.New(1, clock.System())
	if err != nil {
		t.Fatal(err)
	}
	output, _ := json.Marshal([]GeneratedCase{
		{Code: "user-list-001", Title: "list happy", Kind: "api", Script: json.RawMessage(`{"method":"GET"}`), CoverageCellIDs: []int64{11}},
		{Code: "user-delete-001", Title: "delete confirm", Kind: "api", Script: json.RawMessage(`{"method":"DELETE"}`), CoverageCellIDs: []int64{12, 13}},
	})
	svc := NewCaseImportService(NewCaseService(repo, g), cov, importReader{state: "succeeded", output: string(output)})
	cases, err := svc.Import(rls.WithTenant(context.Background(), 1001), 9, 2876690473259008)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 2 || len(repo.cases) != 2 || len(repo.versions) != 2 {
		t.Fatalf("cases=%d repo=%d versions=%d", len(cases), len(repo.cases), len(repo.versions))
	}
	if len(cov.assigned) != 3 {
		t.Fatalf("assigned=%v", cov.assigned)
	}
}

func TestImportRejectsNonSucceeded(t *testing.T) {
	svc := NewCaseImportService(nil, &importCoverageRepo{}, importReader{state: "running"})
	if _, err := svc.Import(rls.WithTenant(context.Background(), 1001), 9, 1); err == nil {
		t.Fatal("expected conflict")
	}
}
