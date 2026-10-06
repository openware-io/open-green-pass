package application

import (
	"context"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type pipelineRepo struct {
	run   *domain.Run
	saves []domain.RunState
}

func (r *pipelineRepo) Save(_ context.Context, run *domain.Run) error {
	r.saves = append(r.saves, run.State)
	return nil
}
func (r *pipelineRepo) Find(context.Context, int64, int64) (*domain.Run, error) { return r.run, nil }
func (r *pipelineRepo) List(context.Context, int64, *int64, string, int) ([]*domain.Run, error) {
	return nil, nil
}
func (r *pipelineRepo) SaveCaseResult(context.Context, *domain.CaseResult) error { return nil }
func (r *pipelineRepo) CaseResults(context.Context, int64, int64) ([]*domain.CaseResult, error) {
	return nil, nil
}
func (r *pipelineRepo) MaxAttempt(context.Context, int64, int64, int64) (int, error) { return 0, nil }

type pipelineCases struct{}

func (pipelineCases) ListIDsByTarget(context.Context, int64, int64) ([]int64, error) {
	return []int64{4001}, nil
}
func (pipelineCases) FetchScript(context.Context, int64, int64) (int, any, error) {
	return 1, map[string]any{"scenario": "api"}, nil
}

type pipelinePolicy struct{}

func (pipelinePolicy) Get(context.Context, int64, int64, int64) (*domain.ScreenshotPolicy, error) {
	return &domain.ScreenshotPolicy{}, nil
}

type pipelineRunner struct{}

func (pipelineRunner) Execute(_ context.Context, specs []*domain.CaseSpec) ([]*domain.CaseResult, error) {
	now := time.Now().UTC()
	return []*domain.CaseResult{{CaseID: specs[0].CaseID, CaseVersion: specs[0].CaseVersion, Status: domain.CasePass, StartedAt: now, EndedAt: &now}}, nil
}

type pipelineGate struct{ calls []int64 }

func (g *pipelineGate) Evaluate(_ context.Context, runID int64) (domain.GateDecision, error) {
	g.calls = append(g.calls, runID)
	return domain.GatePass, nil
}

type pipelineReport struct{ calls []int64 }

func (r *pipelineReport) Generate(_ context.Context, runID int64) error {
	r.calls = append(r.calls, runID)
	return nil
}

type pipelineCost struct{ records []domain.CostRecord }

func (c *pipelineCost) Record(_ context.Context, record domain.CostRecord) error {
	c.records = append(c.records, record)
	return nil
}

func TestExecuteRunInvokesTrustedGateAndReportBeforeDone(t *testing.T) {
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := domain.NewRun(9, 100, 1, 1003, "test", "v1", "main", "manual", []int64{4001})
	run.State = domain.RunScheduled
	repo := &pipelineRepo{run: run}
	gate := &pipelineGate{}
	report := &pipelineReport{}
	cost := &pipelineCost{}
	svc := NewRunService(repo, nil, pipelineCases{}, pipelinePolicy{}, pipelineRunner{}, gen)
	svc.SetGatePort(gate)
	svc.SetReportPort(report)
	svc.SetCostPort(cost)

	completed, _, err := svc.ExecuteRun(rls.WithTenant(context.Background(), 100), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != domain.RunDone || len(gate.calls) != 1 || len(report.calls) != 1 || len(cost.records) != 1 || gate.calls[0] != run.ID || report.calls[0] != run.ID {
		t.Fatalf("run=%s gate=%v report=%v cost=%v", completed.State, gate.calls, report.calls, cost.records)
	}
	if got := cost.records[0]; got.RequestID != "run:9:case:4001:attempt:1" || got.Model != "k8s-api-runner" || got.TokensIn != 0 || got.TokensOut != 0 {
		t.Fatalf("cost=%+v", got)
	}
	want := []domain.RunState{domain.RunCollect, domain.RunGate, domain.RunReport, domain.RunDone}
	if len(repo.saves) < len(want) {
		t.Fatalf("states=%v", repo.saves)
	}
	got := repo.saves[len(repo.saves)-len(want):]
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("states=%v want=%v", got, want)
		}
	}
}
