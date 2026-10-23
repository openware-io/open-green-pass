package application

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/ai/infra"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type memoryRunRepo struct {
	runs map[int64]*domain.GenerationRun
}

func newMemoryRunRepo() *memoryRunRepo {
	return &memoryRunRepo{runs: map[int64]*domain.GenerationRun{}}
}

func cloneRun(in *domain.GenerationRun) *domain.GenerationRun {
	out := *in
	out.Units = make([]*domain.GenerationUnit, 0, len(in.Units))
	for _, unit := range in.Units {
		copyUnit := *unit
		out.Units = append(out.Units, &copyUnit)
	}
	return &out
}

func (r *memoryRunRepo) Create(_ context.Context, run *domain.GenerationRun) error {
	r.runs[run.ID] = cloneRun(run)
	return nil
}
func (r *memoryRunRepo) Find(_ context.Context, teamID, runID int64) (*domain.GenerationRun, error) {
	run, ok := r.runs[runID]
	if !ok || run.TeamID != teamID {
		return nil, domain.ErrGenerationRunNotFound
	}
	return cloneRun(run), nil
}
func (r *memoryRunRepo) ListByProject(_ context.Context, teamID, projectID int64, limit int) ([]*domain.GenerationRun, error) {
	out := []*domain.GenerationRun{}
	for _, run := range r.runs {
		if run.TeamID == teamID && run.ProjectID == projectID {
			out = append(out, cloneRun(run))
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
func (r *memoryRunRepo) LinkBatch(_ context.Context, teamID, runID, unitID, batchID int64) error {
	run := r.runs[runID]
	for _, unit := range run.Units {
		if unit.ID == unitID {
			unit.BatchID = &batchID
			unit.State = domain.GenerationUnitRunning
			return nil
		}
	}
	return domain.ErrGenerationRunNotFound
}
func (r *memoryRunRepo) SetUnitState(_ context.Context, teamID, runID, unitID int64, state domain.GenerationUnitState, summary string) error {
	for _, unit := range r.runs[runID].Units {
		if unit.ID == unitID {
			unit.State = state
			unit.Error = summary
			return nil
		}
	}
	return domain.ErrGenerationRunNotFound
}
func (r *memoryRunRepo) UpdateState(_ context.Context, teamID, runID int64, state domain.GenerationRunState, completed, failed int) error {
	r.runs[runID].State = state
	r.runs[runID].CompletedUnits = completed
	r.runs[runID].FailedUnits = failed
	return nil
}

func TestGenerationRunCreatesUnitsAndCancelsCascade(t *testing.T) {
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	runRepo := newMemoryRunRepo()
	batchRepo := infra.NewMemoryGenerationBatchRepository()
	batchSvc := NewGenerationBatchService(batchRepo)
	batchSvc.SetScopePort(testGenerationScopePort{repoID: 30})
	svc := NewGenerationRunService(runRepo, batchSvc, gen)

	run, err := svc.Create(context.Background(), CreateGenerationRunCommand{
		ID: 1, TeamID: 10, ProjectID: 20, ScopeType: "service", ScopeID: 50,
		ActorID: 40, IdempotencyKey: "run-1",
		Units: []GenerationRunUnitInput{{ServiceID: 50, RepoID: 30, SourcePath: ".", RefType: "branch", RefName: "main"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.State != domain.GenerationRunRunning || len(run.Units) != 1 || run.Units[0].BatchID == nil {
		t.Fatalf("unexpected created run: %+v", run)
	}
	cancelled, err := svc.Cancel(context.Background(), 10, 1, 40, "cancel-1")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.State != domain.GenerationRunCancelled || cancelled.Units[0].State != domain.GenerationUnitCancelled {
		t.Fatalf("unexpected cancelled run: %+v", cancelled)
	}
	batch, err := batchSvc.Find(context.Background(), 10, *run.Units[0].BatchID)
	if err != nil || batch.State != domain.GenerationCancelled {
		t.Fatalf("batch not cancelled: %+v err=%v", batch, err)
	}
}
