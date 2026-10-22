package application

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/clock"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type fakeFindingRepo struct {
	findings map[int64]*domain.Finding
	byHash   map[string]*domain.Finding
	events   []*domain.FindingEvent
}

func newFakeFindingRepo() *fakeFindingRepo {
	return &fakeFindingRepo{findings: map[int64]*domain.Finding{}, byHash: map[string]*domain.Finding{}}
}

func (r *fakeFindingRepo) Create(_ context.Context, f *domain.Finding, e *domain.FindingEvent) (bool, error) {
	if _, ok := r.byHash[f.DedupeHash]; ok {
		return false, nil
	}
	r.findings[f.ID] = f
	r.byHash[f.DedupeHash] = f
	r.events = append(r.events, e)
	return true, nil
}
func (r *fakeFindingRepo) Find(_ context.Context, _, id int64) (*domain.Finding, error) {
	return r.findings[id], nil
}
func (r *fakeFindingRepo) FindByDedupe(_ context.Context, _ int64, hash string) (*domain.Finding, error) {
	return r.byHash[hash], nil
}
func (r *fakeFindingRepo) List(context.Context, int64, domain.FindingFilter) ([]*domain.Finding, error) {
	return nil, nil
}
func (r *fakeFindingRepo) UpdateStatus(_ context.Context, f *domain.Finding, e *domain.FindingEvent) error {
	r.findings[f.ID] = f
	r.events = append(r.events, e)
	return nil
}
func (r *fakeFindingRepo) Events(context.Context, int64, int64) ([]domain.FindingEvent, error) {
	return nil, nil
}
func (r *fakeFindingRepo) CountOpen(context.Context, int64, int64, string) (int, int, error) {
	return 0, 0, nil
}

func findingInput() CreateFindingInput {
	return CreateFindingInput{
		ProjectID: 1, RepoName: "open-chat-admin", RepoURL: "https://github.com/openware-io/open-chat-admin",
		Branch: "2.2.1", CommitSHA: "a4487643", Module: "user", Endpoint: "DELETE /admin/users/{id}",
		Title: "delete user ignores confirm username", Severity: domain.SeverityP1,
		Category: "product", Expected: "400", Actual: "200", RootCause: "confirm check missing",
	}
}

func TestCreateFindingUsesTenantAndDedupe(t *testing.T) {
	repo := newFakeFindingRepo()
	svc := NewFindingService(repo, mustGenerator(t))
	ctx := rls.WithTenant(context.Background(), 1001)
	f, created, err := svc.Create(ctx, findingInput())
	if err != nil || !created {
		t.Fatalf("create: created=%v err=%v", created, err)
	}
	if f.TeamID != 1001 || f.Status != domain.FindingCandidate || f.ID == 0 {
		t.Fatalf("unexpected finding: %+v", f)
	}
	again, created, err := svc.Create(ctx, findingInput())
	if err != nil || created || again.ID != f.ID {
		t.Fatalf("dedupe: created=%v err=%v again=%+v", created, err, again)
	}
}

func TestCreateFindingRequiresTenant(t *testing.T) {
	svc := NewFindingService(newFakeFindingRepo(), mustGenerator(t))
	if _, _, err := svc.Create(context.Background(), findingInput()); err == nil {
		t.Fatal("expected tenant error")
	}
}

func TestTransitionRejectsIllegalAction(t *testing.T) {
	repo := newFakeFindingRepo()
	svc := NewFindingService(repo, mustGenerator(t))
	ctx := rls.WithTenant(context.Background(), 1001)
	f, _, err := svc.Create(ctx, findingInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Transition(ctx, f.ID, domain.ActionClose, ""); err == nil {
		t.Fatal("candidate cannot close")
	}
	if _, err := svc.Transition(ctx, f.ID, domain.ActionConfirm, "confirmed"); err != nil {
		t.Fatal(err)
	}
}

func mustGenerator(t *testing.T) *id.Generator {
	t.Helper()
	g, err := id.New(1, clock.System())
	if err != nil {
		t.Fatal(err)
	}
	return g
}
