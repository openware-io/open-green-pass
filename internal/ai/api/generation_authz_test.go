package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/ai/infra"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type testGenerationAuthorizer struct {
	allowed  bool
	action   string
	resource string
	calls    int
}

func (a *testGenerationAuthorizer) Authorize(_ context.Context, _ iam.Principal, action, resource string) error {
	a.calls++
	a.action, a.resource = action, resource
	if a.allowed {
		return nil
	}
	return iam.ErrForbidden
}

func generationRequest(t *testing.T, mux *http.ServeMux, method, path, body string, team int64, withPrincipal bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if team > 0 {
		req = req.WithContext(rls.WithTenant(req.Context(), team))
	}
	if withPrincipal {
		principal := iam.Principal{Subject: "40", TenantID: "10"}
		req = req.WithContext(iam.WithPrincipal(req.Context(), principal))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// fakePlanService lets transport tests exercise authorization and the response
// contract without a database.
type fakePlanService struct {
	plan  *domain.GenerationPlan
	err   error
	calls int
	found int64
}

func (f *fakePlanService) Preflight(context.Context, application.PreflightGenerationPlanCommand) (*domain.GenerationPlan, error) {
	f.calls++
	return f.plan, f.err
}

func (f *fakePlanService) Find(_ context.Context, _, planID int64) (*domain.GenerationPlan, error) {
	f.found = planID
	return f.plan, f.err
}

func TestGenerationPlansRequireIAM(t *testing.T) {
	authorizer := &testGenerationAuthorizer{allowed: true}
	svc := &fakePlanService{plan: &domain.GenerationPlan{ID: 7, TeamID: 10, ProjectID: 20, PlanHash: "sha256:x", ExpiresAt: time.Now().UTC()}}
	mux := http.NewServeMux()
	RegisterGenerationPlans(mux, svc, authorizer)
	body := `{"project_id":20,"scope_type":"project","scope_id":0,"units":[]}`

	if rec := generationRequest(t, mux, http.MethodPost, "/generation/plans", body, 10, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous plan status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if svc.calls != 0 || authorizer.calls != 0 {
		t.Fatalf("anonymous plan reached service/authorizer")
	}

	authorizer.allowed = false
	if rec := generationRequest(t, mux, http.MethodPost, "/generation/plans", body, 10, true); rec.Code != http.StatusForbidden {
		t.Fatalf("denied plan status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if svc.calls != 0 || authorizer.action != "edit" || authorizer.resource != "20" {
		t.Fatalf("denied plan reached service or used action=%q resource=%q", authorizer.action, authorizer.resource)
	}

	authorizer.allowed = true
	rec := generationRequest(t, mux, http.MethodPost, "/generation/plans", body, 10, true)
	if rec.Code != http.StatusOK || svc.calls != 1 {
		t.Fatalf("allowed plan status = %d calls=%d body=%s", rec.Code, svc.calls, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"plan_id", "plan_hash", "expires_at", "units", "invalid", "summary", "estimate", "risks"} {
		if _, ok := payload[field]; !ok {
			t.Fatalf("plan response missing %q: %s", field, rec.Body.String())
		}
	}
	if authorizer.action != "edit" {
		t.Fatalf("plan create action = %q", authorizer.action)
	}
}

func TestGenerationPlansReadUsesViewAction(t *testing.T) {
	authorizer := &testGenerationAuthorizer{allowed: true}
	svc := &fakePlanService{plan: &domain.GenerationPlan{ID: 7, TeamID: 10, ProjectID: 20, ExpiresAt: time.Now().UTC()}}
	mux := http.NewServeMux()
	RegisterGenerationPlans(mux, svc, authorizer)
	rec := generationRequest(t, mux, http.MethodGet, "/generation/plans/7", "", 10, true)
	if rec.Code != http.StatusOK || svc.found != 7 {
		t.Fatalf("plan get status = %d body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.action != "view" || authorizer.resource != "20" {
		t.Fatalf("plan get authorization = (%q,%q)", authorizer.action, authorizer.resource)
	}
}

type apiTestScopePort struct{ repoID int64 }

func (p apiTestScopePort) ReadGenerationScope(_ context.Context, teamID, targetID, serviceID int64) (*domain.GenerationScope, error) {
	return &domain.GenerationScope{
		ServiceID: serviceID, TeamID: teamID, TargetID: targetID, RepoID: p.repoID, SourcePath: ".",
		Status: "active", GenerationStatus: "confirmed", ProfileStatus: "confirmed", Language: "Go", Framework: "net/http",
	}, nil
}

func newRunAPI(t *testing.T, authorizer iam.AuthorizationPort) (*http.ServeMux, *infra.MemoryGenerationRunRepository, *infra.MemoryGenerationRunEventRepository, *infra.MemoryGenerationPlanRepository) {
	t.Helper()
	gen, err := id.New(3, nil)
	if err != nil {
		t.Fatal(err)
	}
	runRepo := infra.NewMemoryGenerationRunRepository()
	batchSvc := application.NewGenerationBatchService(infra.NewMemoryGenerationBatchRepository())
	batchSvc.SetScopePort(apiTestScopePort{repoID: 30})
	eventRepo := infra.NewMemoryGenerationRunEventRepository()
	planRepo := infra.NewMemoryGenerationPlanRepository()
	runSvc := application.NewGenerationRunService(runRepo, batchSvc, gen)
	runSvc.SetEventPort(eventRepo)
	runSvc.SetPlanPort(planRepo)
	mux := http.NewServeMux()
	RegisterGenerationRunsAuthorized(mux, runSvc, gen, eventRepo, authorizer)
	return mux, runRepo, eventRepo, planRepo
}

const runBody = `{"project_id":20,"scope_type":"project","scope_id":0,"idempotency_key":"ui-run-1","units":[{"service_id":50,"repo_id":30,"source_path":".","ref_type":"branch","ref_name":"main"}]}`

func TestGenerationRunsRequireIAM(t *testing.T) {
	authorizer := &testGenerationAuthorizer{allowed: true}
	mux, runRepo, _, _ := newRunAPI(t, authorizer)
	if rec := generationRequest(t, mux, http.MethodPost, "/generation/runs", runBody, 10, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous create status = %d body=%s", rec.Code, rec.Body.String())
	}
	authorizer.allowed = false
	rec := generationRequest(t, mux, http.MethodPost, "/generation/runs", runBody, 10, true)
	if rec.Code != http.StatusForbidden || authorizer.action != "edit" || authorizer.resource != "20" {
		t.Fatalf("denied create status = %d action=%q resource=%q body=%s", rec.Code, authorizer.action, authorizer.resource, rec.Body.String())
	}
	runs, err := runRepo.ListByProject(context.Background(), 10, 20, 10)
	if err != nil || len(runs) != 0 {
		t.Fatalf("denied create persisted a run: %d, %v", len(runs), err)
	}
}

func TestGenerationRunsCreateIdempotencyAndPlanStale(t *testing.T) {
	mux, _, _, _ := newRunAPI(t, &testGenerationAuthorizer{allowed: true})
	first := generationRequest(t, mux, http.MethodPost, "/generation/runs", runBody, 10, true)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first create status = %d body=%s", first.Code, first.Body.String())
	}
	repeat := generationRequest(t, mux, http.MethodPost, "/generation/runs", runBody, 10, true)
	if repeat.Code != http.StatusOK {
		t.Fatalf("idempotent create status = %d body=%s", repeat.Code, repeat.Body.String())
	}
	conflictBody := `{"project_id":20,"scope_type":"project","scope_id":0,"idempotency_key":"ui-run-1","units":[{"service_id":51,"repo_id":30,"source_path":".","ref_type":"branch","ref_name":"main"}]}`
	conflict := generationRequest(t, mux, http.MethodPost, "/generation/runs", conflictBody, 10, true)
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "idempotency_conflict") {
		t.Fatalf("idempotency conflict status = %d body=%s", conflict.Code, conflict.Body.String())
	}
	staleBody := `{"project_id":20,"scope_type":"project","scope_id":0,"idempotency_key":"ui-run-2","plan_id":123,"plan_hash":"sha256:missing","units":[]}`
	stale := generationRequest(t, mux, http.MethodPost, "/generation/runs", staleBody, 10, true)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "plan_stale") {
		t.Fatalf("stale plan status = %d body=%s", stale.Code, stale.Body.String())
	}
}

func TestGenerationRunGetIsPureReadAndAuthorized(t *testing.T) {
	authorizer := &testGenerationAuthorizer{allowed: true}
	mux, runRepo, _, _ := newRunAPI(t, authorizer)
	create := generationRequest(t, mux, http.MethodPost, "/generation/runs", runBody, 10, true)
	if create.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	runID := int64(created["id"].(float64))
	writes := runRepo.WriteCount()
	rec := generationRequest(t, mux, http.MethodGet, "/generation/runs/"+strconv.FormatInt(runID, 10), "", 10, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", rec.Code, rec.Body.String())
	}
	if runRepo.WriteCount() != writes {
		t.Fatalf("GET mutated the run repository: %d -> %d", writes, runRepo.WriteCount())
	}
	var detail map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"id", "team_id", "project_id", "scope_type", "scope_id", "plan_id", "state", "total_units", "completed_units", "failed_units", "review_units", "progress_percent", "tokens_in", "tokens_out", "cost_amount", "created_at", "updated_at", "units"} {
		if _, ok := detail[field]; !ok {
			t.Fatalf("run detail missing %q: %s", field, rec.Body.String())
		}
	}
	units := detail["units"].([]any)
	unit := units[0].(map[string]any)
	for _, field := range []string{"id", "service_id", "service_name", "repo_id", "repo_url", "ref_type", "ref_name", "commit_sha", "batch_id", "batch_state", "stage", "review_required", "state", "error"} {
		if _, ok := unit[field]; !ok {
			t.Fatalf("run unit missing %q: %s", field, rec.Body.String())
		}
	}
	if authorizer.action != "view" || authorizer.resource != "20" {
		t.Fatalf("get authorization = (%q,%q)", authorizer.action, authorizer.resource)
	}
	authorizer.allowed = false
	if rec := generationRequest(t, mux, http.MethodGet, "/generation/runs/"+strconv.FormatInt(runID, 10), "", 10, true); rec.Code != http.StatusForbidden {
		t.Fatalf("denied get status = %d", rec.Code)
	}
}

func TestGenerationRunEventsStreamSnapshotsAndReplays(t *testing.T) {
	mux, runRepo, eventRepo, _ := newRunAPI(t, &testGenerationAuthorizer{allowed: true})
	run := &domain.GenerationRun{
		ID: 800, TeamID: 10, ProjectID: 20, ScopeType: "project", State: domain.GenerationRunSucceeded,
		TotalUnits: 1, CompletedUnits: 1, IdempotencyKey: "seed", CreatedBy: 40,
		Units: []*domain.GenerationUnit{{ID: 801, TeamID: 10, RunID: 800, ServiceID: 50, RepoID: 30, State: domain.GenerationUnitSucceeded}},
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []domain.GenerationRunEventType{domain.GenerationRunEventRunCreated, domain.GenerationRunEventRunCompleted} {
		event := &domain.GenerationRunEvent{ID: 1, TeamID: 10, RunID: 800, Type: eventType, Payload: map[string]any{"state": "x"}}
		if err := eventRepo.Append(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	rec := generationRequest(t, mux, http.MethodGet, "/generation/runs/800/events?after_seq=1", "", 10, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("events status = %d body=%s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("events content type = %q", contentType)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: run") || !strings.Contains(body, `"type":"run.completed"`) {
		t.Fatalf("events body missing replayed frame: %s", body)
	}
	if strings.Contains(body, "id: 1\n") {
		t.Fatalf("after_seq did not suppress replayed event: %s", body)
	}
	// Requesting a non-existent cursor on a terminal run returns the snapshot.
	rec = generationRequest(t, mux, http.MethodGet, "/generation/runs/800/events", "", 10, true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"type":"run.created"`) {
		t.Fatalf("snapshot/replay body = %s", rec.Body.String())
	}
}

func TestGenerationRunPlanBoundCreateNeedsNoScope(t *testing.T) {
	mux, _, _, planRepo := newRunAPI(t, &testGenerationAuthorizer{allowed: true})
	now := time.Now().UTC()
	items := []domain.GenerationPlanItem{{ServiceID: 50, RepoID: 30, RefType: "branch", RefName: "main", Valid: true}}
	hash := domain.CanonicalGenerationPlanHash(20, "project", 0, items)
	plan := &domain.GenerationPlan{
		ID: 990, TeamID: 10, ProjectID: 20, ScopeType: "project", ScopeID: 0, PlanHash: hash,
		State: domain.GenerationPlanDraft, Items: items, ExpiresAt: now.Add(domain.DefaultGenerationPlanTTL),
		CreatedBy: 40, CreatedAt: now, UpdatedAt: now,
	}
	if err := planRepo.Create(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"project_id":20,"idempotency_key":"plan-api","plan_id":990,"plan_hash":%q}`, hash)
	rec := generationRequest(t, mux, http.MethodPost, "/generation/runs", body, 10, true)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("plan-bound create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var detail map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["scope_type"] != "project" {
		t.Fatalf("plan-bound create did not take plan scope: %v", detail["scope_type"])
	}
	if planID, ok := detail["plan_id"].(float64); !ok || int64(planID) != 990 {
		t.Fatalf("plan_id = %v", detail["plan_id"])
	}
	repeat := generationRequest(t, mux, http.MethodPost, "/generation/runs", body, 10, true)
	if repeat.Code != http.StatusOK {
		t.Fatalf("idempotent plan-bound create status = %d body=%s", repeat.Code, repeat.Body.String())
	}
}

func TestGenerationRunCreateRejectsUnitsOutsideScope(t *testing.T) {
	mux, runRepo, _, _ := newRunAPI(t, &testGenerationAuthorizer{allowed: true})
	body := `{"project_id":20,"scope_type":"repo","scope_id":31,"idempotency_key":"scope-bad","units":[{"service_id":50,"repo_id":30,"source_path":".","ref_type":"branch","ref_name":"main"}]}`
	rec := generationRequest(t, mux, http.MethodPost, "/generation/runs", body, 10, true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("scope mismatch status = %d body=%s", rec.Code, rec.Body.String())
	}
	runs, err := runRepo.ListByProject(context.Background(), 10, 20, 10)
	if err != nil || len(runs) != 0 {
		t.Fatalf("scope mismatch persisted %d runs, err=%v", len(runs), err)
	}
}

// apiFailingCancelBatchRepo injects a batch cancel failure at the HTTP layer.
type apiFailingCancelBatchRepo struct {
	*infra.MemoryGenerationBatchRepository
	failID int64
}

func (r *apiFailingCancelBatchRepo) CompareAndSwap(ctx context.Context, batch *domain.GenerationBatch, expected int64) error {
	if batch != nil && batch.State == domain.GenerationCancelled && batch.ID == r.failID {
		return errors.New("injected batch cancel failure")
	}
	return r.MemoryGenerationBatchRepository.CompareAndSwap(ctx, batch, expected)
}

func TestGenerationRunCancelIncompleteReturns409(t *testing.T) {
	gen, err := id.New(4, nil)
	if err != nil {
		t.Fatal(err)
	}
	runRepo := infra.NewMemoryGenerationRunRepository()
	batchRepo := &apiFailingCancelBatchRepo{MemoryGenerationBatchRepository: infra.NewMemoryGenerationBatchRepository()}
	batchSvc := application.NewGenerationBatchService(batchRepo)
	batchSvc.SetScopePort(apiTestScopePort{repoID: 30})
	eventRepo := infra.NewMemoryGenerationRunEventRepository()
	runSvc := application.NewGenerationRunService(runRepo, batchSvc, gen)
	runSvc.SetEventPort(eventRepo)
	runSvc.SetPlanPort(infra.NewMemoryGenerationPlanRepository())
	mux := http.NewServeMux()
	RegisterGenerationRunsAuthorized(mux, runSvc, gen, eventRepo, &testGenerationAuthorizer{allowed: true})
	body := `{"project_id":20,"scope_type":"project","scope_id":0,"idempotency_key":"cancel-two","units":[{"service_id":50,"repo_id":30,"source_path":".","ref_type":"branch","ref_name":"main"},{"service_id":51,"repo_id":30,"source_path":".","ref_type":"branch","ref_name":"main"}]}`
	create := generationRequest(t, mux, http.MethodPost, "/generation/runs", body, 10, true)
	if create.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var detail map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	runID := int64(detail["id"].(float64))
	units := detail["units"].([]any)
	secondBatch := int64(units[1].(map[string]any)["batch_id"].(float64))
	batchRepo.failID = secondBatch
	cancel := generationRequest(t, mux, http.MethodPost, "/generation/runs/"+strconv.FormatInt(runID, 10)+"/cancel", "", 10, true)
	if cancel.Code != http.StatusConflict || !strings.Contains(cancel.Body.String(), "cancel_incomplete") {
		t.Fatalf("cancel status = %d body=%s", cancel.Code, cancel.Body.String())
	}
	get := generationRequest(t, mux, http.MethodGet, "/generation/runs/"+strconv.FormatInt(runID, 10), "", 10, true)
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d", get.Code)
	}
	var after map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after["state"] == "cancelled" {
		t.Fatalf("run reported cancelled despite live batch: %s", get.Body.String())
	}
}

func TestGenerationRunCancelGetProjectsCancelledBatchState(t *testing.T) {
	mux, _, _, _ := newRunAPI(t, &testGenerationAuthorizer{allowed: true})
	create := generationRequest(t, mux, http.MethodPost, "/generation/runs", runBody, 10, true)
	if create.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	runID := int64(created["id"].(float64))
	cancel := generationRequest(t, mux, http.MethodPost, "/generation/runs/"+strconv.FormatInt(runID, 10)+"/cancel", "", 10, true)
	if cancel.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body=%s", cancel.Code, cancel.Body.String())
	}
	get := generationRequest(t, mux, http.MethodGet, "/generation/runs/"+strconv.FormatInt(runID, 10), "", 10, true)
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", get.Code, get.Body.String())
	}
	var detail map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["state"] != "cancelled" {
		t.Fatalf("run state after cancel = %v", detail["state"])
	}
	unit := detail["units"].([]any)[0].(map[string]any)
	if unit["state"] != "cancelled" || unit["batch_state"] != "cancelled" || unit["stage"] != "cancelled" {
		t.Fatalf("unit projection after cancel = state:%v batch_state:%v stage:%v", unit["state"], unit["batch_state"], unit["stage"])
	}
	if unit["review_required"] != false {
		t.Fatalf("cancelled unit review_required = %v", unit["review_required"])
	}
	// Progress is completed work only: a cancelled run must report 0%, not 100%.
	if detail["completed_units"] != float64(0) {
		t.Fatalf("completed_units after cancel = %v", detail["completed_units"])
	}
	if detail["progress_percent"] != float64(0) {
		t.Fatalf("progress_percent after cancel = %v, want 0", detail["progress_percent"])
	}
}

// fakeRunEventsNotifier is a deterministic in-memory notifier for SSE tests.
type fakeRunEventsNotifier struct {
	mu   sync.Mutex
	subs map[int64]chan struct{}
}

func newFakeRunEventsNotifier() *fakeRunEventsNotifier {
	return &fakeRunEventsNotifier{subs: map[int64]chan struct{}{}}
}

func (f *fakeRunEventsNotifier) Subscribe(_ context.Context, runID int64) (<-chan struct{}, func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	channel := make(chan struct{}, 1)
	f.subs[runID] = channel
	cancel := func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.subs, runID)
	}
	return channel, cancel, nil
}

func (f *fakeRunEventsNotifier) dispatch(runID int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, channel := range f.subs {
		if id != runID {
			continue
		}
		select {
		case channel <- struct{}{}:
		default:
		}
	}
}

func newSSETestService(t *testing.T) (*application.GenerationRunService, *infra.MemoryGenerationRunRepository, *infra.MemoryGenerationRunEventRepository) {
	t.Helper()
	gen, err := id.New(6, nil)
	if err != nil {
		t.Fatal(err)
	}
	runRepo := infra.NewMemoryGenerationRunRepository()
	batchSvc := application.NewGenerationBatchService(infra.NewMemoryGenerationBatchRepository())
	batchSvc.SetScopePort(apiTestScopePort{repoID: 30})
	eventRepo := infra.NewMemoryGenerationRunEventRepository()
	svc := application.NewGenerationRunService(runRepo, batchSvc, gen)
	svc.SetEventPort(eventRepo)
	svc.SetPlanPort(infra.NewMemoryGenerationPlanRepository())
	return svc, runRepo, eventRepo
}

func seedRunningRun(t *testing.T, runRepo *infra.MemoryGenerationRunRepository, runID int64) {
	t.Helper()
	now := time.Now().UTC()
	run := &domain.GenerationRun{
		ID: runID, TeamID: 10, ProjectID: 20, ScopeType: "project", State: domain.GenerationRunRunning,
		TotalUnits: 1, IdempotencyKey: fmt.Sprintf("seed-%d", runID), CreatedBy: 40, CreatedAt: now, UpdatedAt: now,
		Units: []*domain.GenerationUnit{{ID: runID + 1, TeamID: 10, RunID: runID, ServiceID: 50, RepoID: 30, State: domain.GenerationUnitRunning}},
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
}

func TestGenerationRunEventsPeriodicSnapshotWithoutNewEvents(t *testing.T) {
	svc, runRepo, eventRepo := newSSETestService(t)
	seedRunningRun(t, runRepo, 700)
	handler := newRunEventHandler(svc, eventRepo, nil, nil, &testGenerationAuthorizer{allowed: true})
	handler.fallbackInterval = 40 * time.Millisecond
	handler.heartbeatInterval = 30 * time.Millisecond
	handler.snapshotInterval = 60 * time.Millisecond
	mux := http.NewServeMux()
	mux.HandleFunc("GET /generation/runs/{id}/events", handler.stream)

	ctx, cancel := context.WithTimeout(context.Background(), 260*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/generation/runs/700/events", nil).WithContext(ctx)
	req = req.WithContext(rls.WithTenant(req.Context(), 10))
	req = req.WithContext(iam.WithPrincipal(req.Context(), iam.Principal{Subject: "40", TenantID: "10"}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if got := strings.Count(body, `"type":"run.snapshot"`); got < 2 {
		t.Fatalf("snapshot frames = %d, want periodic snapshots: %s", got, body)
	}
	if !strings.Contains(body, ": keepalive") {
		t.Fatalf("keepalive missing: %s", body)
	}
}

func TestGenerationRunEventsNotifierWakeDeliversNewEvent(t *testing.T) {
	svc, runRepo, eventRepo := newSSETestService(t)
	seedRunningRun(t, runRepo, 710)
	notifier := newFakeRunEventsNotifier()
	handler := newRunEventHandler(svc, eventRepo, notifier, nil, &testGenerationAuthorizer{allowed: true})
	handler.fallbackInterval = 5 * time.Second
	handler.heartbeatInterval = 5 * time.Second
	handler.snapshotInterval = 5 * time.Second
	mux := http.NewServeMux()
	mux.HandleFunc("GET /generation/runs/{id}/events", handler.stream)

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/generation/runs/710/events", nil).WithContext(ctx)
	req = req.WithContext(rls.WithTenant(req.Context(), 10))
	req = req.WithContext(iam.WithPrincipal(req.Context(), iam.Principal{Subject: "40", TenantID: "10"}))
	rec := httptest.NewRecorder()
	go func() {
		time.Sleep(80 * time.Millisecond)
		_ = eventRepo.Append(context.Background(), &domain.GenerationRunEvent{
			ID: 1, TeamID: 10, RunID: 710, Type: domain.GenerationRunEventRunProgress,
			Payload: map[string]any{"state": "running"},
		})
		notifier.dispatch(710)
	}()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"type":"run.progress"`) {
		t.Fatalf("notifier wake did not deliver event: %s", rec.Body.String())
	}
}

type generationRunPageResponse struct {
	Runs       []map[string]any `json:"runs"`
	NextCursor *string          `json:"next_cursor"`
}

func newCursorPaginationAPI(t *testing.T) *http.ServeMux {
	t.Helper()
	mux, runRepo, _, _ := newRunAPI(t, &testGenerationAuthorizer{allowed: true})
	now := time.Now().UTC()
	states := map[int64]domain.GenerationRunState{
		1: domain.GenerationRunSucceeded, 2: domain.GenerationRunRunning, 3: domain.GenerationRunSucceeded,
		4: domain.GenerationRunFailed, 5: domain.GenerationRunSucceeded,
	}
	for id := int64(1); id <= 5; id++ {
		run := &domain.GenerationRun{
			ID: id, TeamID: 10, ProjectID: 20, ScopeType: "project", ScopeID: 0,
			State: states[id], TotalUnits: 1, IdempotencyKey: fmt.Sprintf("list-%d", id),
			CreatedBy: 40, CreatedAt: now.Add(time.Duration(id) * time.Second), UpdatedAt: now,
		}
		if err := runRepo.Create(context.Background(), run); err != nil {
			t.Fatal(err)
		}
	}
	return mux
}

func getGenerationRunPage(t *testing.T, mux *http.ServeMux, query string) generationRunPageResponse {
	t.Helper()
	rec := generationRequest(t, mux, http.MethodGet, "/generation/runs?"+query, "", 10, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}
	var page generationRunPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func generationRunPageIDs(page generationRunPageResponse) []int64 {
	out := make([]int64, 0, len(page.Runs))
	for _, run := range page.Runs {
		out = append(out, int64(run["id"].(float64)))
	}
	return out
}

func TestGenerationRunsListCursorPagination(t *testing.T) {
	mux := newCursorPaginationAPI(t)
	first := getGenerationRunPage(t, mux, "project_id=20&limit=2")
	if got := generationRunPageIDs(first); len(got) != 2 || got[0] != 5 || got[1] != 4 {
		t.Fatalf("first page ids = %v", got)
	}
	if first.NextCursor == nil || *first.NextCursor != "4" {
		t.Fatalf("first next_cursor = %v", first.NextCursor)
	}
	second := getGenerationRunPage(t, mux, "project_id=20&limit=2&cursor="+*first.NextCursor)
	if got := generationRunPageIDs(second); len(got) != 2 || got[0] != 3 || got[1] != 2 {
		t.Fatalf("second page ids = %v", got)
	}
	if second.NextCursor == nil || *second.NextCursor != "2" {
		t.Fatalf("second next_cursor = %v", second.NextCursor)
	}
	last := getGenerationRunPage(t, mux, "project_id=20&limit=2&cursor="+*second.NextCursor)
	if got := generationRunPageIDs(last); len(got) != 1 || got[0] != 1 {
		t.Fatalf("last page ids = %v", got)
	}
	if last.NextCursor != nil {
		t.Fatalf("last next_cursor = %v, want null", *last.NextCursor)
	}
}

func TestGenerationRunsListStateFilter(t *testing.T) {
	mux := newCursorPaginationAPI(t)
	filtered := getGenerationRunPage(t, mux, "project_id=20&limit=10&state=succeeded")
	if got := generationRunPageIDs(filtered); len(got) != 3 || got[0] != 5 || got[2] != 1 {
		t.Fatalf("filtered ids = %v", got)
	}
}

func TestGenerationRunsListRejectsInvalidQueries(t *testing.T) {
	mux := newCursorPaginationAPI(t)
	for _, query := range []string{"project_id=20&cursor=abc", "project_id=20&cursor=0", "project_id=20&state=unknown", "project_id=20&limit=0"} {
		if rec := generationRequest(t, mux, http.MethodGet, "/generation/runs?"+query, "", 10, true); rec.Code != http.StatusBadRequest {
			t.Fatalf("query %q status = %d", query, rec.Code)
		}
	}
}
