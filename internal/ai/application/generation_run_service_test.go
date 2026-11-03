package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/ai/infra"
	"github.com/openware-io/open-green-pass/pkg/id"
)

func newTestRunService(t *testing.T) (*GenerationRunService, *infra.MemoryGenerationRunRepository, *GenerationBatchService, *infra.MemoryGenerationRunEventRepository, *id.Generator) {
	t.Helper()
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	runRepo := infra.NewMemoryGenerationRunRepository()
	batchRepo := infra.NewMemoryGenerationBatchRepository()
	batchSvc := NewGenerationBatchService(batchRepo)
	batchSvc.SetScopePort(testGenerationScopePort{repoID: 30})
	eventRepo := infra.NewMemoryGenerationRunEventRepository()
	svc := NewGenerationRunService(runRepo, batchSvc, gen)
	svc.SetEventPort(eventRepo)
	return svc, runRepo, batchSvc, eventRepo, gen
}

func testRunCommand(id int64, key string, serviceIDs ...int64) CreateGenerationRunCommand {
	units := make([]GenerationRunUnitInput, 0, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		units = append(units, GenerationRunUnitInput{ServiceID: serviceID, RepoID: 30, SourcePath: ".", RefType: "branch", RefName: "main"})
	}
	return CreateGenerationRunCommand{
		ID: id, TeamID: 10, ProjectID: 20, ScopeType: "project", ScopeID: 0,
		ActorID: 40, IdempotencyKey: key, Units: units,
	}
}

// advanceBatch drives the fake batch through the real transition graph.
func advanceBatch(t *testing.T, svc *GenerationBatchService, teamID, batchID int64, states ...domain.GenerationBatchState) {
	t.Helper()
	for _, next := range states {
		batch, err := svc.Find(context.Background(), teamID, batchID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Transition(context.Background(), teamID, batchID, batch.StateVersion, next); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
}

func eventTypes(t *testing.T, repo *infra.MemoryGenerationRunEventRepository, teamID, runID int64) []domain.GenerationRunEventType {
	t.Helper()
	events, err := repo.ListAfter(context.Background(), teamID, runID, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	types := make([]domain.GenerationRunEventType, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

func containsEventType(types []domain.GenerationRunEventType, want domain.GenerationRunEventType) bool {
	for _, eventType := range types {
		if eventType == want {
			return true
		}
	}
	return false
}

func TestGenerationRunCreateIsIdempotentAndWritesEvents(t *testing.T) {
	svc, runRepo, _, eventRepo, _ := newTestRunService(t)
	cmd := testRunCommand(100, "run-1", 50)
	first, created, err := svc.Create(context.Background(), cmd)
	if err != nil || !created {
		t.Fatalf("create = %#v, created=%v, err=%v", first, created, err)
	}
	if first.State != domain.GenerationRunRunning || len(first.Units) != 1 || first.Units[0].BatchID == nil {
		t.Fatalf("unexpected created run: %+v", first)
	}
	if types := eventTypes(t, eventRepo, 10, first.ID); !containsEventType(types, domain.GenerationRunEventRunCreated) {
		t.Fatalf("run.created event missing: %v", types)
	}
	cmd.ID = 101
	second, created, err := svc.Create(context.Background(), cmd)
	if err != nil || created {
		t.Fatalf("repeat create = %#v, created=%v, err=%v", second, created, err)
	}
	if second.ID != first.ID {
		t.Fatalf("repeat create returned a new run: %d != %d", second.ID, first.ID)
	}
	runs, err := runRepo.ListByProject(context.Background(), 10, 20, 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("idempotent create produced %d runs, err=%v", len(runs), err)
	}
}

func TestGenerationRunIdempotencyConflictOnDifferentUnits(t *testing.T) {
	svc, _, _, _, _ := newTestRunService(t)
	if _, _, err := svc.Create(context.Background(), testRunCommand(100, "run-1", 50)); err != nil {
		t.Fatal(err)
	}
	cmd := testRunCommand(101, "run-1", 51)
	if _, _, err := svc.Create(context.Background(), cmd); !errors.Is(err, domain.ErrGenerationRunIdempotencyConflict) {
		t.Fatalf("conflicting idempotency error = %v", err)
	}
}

func TestGenerationRunPlanBindingIsValidatedAndConsumed(t *testing.T) {
	svc, runRepo, _, _, _ := newTestRunService(t)
	now := time.Now().UTC()
	items := []domain.GenerationPlanItem{{ServiceID: 50, RepoID: 30, RefType: "branch", RefName: "main", CommitSHA: "abc123", Valid: true}}
	hash := domain.CanonicalGenerationPlanHash(20, "project", 0, items)
	planRepo := infra.NewMemoryGenerationPlanRepository()
	plan := &domain.GenerationPlan{
		ID: 900, TeamID: 10, ProjectID: 20, ScopeType: "project", ScopeID: 0, PlanHash: hash,
		State: domain.GenerationPlanDraft, Items: items, ExpiresAt: now.Add(domain.DefaultGenerationPlanTTL),
		CreatedBy: 40, CreatedAt: now, UpdatedAt: now,
	}
	if err := planRepo.Create(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	svc.SetPlanPort(planRepo)
	run, created, err := svc.Create(context.Background(), CreateGenerationRunCommand{
		// plan-bound create does not need to resend scope: the plan owns it.
		ID: 100, TeamID: 10, ProjectID: 20, ActorID: 40,
		IdempotencyKey: "plan-run", PlanID: 900, PlanHash: hash,
	})
	if err != nil || !created {
		t.Fatalf("plan-bound create = %#v, created=%v, err=%v", run, created, err)
	}
	if run.PlanID == nil || *run.PlanID != 900 {
		t.Fatalf("run plan id not persisted: %+v", run.PlanID)
	}
	stored, err := planRepo.Find(context.Background(), 10, 900)
	if err != nil || stored.State != domain.GenerationPlanConsumed || stored.ConfirmedRunID == nil || *stored.ConfirmedRunID != run.ID {
		t.Fatalf("plan not consumed: %#v, err=%v", stored, err)
	}
	if got, err := runRepo.Find(context.Background(), 10, run.ID); err != nil || got.Units[0].CommitSHA != "abc123" {
		t.Fatalf("unit ref snapshot not persisted: %#v, err=%v", got, err)
	}
}

func TestGenerationRunRejectsStalePlan(t *testing.T) {
	now := time.Now().UTC()
	baseItems := []domain.GenerationPlanItem{{ServiceID: 50, RepoID: 30, RefType: "branch", RefName: "main", Valid: true}}
	hash := domain.CanonicalGenerationPlanHash(20, "project", 0, baseItems)
	cases := []struct {
		name string
		plan *domain.GenerationPlan
		want error
	}{
		{name: "expired", plan: &domain.GenerationPlan{ID: 901, TeamID: 10, ProjectID: 20, ScopeType: "project", PlanHash: hash, State: domain.GenerationPlanDraft, Items: baseItems, ExpiresAt: now.Add(-time.Minute)}, want: domain.ErrGenerationPlanExpired},
		{name: "consumed", plan: &domain.GenerationPlan{ID: 902, TeamID: 10, ProjectID: 20, ScopeType: "project", PlanHash: hash, State: domain.GenerationPlanConsumed, Items: baseItems, ExpiresAt: now.Add(time.Minute)}, want: domain.ErrGenerationPlanStale},
		{name: "invalid", plan: &domain.GenerationPlan{ID: 903, TeamID: 10, ProjectID: 20, ScopeType: "project", PlanHash: hash, State: domain.GenerationPlanDraft, Items: []domain.GenerationPlanItem{{ServiceID: 50, RepoID: 30, Valid: false, Error: "ref_not_found: x"}}, ExpiresAt: now.Add(time.Minute)}, want: domain.ErrGenerationPlanInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, _, _ := newTestRunService(t)
			planRepo := infra.NewMemoryGenerationPlanRepository()
			if err := planRepo.Create(context.Background(), tc.plan); err != nil {
				t.Fatal(err)
			}
			svc.SetPlanPort(planRepo)
			_, _, err := svc.Create(context.Background(), CreateGenerationRunCommand{
				ID: 100, TeamID: 10, ProjectID: 20, ScopeType: "project", ScopeID: 0, ActorID: 40,
				IdempotencyKey: "plan-" + tc.name, PlanID: tc.plan.ID, PlanHash: hash,
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
	// Wrong hash must be indistinguishable from an expired plan to the client.
	svc, _, _, _, _ := newTestRunService(t)
	planRepo := infra.NewMemoryGenerationPlanRepository()
	valid := &domain.GenerationPlan{ID: 904, TeamID: 10, ProjectID: 20, ScopeType: "project", PlanHash: hash, State: domain.GenerationPlanDraft, Items: baseItems, ExpiresAt: now.Add(time.Minute)}
	if err := planRepo.Create(context.Background(), valid); err != nil {
		t.Fatal(err)
	}
	svc.SetPlanPort(planRepo)
	if _, _, err := svc.Create(context.Background(), CreateGenerationRunCommand{
		ID: 100, TeamID: 10, ProjectID: 20, ScopeType: "project", ScopeID: 0, ActorID: 40,
		IdempotencyKey: "plan-hash", PlanID: 904, PlanHash: "sha256:wrong",
	}); !errors.Is(err, domain.ErrGenerationPlanStale) {
		t.Fatalf("hash mismatch error = %v", err)
	}
}

func TestGenerationRunGetIsPureRead(t *testing.T) {
	svc, runRepo, _, _, _ := newTestRunService(t)
	run, _, err := svc.Create(context.Background(), testRunCommand(100, "run-1", 50))
	if err != nil {
		t.Fatal(err)
	}
	writesBefore := runRepo.WriteCount()
	got, err := svc.Get(context.Background(), 10, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if runRepo.WriteCount() != writesBefore {
		t.Fatalf("GET mutated the run repository: writes %d -> %d", writesBefore, runRepo.WriteCount())
	}
	if got.State != run.State || len(got.Units) != 1 {
		t.Fatalf("unexpected run read: %#v", got)
	}
}

//nolint:gocyclo // Scenario fixture intentionally covers the batch-review projection matrix.
func TestGenerationRunSyncMapsBatchReviewToWaitingReview(t *testing.T) {
	svc, _, batchSvc, eventRepo, _ := newTestRunService(t)
	run, _, err := svc.Create(context.Background(), testRunCommand(100, "run-1", 50))
	if err != nil {
		t.Fatal(err)
	}
	batchID := *run.Units[0].BatchID
	advanceBatch(t, batchSvc, 10, batchID,
		domain.GenerationFetching, domain.GenerationParsing, domain.GenerationGenerating,
		domain.GenerationQualityCheck, domain.GenerationReview)

	synced, err := svc.SyncRun(context.Background(), 10, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if synced.State != domain.GenerationRunWaitingReview || synced.WaitingReviewUnits != 1 {
		t.Fatalf("run state after review = %s waiting=%d", synced.State, synced.WaitingReviewUnits)
	}
	if synced.Units[0].State != domain.GenerationUnitWaitingReview || !synced.Units[0].ReviewRequired {
		t.Fatalf("unit projection = %+v", synced.Units[0])
	}
	types := eventTypes(t, eventRepo, 10, run.ID)
	if !containsEventType(types, domain.GenerationRunEventUnitReviewRequired) || !containsEventType(types, domain.GenerationRunEventRunWaitingReview) {
		t.Fatalf("review events missing: %v", types)
	}

	reviewed, err := svc.Review(context.Background(), ReviewGenerationRunCommand{TeamID: 10, RunID: run.ID, ActorID: 40, Decision: "approve", RequestID: "review-1"})
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.State != domain.GenerationRunRunning {
		t.Fatalf("run state after approve = %s", reviewed.State)
	}
	// Approval is only recorded once gen_batch actually reaches approved; the
	// review transition to committing must not pre-emit unit.approved.
	if got := countEventType(t, eventRepo, 10, run.ID, domain.GenerationRunEventUnitApproved); got != 0 {
		t.Fatalf("unit.approved emitted before the batch was approved: %d", got)
	}

	advanceBatch(t, batchSvc, 10, batchID, domain.GenerationApproved)
	final, err := svc.SyncRun(context.Background(), 10, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.State != domain.GenerationRunSucceeded || final.CompletedUnits != 1 || final.FinishedAt == nil {
		t.Fatalf("final run projection = %+v", final)
	}
	types = eventTypes(t, eventRepo, 10, run.ID)
	if !containsEventType(types, domain.GenerationRunEventRunCompleted) {
		t.Fatalf("run.completed event missing: %v", types)
	}
}

func TestGenerationRunRejectLeadsToFailedAggregate(t *testing.T) {
	svc, _, batchSvc, _, _ := newTestRunService(t)
	run, _, err := svc.Create(context.Background(), testRunCommand(100, "run-1", 50))
	if err != nil {
		t.Fatal(err)
	}
	batchID := *run.Units[0].BatchID
	advanceBatch(t, batchSvc, 10, batchID,
		domain.GenerationFetching, domain.GenerationParsing, domain.GenerationGenerating,
		domain.GenerationQualityCheck, domain.GenerationReview)
	if _, err := svc.SyncRun(context.Background(), 10, run.ID); err != nil {
		t.Fatal(err)
	}
	reviewed, err := svc.Review(context.Background(), ReviewGenerationRunCommand{TeamID: 10, RunID: run.ID, ActorID: 40, Decision: "reject", RequestID: "review-1"})
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.State != domain.GenerationRunFailed || reviewed.FailedUnits != 1 {
		t.Fatalf("rejected aggregate = %s failed=%d", reviewed.State, reviewed.FailedUnits)
	}
	if reviewed.Units[0].State != domain.GenerationUnitRejected {
		t.Fatalf("unit state after reject = %s", reviewed.Units[0].State)
	}
}

func TestGenerationRunCancelIsIdempotent(t *testing.T) {
	svc, _, batchSvc, eventRepo, _ := newTestRunService(t)
	run, _, err := svc.Create(context.Background(), testRunCommand(100, "run-1", 50))
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := svc.Cancel(context.Background(), 10, run.ID, 40, "cancel-1")
	if err != nil || cancelled.State != domain.GenerationRunCancelled || cancelled.Units[0].State != domain.GenerationUnitCancelled {
		t.Fatalf("cancel = %#v, err=%v", cancelled, err)
	}
	batch, err := batchSvc.Find(context.Background(), 10, *run.Units[0].BatchID)
	if err != nil || batch.State != domain.GenerationCancelled {
		t.Fatalf("batch not cancelled: %#v err=%v", batch, err)
	}
	again, err := svc.Cancel(context.Background(), 10, run.ID, 40, "cancel-2")
	if err != nil || again.State != domain.GenerationRunCancelled {
		t.Fatalf("idempotent cancel = %#v, err=%v", again, err)
	}
	if types := eventTypes(t, eventRepo, 10, run.ID); !containsEventType(types, domain.GenerationRunEventRunCancelled) {
		t.Fatalf("run.cancelled event missing: %v", types)
	}
}

func TestGenerationRunReconcilerConvergesNonTerminalRuns(t *testing.T) {
	svc, runRepo, batchSvc, _, _ := newTestRunService(t)
	run, _, err := svc.Create(context.Background(), testRunCommand(100, "run-1", 50))
	if err != nil {
		t.Fatal(err)
	}
	reconciler := NewGenerationRunReconciler(runRepo, svc, time.Second, nil)
	processed, err := reconciler.ReconcileOnce(context.Background())
	if err != nil || processed != 1 {
		t.Fatalf("reconcile pass = %d, err=%v", processed, err)
	}
	batchID := *run.Units[0].BatchID
	advanceBatch(t, batchSvc, 10, batchID,
		domain.GenerationFetching, domain.GenerationParsing, domain.GenerationGenerating,
		domain.GenerationQualityCheck, domain.GenerationReview)
	if _, err := reconciler.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	synced, err := svc.Get(context.Background(), 10, run.ID)
	if err != nil || synced.State != domain.GenerationRunWaitingReview {
		t.Fatalf("reconciled run = %#v, err=%v", synced, err)
	}
	advanceBatch(t, batchSvc, 10, batchID, domain.GenerationCommitting, domain.GenerationApproved)
	if _, err := reconciler.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	final, err := svc.Get(context.Background(), 10, run.ID)
	if err != nil || final.State != domain.GenerationRunSucceeded {
		t.Fatalf("reconciled final = %#v, err=%v", final, err)
	}
	// A terminal run is no longer a reconcile candidate.
	if processed, err := reconciler.ReconcileOnce(context.Background()); err != nil || processed != 0 {
		t.Fatalf("terminal run still reconciled: processed=%d err=%v", processed, err)
	}
}

func newDraftPlan(t *testing.T, planRepo *infra.MemoryGenerationPlanRepository, planID int64, expiresAt time.Time) (*domain.GenerationPlan, string) {
	t.Helper()
	now := time.Now().UTC()
	items := []domain.GenerationPlanItem{{ServiceID: 50, RepoID: 30, RefType: "branch", RefName: "main", CommitSHA: "abc123", Valid: true}}
	hash := domain.CanonicalGenerationPlanHash(20, "project", 0, items)
	plan := &domain.GenerationPlan{
		ID: planID, TeamID: 10, ProjectID: 20, ScopeType: "project", ScopeID: 0, PlanHash: hash,
		State: domain.GenerationPlanDraft, Items: items, ExpiresAt: expiresAt, CreatedBy: 40, CreatedAt: now, UpdatedAt: now,
	}
	if err := planRepo.Create(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	return plan, hash
}

func TestGenerationRunPlanConsumeIsAtomicUnderConcurrency(t *testing.T) {
	svc, runRepo, _, _, _ := newTestRunService(t)
	planRepo := infra.NewMemoryGenerationPlanRepository()
	_, hash := newDraftPlan(t, planRepo, 950, time.Now().UTC().Add(domain.DefaultGenerationPlanTTL))
	svc.SetPlanPort(planRepo)
	const requests = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	createdRuns, staleErrors, otherErrors := 0, 0, 0
	for index := 0; index < requests; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, created, err := svc.Create(context.Background(), CreateGenerationRunCommand{
				ID: int64(5000 + index), TeamID: 10, ProjectID: 20, ActorID: 40,
				IdempotencyKey: fmt.Sprintf("plan-race-%d", index), PlanID: 950, PlanHash: hash,
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && created:
				createdRuns++
			case errors.Is(err, domain.ErrGenerationPlanStale):
				staleErrors++
			default:
				otherErrors++
				t.Errorf("unexpected create error: %v", err)
			}
		}(index)
	}
	wg.Wait()
	if createdRuns != 1 || staleErrors != requests-1 || otherErrors != 0 {
		t.Fatalf("created=%d stale=%d other=%d", createdRuns, staleErrors, otherErrors)
	}
	runs, err := runRepo.ListByProject(context.Background(), 10, 20, 100)
	if err != nil || len(runs) != 1 {
		t.Fatalf("concurrent plan consumption created %d runs, err=%v", len(runs), err)
	}
	if runs[0].Units[0].BatchID == nil {
		t.Fatal("winning plan run has no batch")
	}
}

func TestGenerationRunCannotReuseConsumedPlanWithAnotherKey(t *testing.T) {
	svc, _, _, _, _ := newTestRunService(t)
	planRepo := infra.NewMemoryGenerationPlanRepository()
	_, hash := newDraftPlan(t, planRepo, 960, time.Now().UTC().Add(domain.DefaultGenerationPlanTTL))
	svc.SetPlanPort(planRepo)
	if _, created, err := svc.Create(context.Background(), CreateGenerationRunCommand{
		ID: 6000, TeamID: 10, ProjectID: 20, ActorID: 40, IdempotencyKey: "plan-first", PlanID: 960, PlanHash: hash,
	}); err != nil || !created {
		t.Fatalf("first create = %v, created=%v", err, created)
	}
	if _, _, err := svc.Create(context.Background(), CreateGenerationRunCommand{
		ID: 6001, TeamID: 10, ProjectID: 20, ActorID: 40, IdempotencyKey: "plan-second", PlanID: 960, PlanHash: hash,
	}); !errors.Is(err, domain.ErrGenerationPlanStale) {
		t.Fatalf("reused plan error = %v", err)
	}
}

func TestGenerationRunRejectsUnitsOutsideScope(t *testing.T) {
	cases := []struct {
		name      string
		scopeType string
		scopeID   int64
		unit      GenerationRunUnitInput
	}{
		{name: "repo", scopeType: "repo", scopeID: 31, unit: GenerationRunUnitInput{ServiceID: 50, RepoID: 30, SourcePath: ".", RefType: "branch", RefName: "main"}},
		{name: "service", scopeType: "service", scopeID: 51, unit: GenerationRunUnitInput{ServiceID: 50, RepoID: 30, SourcePath: ".", RefType: "branch", RefName: "main"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, runRepo, _, _, _ := newTestRunService(t)
			_, _, err := svc.Create(context.Background(), CreateGenerationRunCommand{
				ID: 7000, TeamID: 10, ProjectID: 20, ScopeType: tc.scopeType, ScopeID: tc.scopeID,
				ActorID: 40, IdempotencyKey: "scope-mismatch", Units: []GenerationRunUnitInput{tc.unit},
			})
			if err == nil {
				t.Fatal("scope mismatch accepted")
			}
			runs, listErr := runRepo.ListByProject(context.Background(), 10, 20, 10)
			if listErr != nil || len(runs) != 0 {
				t.Fatalf("scope mismatch persisted %d runs, err=%v", len(runs), listErr)
			}
		})
	}
}

// failingCancelBatchRepo injects one batch-cancel failure to prove the run is
// not reported as cancelled when a batch keeps running.
type failingCancelBatchRepo struct {
	*infra.MemoryGenerationBatchRepository
	failID int64
}

func (r *failingCancelBatchRepo) CompareAndSwap(ctx context.Context, batch *domain.GenerationBatch, expected int64) error {
	if batch != nil && batch.State == domain.GenerationCancelled && batch.ID == r.failID {
		return errors.New("injected batch cancel failure")
	}
	return r.MemoryGenerationBatchRepository.CompareAndSwap(ctx, batch, expected)
}

func TestGenerationRunCancelFailureDoesNotReportCancelled(t *testing.T) {
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	runRepo := infra.NewMemoryGenerationRunRepository()
	batchRepo := &failingCancelBatchRepo{MemoryGenerationBatchRepository: infra.NewMemoryGenerationBatchRepository()}
	batchSvc := NewGenerationBatchService(batchRepo)
	batchSvc.SetScopePort(testGenerationScopePort{repoID: 30})
	eventRepo := infra.NewMemoryGenerationRunEventRepository()
	svc := NewGenerationRunService(runRepo, batchSvc, gen)
	svc.SetEventPort(eventRepo)
	run, _, err := svc.Create(context.Background(), testRunCommand(8000, "cancel-fail", 50, 51))
	if err != nil {
		t.Fatal(err)
	}
	batchRepo.failID = *run.Units[1].BatchID
	cancelled, err := svc.Cancel(context.Background(), 10, run.ID, 40, "cancel-1")
	if !errors.Is(err, ErrGenerationRunCancelIncomplete) {
		t.Fatalf("cancel error = %v", err)
	}
	if cancelled.State == domain.GenerationRunCancelled {
		t.Fatalf("run reported cancelled despite a live batch: %+v", cancelled)
	}
	if cancelled.State != domain.GenerationRunPartialFailed {
		t.Fatalf("run state = %s, want partial_failed", cancelled.State)
	}
	if cancelled.CancelledAt != nil {
		t.Fatal("cancelled_at must stay nil for an incomplete cancel")
	}
	first, err := batchSvc.Find(context.Background(), 10, *run.Units[0].BatchID)
	if err != nil || first.State != domain.GenerationCancelled {
		t.Fatalf("first batch = %#v, err=%v", first, err)
	}
	second, err := batchSvc.Find(context.Background(), 10, *run.Units[1].BatchID)
	if err != nil || second.State == domain.GenerationCancelled {
		t.Fatalf("failed batch must not be cancelled: %#v, err=%v", second, err)
	}
	if cancelled.Units[1].State != domain.GenerationUnitFailed {
		t.Fatalf("failed unit state = %s", cancelled.Units[1].State)
	}
	types := eventTypes(t, eventRepo, 10, run.ID)
	if !containsEventType(types, domain.GenerationRunEventUnitFailed) || !containsEventType(types, domain.GenerationRunEventRunPartialFailed) {
		t.Fatalf("cancel failure events missing: %v", types)
	}
}

func TestGenerationRunSyncRecoversMissingBatch(t *testing.T) {
	svc, runRepo, batchSvc, eventRepo, gen := newTestRunService(t)
	now := time.Now().UTC()
	runID := int64(8100)
	run := &domain.GenerationRun{
		ID: runID, TeamID: 10, ProjectID: 20, ScopeType: "project", State: domain.GenerationRunQueued,
		TotalUnits: 1, IdempotencyKey: "crash-key", CreatedBy: 40, CreatedAt: now, UpdatedAt: now,
		Units: []*domain.GenerationUnit{{
			ID: gen.Next(), TeamID: 10, RunID: runID, ServiceID: 50, RepoID: 30,
			RefType: "branch", RefName: "main", State: domain.GenerationUnitQueued,
		}},
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	synced, err := svc.SyncRun(context.Background(), 10, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.Units) != 1 || synced.Units[0].BatchID == nil || synced.Units[0].State != domain.GenerationUnitRunning {
		t.Fatalf("missing batch not recovered: %+v", synced.Units[0])
	}
	if synced.State != domain.GenerationRunRunning {
		t.Fatalf("run state after recovery = %s", synced.State)
	}
	if _, err := batchSvc.Find(context.Background(), 10, *synced.Units[0].BatchID); err != nil {
		t.Fatalf("recovered batch not found: %v", err)
	}
	if types := eventTypes(t, eventRepo, 10, runID); !containsEventType(types, domain.GenerationRunEventUnitQueued) {
		t.Fatalf("recovery event missing: %v", types)
	}
}

func TestGenerationRunSyncFailsUnrecoverableMissingBatch(t *testing.T) {
	svc, runRepo, _, eventRepo, gen := newTestRunService(t)
	now := time.Now().UTC()
	runID := int64(8200)
	run := &domain.GenerationRun{
		ID: runID, TeamID: 10, ProjectID: 20, ScopeType: "project", State: domain.GenerationRunQueued,
		TotalUnits: 1, IdempotencyKey: "crash-bad", CreatedBy: 40, CreatedAt: now, UpdatedAt: now,
		Units: []*domain.GenerationUnit{{
			// Repo 31 does not match the fake scope reader's repo 30, so the
			// catalog rejects the reconstruction.
			ID: gen.Next(), TeamID: 10, RunID: runID, ServiceID: 50, RepoID: 31,
			RefType: "branch", RefName: "main", State: domain.GenerationUnitQueued,
		}},
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	synced, err := svc.SyncRun(context.Background(), 10, runID)
	if err != nil {
		t.Fatal(err)
	}
	if synced.Units[0].State != domain.GenerationUnitFailed || !strings.Contains(synced.Units[0].Error, "batch_not_created") {
		t.Fatalf("unit projection = %+v", synced.Units[0])
	}
	if synced.State != domain.GenerationRunFailed {
		t.Fatalf("run state = %s", synced.State)
	}
	if types := eventTypes(t, eventRepo, 10, runID); !containsEventType(types, domain.GenerationRunEventUnitFailed) || !containsEventType(types, domain.GenerationRunEventRunFailed) {
		t.Fatalf("unrecoverable batch events missing: %v", types)
	}
	processed, err := NewGenerationRunReconciler(runRepo, svc, time.Second, nil).ReconcileOnce(context.Background())
	if err != nil || processed != 0 {
		t.Fatalf("failed run still a reconcile candidate: %d, %v", processed, err)
	}
}

func countEventType(t *testing.T, repo *infra.MemoryGenerationRunEventRepository, teamID, runID int64, want domain.GenerationRunEventType) int {
	t.Helper()
	count := 0
	for _, eventType := range eventTypes(t, repo, teamID, runID) {
		if eventType == want {
			count++
		}
	}
	return count
}

func TestGenerationRunCancelProjectsCancelledStage(t *testing.T) {
	svc, _, batchSvc, eventRepo, _ := newTestRunService(t)
	run, _, err := svc.Create(context.Background(), testRunCommand(8300, "cancel-projection", 50))
	if err != nil {
		t.Fatal(err)
	}
	batchID := *run.Units[0].BatchID
	advanceBatch(t, batchSvc, 10, batchID, domain.GenerationFetching)
	if _, err := svc.Cancel(context.Background(), 10, run.ID, 40, "cancel-projection"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	after, err := svc.Get(context.Background(), 10, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.State != domain.GenerationRunCancelled {
		t.Fatalf("run state = %s", after.State)
	}
	unit := after.Units[0]
	if unit.State != domain.GenerationUnitCancelled || unit.BatchState != "cancelled" || unit.Stage != "cancelled" || unit.ReviewRequired {
		t.Fatalf("cancelled unit projection stale: %+v", unit)
	}
	if types := eventTypes(t, eventRepo, 10, run.ID); !containsEventType(types, domain.GenerationRunEventUnitCancelled) {
		t.Fatalf("unit.cancelled event missing: %v", types)
	}
}

func TestGenerationRunCancelRepairsTerminalBatchProjection(t *testing.T) {
	svc, _, batchSvc, _, _ := newTestRunService(t)
	run, _, err := svc.Create(context.Background(), testRunCommand(8400, "cancel-terminal", 50))
	if err != nil {
		t.Fatal(err)
	}
	batchID := *run.Units[0].BatchID
	advanceBatch(t, batchSvc, 10, batchID,
		domain.GenerationFetching, domain.GenerationParsing, domain.GenerationGenerating,
		domain.GenerationQualityCheck, domain.GenerationReview, domain.GenerationCommitting, domain.GenerationApproved)
	after, err := svc.Cancel(context.Background(), 10, run.ID, 40, "cancel-terminal")
	if err != nil {
		t.Fatal(err)
	}
	unit := after.Units[0]
	if unit.State != domain.GenerationUnitSucceeded || unit.BatchState != "approved" || unit.Stage != "approved" {
		t.Fatalf("approved terminal batch projection = %+v", unit)
	}
}

func TestGenerationRunSameKeyPlanConcurrentRetryReturnsExistingRun(t *testing.T) {
	svc, runRepo, _, _, _ := newTestRunService(t)
	planRepo := infra.NewMemoryGenerationPlanRepository()
	_, hash := newDraftPlan(t, planRepo, 970, time.Now().UTC().Add(domain.DefaultGenerationPlanTTL))
	svc.SetPlanPort(planRepo)
	const requests = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	createdCount, existingCount := 0, 0
	runIDs := map[int64]struct{}{}
	errorsSeen := make([]error, 0)
	for index := 0; index < requests; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			run, created, err := svc.Create(context.Background(), CreateGenerationRunCommand{
				ID: int64(9000 + index), TeamID: 10, ProjectID: 20, ActorID: 40,
				IdempotencyKey: "same-key-plan-race", PlanID: 970, PlanHash: hash,
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errorsSeen = append(errorsSeen, err)
				return
			}
			if created {
				createdCount++
			} else {
				existingCount++
			}
			runIDs[run.ID] = struct{}{}
		}(index)
	}
	wg.Wait()
	if len(errorsSeen) != 0 {
		t.Fatalf("same-key concurrent create errors: %v", errorsSeen)
	}
	if createdCount != 1 || existingCount != requests-1 || len(runIDs) != 1 {
		t.Fatalf("created=%d existing=%d distinctRuns=%d", createdCount, existingCount, len(runIDs))
	}
	runs, err := runRepo.ListByProject(context.Background(), 10, 20, 100)
	if err != nil || len(runs) != 1 {
		t.Fatalf("same-key concurrent create produced %d runs, err=%v", len(runs), err)
	}
}

func TestGenerationRunReviewApprovedEventEmittedOnce(t *testing.T) {
	svc, runRepo, batchSvc, eventRepo, _ := newTestRunService(t)
	run, _, err := svc.Create(context.Background(), testRunCommand(8500, "review-event", 50))
	if err != nil {
		t.Fatal(err)
	}
	batchID := *run.Units[0].BatchID
	advanceBatch(t, batchSvc, 10, batchID,
		domain.GenerationFetching, domain.GenerationParsing, domain.GenerationGenerating,
		domain.GenerationQualityCheck, domain.GenerationReview)
	if _, err := svc.SyncRun(context.Background(), 10, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Review(context.Background(), ReviewGenerationRunCommand{TeamID: 10, RunID: run.ID, ActorID: 40, Decision: "approve", RequestID: "review-event"}); err != nil {
		t.Fatal(err)
	}
	if got := countEventType(t, eventRepo, 10, run.ID, domain.GenerationRunEventUnitApproved); got != 0 {
		t.Fatalf("unit.approved emitted before batch approved: %d", got)
	}
	advanceBatch(t, batchSvc, 10, batchID, domain.GenerationApproved)
	if _, err := svc.SyncRun(context.Background(), 10, run.ID); err != nil {
		t.Fatal(err)
	}
	if got := countEventType(t, eventRepo, 10, run.ID, domain.GenerationRunEventUnitApproved); got != 1 {
		t.Fatalf("unit.approved events = %d, want exactly 1", got)
	}
	// Terminal unit events are event_key idempotent: a stale projection that
	// makes SyncRun re-emit must not duplicate the approved event.
	unitID := run.Units[0].ID
	if err := runRepo.UpdateUnit(context.Background(), 10, run.ID, unitID, domain.GenerationRunUnitUpdate{
		State: domain.GenerationUnitSucceeded, BatchState: "committing", Stage: "committing",
		SyncedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SyncRun(context.Background(), 10, run.ID); err != nil {
		t.Fatal(err)
	}
	if got := countEventType(t, eventRepo, 10, run.ID, domain.GenerationRunEventUnitApproved); got != 1 {
		t.Fatalf("unit.approved duplicated after re-sync: %d", got)
	}
}

func TestGenerationRunReviewRejectedEventEmittedOnce(t *testing.T) {
	svc, _, batchSvc, eventRepo, _ := newTestRunService(t)
	run, _, err := svc.Create(context.Background(), testRunCommand(8600, "review-reject-event", 50))
	if err != nil {
		t.Fatal(err)
	}
	batchID := *run.Units[0].BatchID
	advanceBatch(t, batchSvc, 10, batchID,
		domain.GenerationFetching, domain.GenerationParsing, domain.GenerationGenerating,
		domain.GenerationQualityCheck, domain.GenerationReview)
	if _, err := svc.SyncRun(context.Background(), 10, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Review(context.Background(), ReviewGenerationRunCommand{TeamID: 10, RunID: run.ID, ActorID: 40, Decision: "reject", RequestID: "review-reject-event"}); err != nil {
		t.Fatal(err)
	}
	if got := countEventType(t, eventRepo, 10, run.ID, domain.GenerationRunEventUnitRejected); got != 1 {
		t.Fatalf("unit.rejected events = %d, want exactly 1", got)
	}
}
