package application

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/clock"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type triageRepo struct {
	result     *domain.CaseResult
	markedID   int64
	markedFind int64
}

func (r *triageRepo) Save(context.Context, *domain.Run) error { return nil }
func (r *triageRepo) Find(context.Context, int64, int64) (*domain.Run, error) {
	return nil, nil
}
func (r *triageRepo) List(context.Context, int64, *int64, string, int) ([]*domain.Run, error) {
	return nil, nil
}
func (r *triageRepo) SaveCaseResult(context.Context, *domain.CaseResult) error { return nil }
func (r *triageRepo) CaseResults(context.Context, int64, int64) ([]*domain.CaseResult, error) {
	return []*domain.CaseResult{r.result}, nil
}
func (r *triageRepo) MaxAttempt(context.Context, int64, int64, int64) (int, error) { return 1, nil }
func (r *triageRepo) MarkCaseResultFinding(_ context.Context, _ int64, resultID, findingID int64, _ string) error {
	r.markedID = resultID
	r.markedFind = findingID
	return nil
}

type triageFindingPort struct{ draft FindingDraft }

func (p *triageFindingPort) CreateFinding(_ context.Context, draft FindingDraft) (int64, error) {
	p.draft = draft
	return 999, nil
}

func TestTriageCreatesFindingAndLinksResult(t *testing.T) {
	g, _ := id.New(1, clock.System())
	repo := &triageRepo{result: &domain.CaseResult{
		ID: 7, RunID: 3, CaseID: 42, CaseVersion: 1, Status: domain.CaseFail,
		ResultText: "HTTP 500",
		Evidence:   &domain.EvidenceRef{Logs: []string{"error.log"}},
	}}
	port := &triageFindingPort{}
	svc := NewRunService(repo, nil, nil, nil, nil, g)
	svc.SetFindingPort(port)
	ctx := rls.WithTenant(context.Background(), 1001)
	result, err := svc.Triage(ctx, 3, TriageInput{
		CaseResultID: 7, ProjectID: 1, RepoURL: "https://github.com/a/b", CommitSHA: "abc",
		Title: "callback 500", Severity: "P0", Expected: "401", Actual: "500",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FindingID == nil || *result.FindingID != 999 || repo.markedID != 7 || repo.markedFind != 999 {
		t.Fatalf("result=%+v marked=%d/%d", result, repo.markedID, repo.markedFind)
	}
	if port.draft.LinkedRunID != 3 || port.draft.LinkedCaseID != 42 || port.draft.Log == "" {
		t.Fatalf("draft=%+v", port.draft)
	}
}

func TestTriageRejectsPassedResult(t *testing.T) {
	g, _ := id.New(1, clock.System())
	repo := &triageRepo{result: &domain.CaseResult{ID: 7, Status: domain.CasePass}}
	svc := NewRunService(repo, nil, nil, nil, nil, g)
	svc.SetFindingPort(&triageFindingPort{})
	if _, err := svc.Triage(rls.WithTenant(context.Background(), 1001), 3, TriageInput{CaseResultID: 7}); err == nil {
		t.Fatal("expected conflict")
	}
}
