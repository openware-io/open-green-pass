package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// ErrGenerationRunCancelIncomplete means at least one active batch could not be
// cancelled. The run is persisted as partial_failed/failed instead of cancelled
// so the UI never claims a batch was stopped while it may still commit cases.
var ErrGenerationRunCancelIncomplete = errors.New("generation run cancel incomplete")

// GenerationRunUnitInput describes one service inside a run.
type GenerationRunUnitInput struct {
	ServiceID  int64
	RepoID     int64
	SourcePath string
	RefType    string
	RefName    string
	CommitSHA  string
}

// CreateGenerationRunCommand is one operator action.
type CreateGenerationRunCommand struct {
	ID             int64
	TeamID         int64
	ProjectID      int64
	ScopeType      string
	ScopeID        int64
	ActorID        int64
	IdempotencyKey string
	// PlanID/PlanHash bind the create request to a persisted preflight plan. The
	// server re-loads the plan and uses its resolved units.
	PlanID   int64
	PlanHash string
	Units    []GenerationRunUnitInput
}

// GenerationRunService coordinates a run over existing generation batches.
//
// Create is idempotent by tenant + idempotency_key, Get is a pure read, and all
// unit/run convergence is persisted by the batch-driven SyncRun path.
type GenerationRunService struct {
	runs    domain.GenerationRunRepository
	batches *GenerationBatchService
	gen     *id.Generator
	scope   domain.GenerationRunScopePort
	plans   domain.GenerationPlanRepository
	events  domain.GenerationRunEventRepository
	now     func() time.Time
	log     *slog.Logger
}

func NewGenerationRunService(runs domain.GenerationRunRepository, batches *GenerationBatchService, gen *id.Generator) *GenerationRunService {
	return &GenerationRunService{runs: runs, batches: batches, gen: gen, now: func() time.Time { return time.Now().UTC() }, log: slog.Default()}
}

func (s *GenerationRunService) SetScopePort(scope domain.GenerationRunScopePort) { s.scope = scope }

// SetPlanPort enables plan-bound creation. Without it only the legacy units
// request is accepted (embedded/test composition).
func (s *GenerationRunService) SetPlanPort(plans domain.GenerationPlanRepository) { s.plans = plans }

// SetEventPort enables append-only run events for SSE and audit.
func (s *GenerationRunService) SetEventPort(events domain.GenerationRunEventRepository) {
	s.events = events
}

// Find returns the persisted aggregate. It is a pure read.
func (s *GenerationRunService) Find(ctx context.Context, teamID, runID int64) (*domain.GenerationRun, error) {
	if s == nil || s.runs == nil {
		return nil, errors.New("generation run repository is required")
	}
	if teamID <= 0 || runID <= 0 {
		return nil, errors.New("generation run requires tenant and id")
	}
	return s.runs.Find(ctx, teamID, runID)
}

// Create creates the run and one batch per service. Batch failures are isolated
// per unit; a partial run remains visible instead of being silently dropped.
// The bool result is true only when a new run was created.
//
//nolint:gocyclo // Creation validates plan/idempotency/scope/units in one transaction boundary.
func (s *GenerationRunService) Create(ctx context.Context, cmd CreateGenerationRunCommand) (*domain.GenerationRun, bool, error) {
	if s == nil || s.runs == nil || s.batches == nil || s.gen == nil {
		return nil, false, errors.New("generation run dependencies are not configured")
	}
	if cmd.TeamID <= 0 || cmd.ProjectID <= 0 || cmd.ID <= 0 || cmd.ActorID <= 0 || strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return nil, false, errors.New("generation run requires tenant, project, actor, id and idempotency key")
	}
	// A plan-bound request takes its scope from the plan; the legacy request
	// still carries scope_type explicitly.
	if cmd.PlanID == 0 && !domain.ValidGenerationScopeType(cmd.ScopeType) {
		return nil, false, errors.New("generation run scope must be project, repo or service")
	}
	if cmd.PlanID > 0 && s.plans == nil {
		return nil, false, errors.New("generation plan repository is required for plan-bound runs")
	}

	// Idempotency is checked before plan validation: a retried request must
	// return the already-created run even if its plan TTL has since elapsed.
	if existing, err := s.runs.FindByIdempotencyKey(ctx, cmd.TeamID, cmd.IdempotencyKey); err == nil {
		if s.sameRunRequest(ctx, existing, cmd) {
			return existing, false, nil
		}
		return nil, false, domain.ErrGenerationRunIdempotencyConflict
	} else if !errors.Is(err, domain.ErrGenerationRunNotFound) {
		return nil, false, fmt.Errorf("find idempotent generation run: %w", err)
	}

	var plan *domain.GenerationPlan
	if cmd.PlanID > 0 {
		loaded, err := s.plans.Find(ctx, cmd.TeamID, cmd.PlanID)
		if err != nil {
			return nil, false, s.planError(err)
		}
		plan = loaded
		if err := s.validatePlanForCreate(plan, cmd); err != nil {
			// A same-key request may have consumed the plan and still be about to
			// insert its run; retry the read before reporting plan_stale.
			if errors.Is(err, domain.ErrGenerationPlanStale) || errors.Is(err, domain.ErrGenerationPlanExpired) {
				if existing, found, _ := s.waitForSameRun(ctx, cmd); found {
					return existing, false, nil
				}
			}
			return nil, false, err
		}
		valid := plan.ValidItems()
		if len(valid) != len(plan.Items) || len(valid) == 0 {
			return nil, false, domain.ErrGenerationPlanInvalid
		}
		cmd.ProjectID = plan.ProjectID
		cmd.ScopeType = plan.ScopeType
		cmd.ScopeID = plan.ScopeID
		if !domain.ValidGenerationScopeType(cmd.ScopeType) {
			return nil, false, errors.New("generation plan has an invalid scope")
		}
		cmd.Units = make([]GenerationRunUnitInput, 0, len(valid))
		for _, item := range valid {
			cmd.Units = append(cmd.Units, GenerationRunUnitInput{
				ServiceID: item.ServiceID, RepoID: item.RepoID, SourcePath: item.SourcePath,
				RefType: item.RefType, RefName: item.RefName, CommitSHA: item.CommitSHA,
			})
		}
	}
	if len(cmd.Units) == 0 {
		if s.scope == nil {
			return nil, false, errors.New("generation run scope resolver is required")
		}
		resolved, err := s.scope.ResolveGenerationRunScope(ctx, cmd.TeamID, cmd.ProjectID, cmd.ScopeType, cmd.ScopeID)
		if err != nil {
			return nil, false, err
		}
		if len(resolved) == 0 {
			return nil, false, errors.New("no confirmed services match the generation scope")
		}
		for _, item := range resolved {
			cmd.Units = append(cmd.Units, GenerationRunUnitInput{ServiceID: item.ServiceID, RepoID: item.RepoID, SourcePath: item.SourcePath, RefType: "branch", RefName: item.DefaultBranch})
		}
	}
	if len(cmd.Units) == 0 || len(cmd.Units) > 100 {
		return nil, false, errors.New("generation run requires between 1 and 100 service units")
	}

	now := s.now()
	run := &domain.GenerationRun{
		ID: cmd.ID, TeamID: cmd.TeamID, ProjectID: cmd.ProjectID, ScopeType: cmd.ScopeType,
		ScopeID: cmd.ScopeID, State: domain.GenerationRunQueued, TotalUnits: len(cmd.Units),
		IdempotencyKey: cmd.IdempotencyKey, CreatedBy: cmd.ActorID, CreatedAt: now, UpdatedAt: now,
	}
	if cmd.PlanID > 0 {
		planID := cmd.PlanID
		run.PlanID = &planID
		run.PlanHash = cmd.PlanHash
	}
	seen := make(map[int64]struct{}, len(cmd.Units))
	for _, input := range cmd.Units {
		input.RefType = strings.TrimSpace(input.RefType)
		if input.RefType == "" {
			input.RefType = "branch"
		}
		if input.ServiceID <= 0 || input.RepoID <= 0 {
			return nil, false, errors.New("generation run unit requires service and repo")
		}
		if input.RefType != "branch" && input.RefType != "tag" {
			return nil, false, errors.New("generation run unit reference type must be branch or tag")
		}
		sourcePath := ""
		if strings.TrimSpace(input.SourcePath) != "" {
			normalized, err := generationSourcePath(input.SourcePath)
			if err != nil {
				return nil, false, err
			}
			sourcePath = normalized
		}
		if _, exists := seen[input.ServiceID]; exists {
			return nil, false, errors.New("generation run contains duplicate service")
		}
		seen[input.ServiceID] = struct{}{}
		// Scope ownership and catalog eligibility are re-checked before the run
		// row exists, so scope_type/scope_id can never disagree with the units.
		if err := validateUnitScopeOwnership(cmd.ScopeType, cmd.ScopeID, domain.GenerationPlanUnitInput{ServiceID: input.ServiceID, RepoID: input.RepoID}); err != nil {
			return nil, false, s.scopeValidationError(cmd, err)
		}
		serviceID := input.ServiceID
		if err := s.batches.ValidateUnitScope(ctx, CreateGenerationBatchCommand{
			TeamID: cmd.TeamID, TargetID: cmd.ProjectID, RepoID: input.RepoID, ServiceID: &serviceID,
			SourcePath: input.SourcePath, RefType: input.RefType, Branch: input.RefName,
		}); err != nil {
			return nil, false, s.scopeValidationError(cmd, err)
		}
		run.Units = append(run.Units, &domain.GenerationUnit{
			ID: s.gen.Next(), TeamID: cmd.TeamID, RunID: cmd.ID, ServiceID: input.ServiceID,
			RepoID: input.RepoID, SourcePath: sourcePath, RefType: input.RefType, RefName: input.RefName,
			CommitSHA: input.CommitSHA, State: domain.GenerationUnitQueued,
		})
	}
	// Consume the plan atomically BEFORE creating the run. A plan backs at most
	// one run even when two different idempotency keys race; the loser re-reads
	// its own key first so same-key retries still return the winner's run.
	if plan != nil {
		consumed, err := s.plans.Consume(ctx, cmd.TeamID, plan.ID, run.ID, now)
		if err != nil {
			return nil, false, s.planError(err)
		}
		if !consumed {
			if existing, found, _ := s.waitForSameRun(ctx, cmd); found {
				return existing, false, nil
			}
			return nil, false, domain.ErrGenerationPlanStale
		}
	}
	if err := s.runs.Create(ctx, run); err != nil {
		// A competing create may have won the unique (team, idempotency_key)
		// index; retry the read briefly (the winner may still be committing its
		// transaction) and return its run instead of leaking a duplicate error.
		existing, found, different := s.waitForSameRun(ctx, cmd)
		switch {
		case found:
			return existing, false, nil
		case different:
			return nil, false, domain.ErrGenerationRunIdempotencyConflict
		default:
			return nil, false, fmt.Errorf("create generation run: %w", err)
		}
	}
	s.appendEvents(ctx, cmd.TeamID, run.ID,
		s.newRunEvent(cmd.TeamID, run.ID, domain.GenerationRunEventRunCreated, nil, "run.created", map[string]any{
			"state": string(run.State), "total_units": run.TotalUnits,
		}))

	for index, unit := range run.Units {
		input := cmd.Units[index]
		batch, _, err := s.batches.Create(ctx, CreateGenerationBatchCommand{
			ID: s.gen.Next(), TeamID: cmd.TeamID, TargetID: cmd.ProjectID, RepoID: input.RepoID,
			ServiceID: &input.ServiceID, SourcePath: input.SourcePath, RefType: input.RefType,
			Branch: input.RefName, IdempotencyKey: fmt.Sprintf("%s-%d", cmd.IdempotencyKey, input.ServiceID),
			ActorID: cmd.ActorID,
		})
		if err != nil {
			_ = s.runs.SetUnitState(ctx, cmd.TeamID, run.ID, unit.ID, domain.GenerationUnitFailed, err.Error())
			unit.State = domain.GenerationUnitFailed
			unit.Error = err.Error()
			s.appendEvents(ctx, cmd.TeamID, run.ID, s.newUnitEvent(cmd.TeamID, run.ID, unit.ID, domain.GenerationRunEventUnitFailed, map[string]any{
				"state": string(domain.GenerationUnitFailed), "error": err.Error(),
			}))
			continue
		}
		if err := s.runs.LinkBatch(ctx, cmd.TeamID, run.ID, unit.ID, batch.ID); err != nil {
			_ = s.runs.SetUnitState(ctx, cmd.TeamID, run.ID, unit.ID, domain.GenerationUnitFailed, err.Error())
			unit.State = domain.GenerationUnitFailed
			unit.Error = err.Error()
			continue
		}
		batchID := batch.ID
		unit.BatchID = &batchID
		unit.State = domain.GenerationUnitRunning
		unit.BatchState = string(batch.State)
		unit.Stage = string(batch.State)
	}
	state, completedUnits, failedUnits, waiting := domain.FoldGenerationRunState(false, run.Units)
	progress := domain.GenerationRunProgress{State: state, Completed: completedUnits, Failed: failedUnits, WaitingReview: waiting}
	if state != domain.GenerationRunQueued {
		started := now
		progress.StartedAt = &started
	}
	if err := s.runs.UpdateProgress(ctx, cmd.TeamID, run.ID, progress); err != nil {
		return nil, false, fmt.Errorf("update generation run: %w", err)
	}
	if state != domain.GenerationRunQueued {
		s.appendEvents(ctx, cmd.TeamID, run.ID, s.newRunEvent(cmd.TeamID, run.ID, domain.RunStateEventType(state), nil, "", map[string]any{
			"state": string(state), "completed_units": completedUnits, "failed_units": failedUnits, "total_units": run.TotalUnits,
		}))
	}
	refreshed, err := s.runs.Find(ctx, cmd.TeamID, run.ID)
	if err != nil {
		return nil, false, err
	}
	return refreshed, true, nil
}

// Get returns the persisted aggregate without mutating it. Convergence is the
// worker reconciler's job; GET purity is part of the frozen contract.
func (s *GenerationRunService) Get(ctx context.Context, teamID, runID int64) (*domain.GenerationRun, error) {
	return s.Find(ctx, teamID, runID)
}

// Cancel cancels all active service batches and then the run. It is idempotent:
// a terminal run is returned unchanged.
//
//nolint:gocyclo // Cancellation reconciles per-unit batch outcomes before closing the run.
func (s *GenerationRunService) Cancel(ctx context.Context, teamID, runID, actorID int64, requestID string) (*domain.GenerationRun, error) {
	run, err := s.Find(ctx, teamID, runID)
	if err != nil {
		return nil, err
	}
	if domain.IsTerminalRunState(run.State) {
		return run, nil
	}
	now := s.now()
	cancelFailures := make([]string, 0)
	failUnit := func(unit *domain.GenerationUnit, reason string) {
		_ = s.runs.UpdateUnit(ctx, teamID, runID, unit.ID, domain.GenerationRunUnitUpdate{
			State: domain.GenerationUnitFailed, BatchState: unit.BatchState, Stage: "cancel_failed",
			Error: reason, SyncedAt: now, FinishedAt: &now,
		})
		unit.State = domain.GenerationUnitFailed
		unit.Error = reason
		unit.FinishedAt = &now
		cancelFailures = append(cancelFailures, fmt.Sprintf("service %d: %s", unit.ServiceID, reason))
		s.appendEvents(ctx, teamID, runID, s.newUnitEvent(teamID, runID, unit.ID, domain.GenerationRunEventUnitFailed, map[string]any{
			"state": string(domain.GenerationUnitFailed), "reason": reason,
		}))
	}
	for _, unit := range run.Units {
		if unit.BatchID == nil {
			previous := unit.State
			if err := s.projectUnit(ctx, teamID, runID, unit, domain.GenerationRunUnitUpdate{
				State: domain.GenerationUnitCancelled, BatchState: "cancelled", Stage: "cancelled",
				ReviewRequired: false, Error: "run cancelled before batch started", SyncedAt: now, FinishedAt: &now,
			}); err != nil {
				return nil, err
			}
			if previous != domain.GenerationUnitCancelled {
				s.appendEvents(ctx, teamID, runID, s.newUnitEvent(teamID, runID, unit.ID, domain.GenerationRunEventUnitCancelled, map[string]any{"state": string(domain.GenerationUnitCancelled), "batch_state": "cancelled"}))
			}
			continue
		}
		batch, err := s.batches.Find(ctx, teamID, *unit.BatchID)
		if err != nil {
			// An unreadable batch means cancel was not confirmed; report failure
			// instead of pretending the unit stopped.
			failUnit(unit, "batch unavailable while cancelling")
			continue
		}
		switch batch.State {
		case domain.GenerationApproved, domain.GenerationRejected, domain.GenerationFailed, domain.GenerationRolledBack, domain.GenerationCancelled:
			// Already-terminal batch: cancel cannot change it, but the unit
			// projection must be repaired to the batch's real terminal stage so a
			// terminal run never keeps a stale stage (the reconciler skips
			// terminal runs).
			state, stage, reviewRequired := domain.GenerationUnitProjection(batch.State)
			errorSummary := ""
			if state == domain.GenerationUnitFailed || state == domain.GenerationUnitRejected {
				errorSummary = "batch already terminal"
			}
			previous := unit.State
			if err := s.projectUnit(ctx, teamID, runID, unit, domain.GenerationRunUnitUpdate{
				State: state, BatchState: string(batch.State), Stage: stage,
				ReviewRequired: reviewRequired, Error: errorSummary, SyncedAt: now, FinishedAt: &now,
			}); err != nil {
				return nil, err
			}
			if previous != state {
				s.appendEvents(ctx, teamID, runID, s.newUnitEvent(teamID, runID, unit.ID, domain.UnitStateEventType(state), map[string]any{
					"state": string(state), "batch_state": string(batch.State), "stage": stage,
				}))
			}
			continue
		}
		if _, cancelErr := s.batches.Cancel(ctx, CancelGenerationCommand{TeamID: teamID, BatchID: batch.ID, ActorID: actorID, ExpectedVersion: batch.StateVersion, RequestID: requestID + "-" + fmt.Sprint(batch.ID)}); cancelErr != nil {
			s.logWarn(ctx, "generation unit cancel failed", "team_id", teamID, "run_id", runID, "unit_id", unit.ID, "batch_id", batch.ID, "error", cancelErr)
			failUnit(unit, "batch cancel failed: "+cancelErr.Error())
			continue
		}
		if err := s.projectUnit(ctx, teamID, runID, unit, domain.GenerationRunUnitUpdate{
			State: domain.GenerationUnitCancelled, BatchState: "cancelled", Stage: "cancelled",
			ReviewRequired: false, Error: "run cancelled", SyncedAt: now, FinishedAt: &now,
		}); err != nil {
			return nil, err
		}
		s.appendEvents(ctx, teamID, runID, s.newUnitEvent(teamID, runID, unit.ID, domain.GenerationRunEventUnitCancelled, map[string]any{"state": string(domain.GenerationUnitCancelled), "batch_state": "cancelled"}))
	}
	if len(cancelFailures) > 0 {
		state, completed, failed, waiting := domain.FoldGenerationRunState(false, run.Units)
		if !domain.IsTerminalRunState(state) {
			// Some batches are still live; surface the inconsistency instead of
			// claiming cancellation.
			if completed > 0 {
				state = domain.GenerationRunPartialFailed
			} else {
				state = domain.GenerationRunFailed
			}
		} else if state == domain.GenerationRunFailed {
			// Mixed outcome: the cancel did stop some units but not all, so the
			// run is partially failed rather than fully failed.
			for _, unit := range run.Units {
				if unit.State == domain.GenerationUnitCancelled || unit.State == domain.GenerationUnitSucceeded {
					state = domain.GenerationRunPartialFailed
					break
				}
			}
		}
		finished := now
		progress := domain.GenerationRunProgress{
			State: state, Completed: completed, Failed: failed, WaitingReview: waiting,
			StartedAt: run.StartedAt, FinishedAt: &finished,
		}
		if progress.StartedAt == nil {
			progress.StartedAt = &now
		}
		if err := s.runs.UpdateProgress(ctx, teamID, runID, progress); err != nil {
			return nil, err
		}
		s.appendEvents(ctx, teamID, runID, s.newRunEvent(teamID, runID, domain.RunStateEventType(state), nil, "", map[string]any{
			"state": string(state), "completed_units": completed, "failed_units": failed,
			"cancel_failed_units": len(cancelFailures),
			"reason":              "cancel_incomplete",
			"failures":            truncateCancelFailures(cancelFailures),
		}))
		run, findErr := s.Find(ctx, teamID, runID)
		if findErr != nil {
			return nil, findErr
		}
		return run, fmt.Errorf("%w: %d unit(s): %s", ErrGenerationRunCancelIncomplete, len(cancelFailures), strings.Join(truncateCancelFailures(cancelFailures), "; "))
	}
	_, completed, failed, _ := domain.FoldGenerationRunState(true, run.Units)
	// Cancellation is authoritative only when every active batch was stopped;
	// already-terminal approved/rejected units keep their state.
	progress := domain.GenerationRunProgress{
		State: domain.GenerationRunCancelled, Completed: completed, Failed: failed,
		StartedAt: run.StartedAt, CancelledAt: &now, CancelledBy: &actorID,
	}
	if progress.StartedAt == nil {
		progress.StartedAt = &now
	}
	if err := s.runs.UpdateProgress(ctx, teamID, runID, progress); err != nil {
		return nil, err
	}
	s.appendEvents(ctx, teamID, runID, s.newRunEvent(teamID, runID, domain.GenerationRunEventRunCancelled, nil, "run.cancelled", map[string]any{
		"state": string(domain.GenerationRunCancelled), "completed_units": completed, "failed_units": failed,
	}))
	return s.Find(ctx, teamID, runID)
}

// truncateCancelFailures bounds the cancellable failure detail that is both
// returned and stored as an event payload (the event table enforces 4KB).
func truncateCancelFailures(failures []string) []string {
	const maxItems = 5
	const maxLen = 160
	out := make([]string, 0, min(len(failures), maxItems))
	for index, failure := range failures {
		if index >= maxItems {
			break
		}
		if len(failure) > maxLen {
			failure = failure[:maxLen]
		}
		out = append(out, failure)
	}
	return out
}

// projectUnit persists a full unit projection (state + batch_state/stage +
// review flag + timestamps) and mirrors it into the in-memory aggregate. Cancel
// uses it because a cancelled run is terminal: the reconciler will never revisit
// it, so GET must not keep a stale batch stage.
func (s *GenerationRunService) projectUnit(ctx context.Context, teamID, runID int64, unit *domain.GenerationUnit, update domain.GenerationRunUnitUpdate) error {
	if update.SyncedAt.IsZero() {
		update.SyncedAt = s.now()
	}
	if update.FinishedAt == nil {
		finished := update.SyncedAt
		update.FinishedAt = &finished
	}
	if err := s.runs.UpdateUnit(ctx, teamID, runID, unit.ID, update); err != nil {
		return err
	}
	unit.State = update.State
	unit.BatchState = update.BatchState
	unit.Stage = update.Stage
	unit.ReviewRequired = update.ReviewRequired
	unit.Error = update.Error
	unit.StartedAt = update.StartedAt
	unit.FinishedAt = update.FinishedAt
	return nil
}

// Bounded best-effort wait used only on the plan-consume stale path: a
// same-idempotency-key request may have won the plan CAS but not yet inserted its
// run row, so the loser retries the read briefly instead of returning plan_stale
// for what is a retry of the same request.
const (
	generationRunIdempotentRetryAttempts = 10
	generationRunIdempotentRetryDelay    = 25 * time.Millisecond
)

func (s *GenerationRunService) waitForSameRun(ctx context.Context, cmd CreateGenerationRunCommand) (existing *domain.GenerationRun, foundSame, foundDifferent bool) {
	for attempt := 0; attempt < generationRunIdempotentRetryAttempts; attempt++ {
		run, err := s.runs.FindByIdempotencyKey(ctx, cmd.TeamID, cmd.IdempotencyKey)
		if err == nil {
			if s.sameRunRequest(ctx, run, cmd) {
				return run, true, false
			}
			return nil, false, true
		}
		if !errors.Is(err, domain.ErrGenerationRunNotFound) {
			return nil, false, false
		}
		if attempt == generationRunIdempotentRetryAttempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, false, false
		case <-time.After(generationRunIdempotentRetryDelay):
		}
	}
	return nil, false, false
}

// ReviewGenerationRunCommand is one aggregate review decision.
type ReviewGenerationRunCommand struct {
	TeamID, RunID, ActorID int64
	Decision               string
	RequestID              string
}

// Review applies one decision to every unit that is currently awaiting review.
// Successful units are recorded individually; the aggregate is then re-derived
// from the authoritative batch states.
//
//nolint:gocyclo // legacy complexity; refactor tracked in lint backlog
func (s *GenerationRunService) Review(ctx context.Context, cmd ReviewGenerationRunCommand) (*domain.GenerationRun, error) {
	if cmd.TeamID <= 0 || cmd.RunID <= 0 || cmd.ActorID <= 0 || (cmd.Decision != "approve" && cmd.Decision != "reject") || strings.TrimSpace(cmd.RequestID) == "" {
		return nil, errors.New("run review requires tenant, run, actor, decision and request id")
	}
	run, err := s.Find(ctx, cmd.TeamID, cmd.RunID)
	if err != nil {
		return nil, err
	}
	reviewed := 0
	var firstErr error
	for _, unit := range run.Units {
		if unit.BatchID == nil {
			continue
		}
		batch, err := s.batches.Find(ctx, cmd.TeamID, *unit.BatchID)
		if err != nil || batch.State != domain.GenerationReview {
			continue
		}
		if _, err := s.batches.Review(ctx, ReviewGenerationCommand{TeamID: cmd.TeamID, BatchID: batch.ID, ActorID: cmd.ActorID, ExpectedVersion: batch.StateVersion, Decision: GenerationReviewDecision(cmd.Decision), RequestID: cmd.RequestID + "-" + fmt.Sprint(batch.ID)}); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		reviewed++
		// Do not append unit.approved/unit.rejected here: at approve time the
		// batch is only committing, so the real approval event must come from
		// SyncRun when gen_batch reaches approved. Emitting here would duplicate
		// the event (review-time + reconciliation-time). Reject is projected
		// immediately by the SyncRun call below.
	}
	if reviewed == 0 {
		if firstErr != nil {
			return nil, fmt.Errorf("review unit: %w", firstErr)
		}
		return nil, errors.New("no generation units are waiting for review")
	}
	if firstErr != nil {
		s.logWarn(ctx, "partial generation review", "team_id", cmd.TeamID, "run_id", cmd.RunID, "reviewed", reviewed, "error", firstErr)
	}
	return s.SyncRun(ctx, cmd.TeamID, cmd.RunID)
}

// SyncRun re-derives persisted unit/run state from the authoritative batch
// states and appends events for effective changes. It is shared by review and
// the worker reconciler; it is never called from GET.
//
//nolint:gocyclo // Projection reconciliation deliberately enumerates every batch/unit transition.
func (s *GenerationRunService) SyncRun(ctx context.Context, teamID, runID int64) (*domain.GenerationRun, error) {
	if s == nil || s.runs == nil || s.batches == nil {
		return nil, errors.New("generation run dependencies are not configured")
	}
	run, err := s.runs.Find(ctx, teamID, runID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for _, unit := range run.Units {
		if unit.BatchID == nil {
			if domain.IsTerminalUnitState(unit.State) || run.CancelledAt != nil {
				continue
			}
			// Crash recovery: run+unit were persisted but the batch was not
			// created (or was created before LinkBatch). Recreate it with the
			// same per-unit idempotency key used at create time; if it already
			// exists, batch Create returns it and we just link.
			if err := s.recoverMissingBatch(ctx, run, unit); err != nil {
				reason := "batch_not_created: " + err.Error()
				finished := now
				_ = s.runs.UpdateUnit(ctx, teamID, runID, unit.ID, domain.GenerationRunUnitUpdate{
					State: domain.GenerationUnitFailed, Stage: "batch_not_created",
					Error: reason, SyncedAt: now, FinishedAt: &finished,
				})
				unit.State = domain.GenerationUnitFailed
				unit.Stage = "batch_not_created"
				unit.Error = reason
				unit.FinishedAt = &finished
				s.appendEvents(ctx, teamID, runID, s.newUnitEvent(teamID, runID, unit.ID, domain.GenerationRunEventUnitFailed, map[string]any{
					"state": string(domain.GenerationUnitFailed), "reason": "batch_not_created", "error": err.Error(),
				}))
				continue
			}
			unit.State = domain.GenerationUnitRunning
			unit.BatchState = string(domain.GenerationEnqueued)
			unit.Stage = string(domain.GenerationEnqueued)
			unit.Error = ""
			s.appendEvents(ctx, teamID, runID, s.newUnitEvent(teamID, runID, unit.ID, domain.GenerationRunEventUnitQueued, map[string]any{
				"state": string(domain.GenerationUnitRunning), "batch_state": string(domain.GenerationEnqueued), "recovered": true,
			}))
			continue
		}
		batch, err := s.batches.Find(ctx, teamID, *unit.BatchID)
		if err != nil {
			// The batch is the authority; a transient read failure leaves the
			// last projection untouched for the next tick.
			continue
		}
		state, stage, reviewRequired := domain.GenerationUnitProjection(batch.State)
		errorSummary := ""
		if state == domain.GenerationUnitFailed || state == domain.GenerationUnitRejected {
			errorSummary = batch.FailureSummary
		}
		if unit.State == state && unit.BatchState == string(batch.State) && unit.Stage == stage &&
			unit.ReviewRequired == reviewRequired && unit.Error == errorSummary {
			continue
		}
		update := domain.GenerationRunUnitUpdate{
			State: state, BatchState: string(batch.State), Stage: stage,
			ReviewRequired: reviewRequired, Error: errorSummary, SyncedAt: now,
		}
		if update.StartedAt = unit.StartedAt; update.StartedAt == nil {
			update.StartedAt = &now
		}
		if update.FinishedAt = unit.FinishedAt; update.FinishedAt == nil && domain.IsTerminalUnitState(state) {
			update.FinishedAt = &now
		}
		if err := s.runs.UpdateUnit(ctx, teamID, runID, unit.ID, update); err != nil {
			return nil, err
		}
		unit.State = state
		unit.BatchState = update.BatchState
		unit.Stage = stage
		unit.ReviewRequired = reviewRequired
		unit.Error = errorSummary
		unit.StartedAt = update.StartedAt
		unit.FinishedAt = update.FinishedAt
		// Terminal unit events carry a deterministic event_key so concurrent
		// reconcilers (or a retried reconciliation) cannot duplicate
		// unit.approved/unit.rejected/unit.failed/unit.cancelled history.
		eventKey := ""
		if domain.IsTerminalUnitState(state) {
			eventKey = fmt.Sprintf("unit-state:%d:%s", unit.ID, state)
		}
		s.appendEvents(ctx, teamID, runID, s.newUnitEventWithKey(teamID, runID, unit.ID, domain.UnitStateEventType(state), eventKey, map[string]any{
			"state": string(state), "batch_state": update.BatchState, "stage": stage,
		}))
	}
	state, completed, failed, waiting := domain.FoldGenerationRunState(run.CancelledAt != nil, run.Units)
	progress := domain.GenerationRunProgress{
		State: state, Completed: completed, Failed: failed, WaitingReview: waiting,
		StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, CancelledAt: run.CancelledAt, CancelledBy: run.CancelledBy,
	}
	runChanged := state != run.State || completed != run.CompletedUnits || failed != run.FailedUnits || waiting != run.WaitingReviewUnits
	if progress.StartedAt == nil && state != domain.GenerationRunQueued {
		started := now
		progress.StartedAt = &started
		runChanged = true
	}
	if progress.FinishedAt == nil && domain.IsTerminalRunState(state) {
		finished := now
		progress.FinishedAt = &finished
		runChanged = true
	}
	if !runChanged {
		return run, nil
	}
	if err := s.runs.UpdateProgress(ctx, teamID, runID, progress); err != nil {
		return nil, err
	}
	s.appendEvents(ctx, teamID, runID, s.newRunEvent(teamID, runID, domain.RunStateEventType(state), nil, "", map[string]any{
		"state": string(state), "completed_units": completed, "failed_units": failed,
		"review_units": waiting, "total_units": run.TotalUnits,
	}))
	return s.runs.Find(ctx, teamID, runID)
}

// recoverMissingBatch recreates the durable batch for a unit whose batch link is
// missing. The idempotency key is deterministic (run key + service id), so a
// concurrent reconciler or a partially-completed original create cannot produce
// a second batch; it either creates one or links the existing one.
func (s *GenerationRunService) recoverMissingBatch(ctx context.Context, run *domain.GenerationRun, unit *domain.GenerationUnit) error {
	if unit.ServiceID <= 0 || unit.RepoID <= 0 {
		return errors.New("unit is missing service or repo")
	}
	serviceID := unit.ServiceID
	batch, _, err := s.batches.Create(ctx, CreateGenerationBatchCommand{
		ID: s.gen.Next(), TeamID: run.TeamID, TargetID: run.ProjectID, RepoID: unit.RepoID,
		ServiceID: &serviceID, SourcePath: unit.SourcePath, RefType: unit.RefType, Branch: unit.RefName,
		IdempotencyKey: fmt.Sprintf("%s-%d", run.IdempotencyKey, unit.ServiceID), ActorID: run.CreatedBy,
	})
	if err != nil {
		return err
	}
	return s.runs.LinkBatch(ctx, run.TeamID, run.ID, unit.ID, batch.ID)
}

// ListByProject returns aggregate runs for the UI.
func (s *GenerationRunService) ListByProject(ctx context.Context, teamID, projectID int64, limit int) ([]*domain.GenerationRun, error) {
	return s.runs.ListByProject(ctx, teamID, projectID, limit)
}

// ListByProjectPage returns one stable id-descending page of project runs and a
// next cursor when older runs exist. state is an optional exact run-state filter
// ("" = all states); the HTTP adapter validates the state value.
func (s *GenerationRunService) ListByProjectPage(ctx context.Context, teamID, projectID, beforeID int64, state string, limit int) (*domain.GenerationRunPage, error) {
	if s == nil || s.runs == nil {
		return nil, errors.New("generation run repository is required")
	}
	if teamID <= 0 || projectID <= 0 {
		return nil, errors.New("generation run page requires tenant and project")
	}
	if beforeID < 0 {
		return nil, errors.New("generation run cursor must be a positive id")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.runs.ListProjectRunsPage(ctx, teamID, projectID, beforeID, state, limit)
}

// validatePlanForCreate enforces TTL, tenant/project, hash and single-use rules.
// Scope is intentionally not compared with the request: a plan-bound create
// takes scope from the plan (the frozen snapshot), so external clients only
// need plan_id+plan_hash+project_id.
func (s *GenerationRunService) validatePlanForCreate(plan *domain.GenerationPlan, cmd CreateGenerationRunCommand) error {
	if plan == nil || plan.TeamID != cmd.TeamID || plan.ProjectID != cmd.ProjectID {
		return domain.ErrGenerationPlanStale
	}
	if strings.TrimSpace(cmd.PlanHash) == "" || cmd.PlanHash != plan.PlanHash {
		return domain.ErrGenerationPlanStale
	}
	if plan.State != domain.GenerationPlanDraft {
		return domain.ErrGenerationPlanStale
	}
	if plan.Expired(s.now()) {
		return domain.ErrGenerationPlanExpired
	}
	return nil
}

// scopeValidationError maps a unit/scope inconsistency to plan_stale for
// plan-bound creates (the frozen plan no longer matches the catalog or the
// requested scope) and to a validation error for legacy requests.
func (s *GenerationRunService) scopeValidationError(cmd CreateGenerationRunCommand, err error) error {
	if cmd.PlanID > 0 {
		return fmt.Errorf("%w: %v", domain.ErrGenerationPlanStale, err)
	}
	return err
}

// planError maps plan-repository errors to the stable confirmation semantics.
func (s *GenerationRunService) planError(err error) error {
	switch {
	case errors.Is(err, domain.ErrGenerationPlanNotFound):
		return domain.ErrGenerationPlanStale
	case errors.Is(err, domain.ErrGenerationPlanExpired):
		return domain.ErrGenerationPlanExpired
	default:
		return err
	}
}

// sameRunRequest compares the durable run with a retried request. A plan-bound
// retry is identified by project + plan id+hash (scope is plan-owned); a legacy
// retry is identified by scope and unit set.
func (s *GenerationRunService) sameRunRequest(ctx context.Context, run *domain.GenerationRun, cmd CreateGenerationRunCommand) bool {
	if run.TeamID != cmd.TeamID || run.ProjectID != cmd.ProjectID {
		return false
	}
	if cmd.PlanID > 0 {
		return run.PlanID != nil && *run.PlanID == cmd.PlanID && run.PlanHash == cmd.PlanHash
	}
	if run.ScopeType != cmd.ScopeType || run.ScopeID != cmd.ScopeID {
		return false
	}
	units := cmd.Units
	if len(units) == 0 {
		if s.scope == nil {
			return false
		}
		resolved, err := s.scope.ResolveGenerationRunScope(ctx, cmd.TeamID, cmd.ProjectID, cmd.ScopeType, cmd.ScopeID)
		if err != nil {
			return false
		}
		for _, item := range resolved {
			units = append(units, GenerationRunUnitInput{ServiceID: item.ServiceID, RepoID: item.RepoID, SourcePath: item.SourcePath, RefType: "branch", RefName: item.DefaultBranch})
		}
	}
	return sameRunUnits(run.Units, units)
}

func sameRunUnits(existing []*domain.GenerationUnit, requested []GenerationRunUnitInput) bool {
	if len(existing) != len(requested) {
		return false
	}
	expected := make(map[int64]string, len(requested))
	for _, unit := range requested {
		normalized, err := generationSourcePath(unit.SourcePath)
		if err != nil {
			return false
		}
		refType := strings.TrimSpace(unit.RefType)
		if refType == "" {
			refType = "branch"
		}
		expected[unit.ServiceID] = fmt.Sprintf("%d|%s|%s|%s", unit.RepoID, normalized, refType, unit.RefName)
	}
	if len(expected) != len(existing) {
		return false
	}
	seen := map[int64]struct{}{}
	for _, unit := range existing {
		key, ok := expected[unit.ServiceID]
		if !ok {
			return false
		}
		if _, duplicate := seen[unit.ServiceID]; duplicate {
			return false
		}
		seen[unit.ServiceID] = struct{}{}
		actual := fmt.Sprintf("%d|%s|%s|%s", unit.RepoID, unit.SourcePath, unit.RefType, unit.RefName)
		if actual != key {
			return false
		}
	}
	return true
}

func (s *GenerationRunService) newRunEvent(teamID, runID int64, eventType domain.GenerationRunEventType, unitID *int64, eventKey string, payload map[string]any) *domain.GenerationRunEvent {
	var eventID int64
	if s.gen != nil {
		eventID = s.gen.Next()
	}
	return &domain.GenerationRunEvent{
		ID: eventID, TeamID: teamID, RunID: runID, Type: eventType, UnitID: unitID,
		EventKey: eventKey, Payload: payload, CreatedAt: s.now(),
	}
}

func (s *GenerationRunService) newUnitEvent(teamID, runID, unitID int64, eventType domain.GenerationRunEventType, payload map[string]any) *domain.GenerationRunEvent {
	id := unitID
	return s.newRunEvent(teamID, runID, eventType, &id, "", payload)
}

// newUnitEventWithKey builds a unit event with an idempotency key; the event
// store enforces UNIQUE(run_id,event_key) so duplicate terminal events are
// dropped instead of polluting the SSE replay stream.
func (s *GenerationRunService) newUnitEventWithKey(teamID, runID, unitID int64, eventType domain.GenerationRunEventType, eventKey string, payload map[string]any) *domain.GenerationRunEvent {
	id := unitID
	return s.newRunEvent(teamID, runID, eventType, &id, eventKey, payload)
}

// appendEvents is best effort: event write failures must never fail or roll
// back the authoritative state transition.
func (s *GenerationRunService) appendEvents(ctx context.Context, teamID, runID int64, events ...*domain.GenerationRunEvent) {
	if s.events == nil {
		return
	}
	for _, event := range events {
		if event == nil {
			continue
		}
		if err := s.events.Append(ctx, event); err != nil {
			s.logWarn(ctx, "append generation run event failed", "team_id", teamID, "run_id", runID, "event_type", event.Type, "error", err)
		}
	}
}

func (s *GenerationRunService) logWarn(ctx context.Context, message string, args ...any) {
	if s.log == nil {
		return
	}
	s.log.WarnContext(ctx, message, args...)
}
