package infra

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// MemoryGenerationRunRepository is a deterministic fake for application, api,
// and reconciler tests. Production code must use GenerationRunStore, which
// enforces the same idempotency and tenant constraints in PostgreSQL.
//
// Write accounting is intentional: the GET-purity test asserts that reading a
// run never mutates the repository.
type MemoryGenerationRunRepository struct {
	mu     sync.Mutex
	runs   map[int64]*domain.GenerationRun
	byKey  map[generationRunKey]int64
	writes int
	reads  int
}

type generationRunKey struct {
	teamID int64
	key    string
}

func NewMemoryGenerationRunRepository() *MemoryGenerationRunRepository {
	return &MemoryGenerationRunRepository{
		runs:  map[int64]*domain.GenerationRun{},
		byKey: map[generationRunKey]int64{},
	}
}

// WriteCount exposes how many mutating calls were made; GET purity tests use it.
func (r *MemoryGenerationRunRepository) WriteCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writes
}

// ReadCount exposes how many read calls were made.
func (r *MemoryGenerationRunRepository) ReadCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reads
}

func (r *MemoryGenerationRunRepository) Create(_ context.Context, run *domain.GenerationRun) error {
	if run == nil {
		return domain.ErrGenerationRunNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runs[run.ID]; exists {
		return domain.ErrGenerationRunConflict
	}
	if _, exists := r.byKey[generationRunKey{run.TeamID, run.IdempotencyKey}]; exists {
		return domain.ErrGenerationRunConflict
	}
	r.runs[run.ID] = cloneGenerationRun(run)
	r.byKey[generationRunKey{run.TeamID, run.IdempotencyKey}] = run.ID
	r.writes++
	return nil
}

func (r *MemoryGenerationRunRepository) Find(_ context.Context, teamID, runID int64) (*domain.GenerationRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads++
	run, ok := r.runs[runID]
	if !ok || run.TeamID != teamID {
		return nil, domain.ErrGenerationRunNotFound
	}
	return cloneGenerationRun(run), nil
}

func (r *MemoryGenerationRunRepository) FindByIdempotencyKey(_ context.Context, teamID int64, key string) (*domain.GenerationRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads++
	id, ok := r.byKey[generationRunKey{teamID, key}]
	if !ok {
		return nil, domain.ErrGenerationRunNotFound
	}
	return cloneGenerationRun(r.runs[id]), nil
}

func (r *MemoryGenerationRunRepository) ListByProject(_ context.Context, teamID, projectID int64, limit int) ([]*domain.GenerationRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads++
	result := make([]*domain.GenerationRun, 0)
	for _, run := range r.runs {
		if run.TeamID == teamID && run.ProjectID == projectID {
			result = append(result, cloneGenerationRun(run))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID > result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *MemoryGenerationRunRepository) ListProjectRunsPage(_ context.Context, teamID, projectID, beforeID int64, state string, limit int) (*domain.GenerationRunPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads++
	result := make([]*domain.GenerationRun, 0)
	for _, run := range r.runs {
		if run.TeamID != teamID || run.ProjectID != projectID {
			continue
		}
		if beforeID > 0 && run.ID >= beforeID {
			continue
		}
		if state != "" && string(run.State) != state {
			continue
		}
		result = append(result, cloneGenerationRun(run))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID > result[j].ID })
	page := &domain.GenerationRunPage{Runs: result}
	if len(result) > limit {
		page.Runs = result[:limit]
		cursor := page.Runs[len(page.Runs)-1].ID
		page.NextCursor = &cursor
	}
	return page, nil
}

func (r *MemoryGenerationRunRepository) LinkBatch(_ context.Context, teamID, runID, unitID, batchID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[runID]
	if !ok || run.TeamID != teamID {
		return domain.ErrGenerationRunNotFound
	}
	for _, unit := range run.Units {
		if unit.ID != unitID {
			continue
		}
		unit.BatchID = &batchID
		unit.State = domain.GenerationUnitRunning
		unit.BatchState = string(domain.GenerationEnqueued)
		unit.Stage = string(domain.GenerationEnqueued)
		r.writes++
		return nil
	}
	return domain.ErrGenerationRunNotFound
}

func (r *MemoryGenerationRunRepository) SetUnitState(_ context.Context, teamID, runID, unitID int64, state domain.GenerationUnitState, summary string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[runID]
	if !ok || run.TeamID != teamID {
		return domain.ErrGenerationRunNotFound
	}
	for _, unit := range run.Units {
		if unit.ID == unitID {
			unit.State = state
			unit.Error = summary
			r.writes++
			return nil
		}
	}
	return domain.ErrGenerationRunNotFound
}

func (r *MemoryGenerationRunRepository) UpdateState(_ context.Context, teamID, runID int64, state domain.GenerationRunState, completed, failed int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[runID]
	if !ok || run.TeamID != teamID {
		return domain.ErrGenerationRunNotFound
	}
	run.State = state
	run.CompletedUnits = completed
	run.FailedUnits = failed
	r.writes++
	return nil
}

func (r *MemoryGenerationRunRepository) UpdateUnit(_ context.Context, teamID, runID, unitID int64, update domain.GenerationRunUnitUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[runID]
	if !ok || run.TeamID != teamID {
		return domain.ErrGenerationRunNotFound
	}
	for _, unit := range run.Units {
		if unit.ID != unitID {
			continue
		}
		unit.State = update.State
		unit.BatchState = update.BatchState
		unit.Stage = update.Stage
		unit.ReviewRequired = update.ReviewRequired
		unit.Error = update.Error
		unit.StartedAt = cloneTime(update.StartedAt)
		unit.FinishedAt = cloneTime(update.FinishedAt)
		unit.LastSyncedAt = cloneTime(&update.SyncedAt)
		r.writes++
		return nil
	}
	return domain.ErrGenerationRunNotFound
}

func (r *MemoryGenerationRunRepository) UpdateProgress(_ context.Context, teamID, runID int64, progress domain.GenerationRunProgress) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[runID]
	if !ok || run.TeamID != teamID {
		return domain.ErrGenerationRunNotFound
	}
	run.State = progress.State
	run.CompletedUnits = progress.Completed
	run.FailedUnits = progress.Failed
	run.WaitingReviewUnits = progress.WaitingReview
	run.StartedAt = cloneTime(progress.StartedAt)
	run.FinishedAt = cloneTime(progress.FinishedAt)
	run.CancelledAt = cloneTime(progress.CancelledAt)
	if progress.CancelledBy != nil {
		value := *progress.CancelledBy
		run.CancelledBy = &value
	}
	r.writes++
	return nil
}

func (r *MemoryGenerationRunRepository) ListReconcileCandidates(_ context.Context, limit int) ([]domain.GenerationReconcileCandidate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads++
	result := make([]domain.GenerationReconcileCandidate, 0)
	ids := make([]int64, 0, len(r.runs))
	for id := range r.runs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		run := r.runs[id]
		if domain.IsTerminalRunState(run.State) {
			continue
		}
		result = append(result, domain.GenerationReconcileCandidate{TeamID: run.TeamID, RunID: run.ID})
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

// MemoryGenerationPlanRepository is the deterministic plan-repository fake.
type MemoryGenerationPlanRepository struct {
	mu    sync.Mutex
	plans map[int64]*domain.GenerationPlan
}

func NewMemoryGenerationPlanRepository() *MemoryGenerationPlanRepository {
	return &MemoryGenerationPlanRepository{plans: map[int64]*domain.GenerationPlan{}}
}

func (r *MemoryGenerationPlanRepository) Create(_ context.Context, plan *domain.GenerationPlan) error {
	if plan == nil {
		return domain.ErrGenerationPlanNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plans[plan.ID] = cloneGenerationPlan(plan)
	return nil
}

func (r *MemoryGenerationPlanRepository) Find(_ context.Context, teamID, planID int64) (*domain.GenerationPlan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	plan, ok := r.plans[planID]
	if !ok || plan.TeamID != teamID {
		return nil, domain.ErrGenerationPlanNotFound
	}
	return cloneGenerationPlan(plan), nil
}

func (r *MemoryGenerationPlanRepository) FindIdempotent(_ context.Context, teamID int64, planHash string, now time.Time) (*domain.GenerationPlan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var newest *domain.GenerationPlan
	for _, plan := range r.plans {
		if plan.TeamID != teamID || plan.PlanHash != planHash || plan.State != domain.GenerationPlanDraft || plan.Expired(now) {
			continue
		}
		if newest == nil || plan.CreatedAt.After(newest.CreatedAt) || (plan.CreatedAt.Equal(newest.CreatedAt) && plan.ID > newest.ID) {
			newest = plan
		}
	}
	if newest == nil {
		return nil, domain.ErrGenerationPlanNotFound
	}
	return cloneGenerationPlan(newest), nil
}

func (r *MemoryGenerationPlanRepository) Consume(_ context.Context, teamID, planID, runID int64, now time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	plan, ok := r.plans[planID]
	if !ok || plan.TeamID != teamID {
		return false, domain.ErrGenerationPlanNotFound
	}
	if plan.State != domain.GenerationPlanDraft || plan.Expired(now) {
		return false, nil
	}
	plan.State = domain.GenerationPlanConsumed
	value := runID
	plan.ConfirmedRunID = &value
	return true, nil
}

func (r *MemoryGenerationPlanRepository) ExpireBefore(_ context.Context, now time.Time, limit int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var expired int64
	for _, plan := range r.plans {
		if plan.State != domain.GenerationPlanDraft || !plan.Expired(now) {
			continue
		}
		plan.State = domain.GenerationPlanExpired
		expired++
		if limit > 0 && int(expired) >= limit {
			break
		}
	}
	return expired, nil
}

// MemoryGenerationRunEventRepository is the deterministic append-only fake.
type MemoryGenerationRunEventRepository struct {
	mu     sync.Mutex
	events []*domain.GenerationRunEvent
	seq    int64
}

func NewMemoryGenerationRunEventRepository() *MemoryGenerationRunEventRepository {
	return &MemoryGenerationRunEventRepository{events: []*domain.GenerationRunEvent{}}
}

func (r *MemoryGenerationRunEventRepository) Append(_ context.Context, event *domain.GenerationRunEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if event.EventKey != "" {
		for _, existing := range r.events {
			if existing.RunID == event.RunID && existing.EventKey == event.EventKey {
				return nil
			}
		}
	}
	r.seq++
	cloned := *event
	cloned.Seq = r.seq
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = time.Now().UTC()
	}
	r.events = append(r.events, &cloned)
	return nil
}

func (r *MemoryGenerationRunEventRepository) ListAfter(_ context.Context, teamID, runID, afterSeq int64, limit int) ([]*domain.GenerationRunEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]*domain.GenerationRunEvent, 0)
	for _, event := range r.events {
		if event.TeamID != teamID || event.RunID != runID || event.Seq <= afterSeq {
			continue
		}
		cloned := *event
		result = append(result, &cloned)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Seq < result[j].Seq })
	return result, nil
}

func (r *MemoryGenerationRunEventRepository) LastSeq(_ context.Context, teamID, runID int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var last int64
	for _, event := range r.events {
		if event.TeamID == teamID && event.RunID == runID && event.Seq > last {
			last = event.Seq
		}
	}
	return last, nil
}

func cloneGenerationRun(in *domain.GenerationRun) *domain.GenerationRun {
	if in == nil {
		return nil
	}
	out := *in
	out.StartedAt = cloneTime(in.StartedAt)
	out.FinishedAt = cloneTime(in.FinishedAt)
	out.CancelledAt = cloneTime(in.CancelledAt)
	if in.CancelledBy != nil {
		value := *in.CancelledBy
		out.CancelledBy = &value
	}
	if in.PlanID != nil {
		value := *in.PlanID
		out.PlanID = &value
	}
	out.Units = make([]*domain.GenerationUnit, 0, len(in.Units))
	for _, unit := range in.Units {
		copied := *unit
		if unit.BatchID != nil {
			value := *unit.BatchID
			copied.BatchID = &value
		}
		copied.StartedAt = cloneTime(unit.StartedAt)
		copied.FinishedAt = cloneTime(unit.FinishedAt)
		copied.LastSyncedAt = cloneTime(unit.LastSyncedAt)
		out.Units = append(out.Units, &copied)
	}
	return &out
}

func cloneGenerationPlan(in *domain.GenerationPlan) *domain.GenerationPlan {
	if in == nil {
		return nil
	}
	out := *in
	if in.ConfirmedRunID != nil {
		value := *in.ConfirmedRunID
		out.ConfirmedRunID = &value
	}
	out.Items = append([]domain.GenerationPlanItem(nil), in.Items...)
	out.Risks = append([]domain.GenerationPlanRisk(nil), in.Risks...)
	out.Request.Units = append([]domain.GenerationPlanUnitInput(nil), in.Request.Units...)
	return &out
}

func cloneTime(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}
