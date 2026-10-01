package application

import (
	"context"
	"errors"
	"testing"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

type conflictRunRepo struct{ run *domain.Run }

func (r *conflictRunRepo) Save(context.Context, *domain.Run) error                 { return nil }
func (r *conflictRunRepo) Find(context.Context, int64, int64) (*domain.Run, error) { return r.run, nil }
func (r *conflictRunRepo) List(context.Context, int64, *int64, string, int) ([]*domain.Run, error) {
	return nil, nil
}
func (r *conflictRunRepo) SaveCaseResult(context.Context, *domain.CaseResult) error { return nil }
func (r *conflictRunRepo) CaseResults(context.Context, int64, int64) ([]*domain.CaseResult, error) {
	return nil, nil
}
func (r *conflictRunRepo) MaxAttempt(context.Context, int64, int64, int64) (int, error) {
	return 0, nil
}

type alwaysConflict struct{}

func (alwaysConflict) Acquire(context.Context, domain.ResourceClaim) (string, error) {
	return "", domain.ErrResourceConflict
}
func (alwaysConflict) Release(context.Context, string) error { return nil }

type recordedExecutionAudit struct{ event domain.ExecutionAuditEvent }

func (a *recordedExecutionAudit) Append(_ context.Context, event domain.ExecutionAuditEvent) error {
	a.event = event
	return nil
}

func TestExecuteRunAuditsResourceConflict(t *testing.T) {
	run := domain.NewRun(9, 100, 1, 1003, "test", "v1", "main", "manual", []int64{4001})
	run.State = domain.RunScheduled
	audit := &recordedExecutionAudit{}
	svc := NewRunService(&conflictRunRepo{run: run}, nil, nil, nil, nil, nil)
	svc.SetConflictPort(alwaysConflict{})
	svc.SetAuditPort(audit)

	_, _, err := svc.ExecuteRun(rls.WithTenant(context.Background(), 100), run.ID)
	if !errors.Is(err, ErrResourceConflict) {
		t.Fatalf("error=%v, want execution resource conflict", err)
	}
	if audit.event.Op != "execution.resource_conflict" || audit.event.Asset != "run" || audit.event.AssetID != run.ID {
		t.Fatalf("audit event=%+v", audit.event)
	}
	if audit.event.Payload["target_id"] != run.TargetID {
		t.Fatalf("payload=%v", audit.event.Payload)
	}
}
