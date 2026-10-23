package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// GenerationRunUnitInput describes one service inside a run.
type GenerationRunUnitInput struct {
	ServiceID  int64
	RepoID     int64
	SourcePath string
	RefType    string
	RefName    string
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
	Units          []GenerationRunUnitInput
}

// GenerationRunService coordinates a run over existing generation batches.
type GenerationRunService struct {
	runs    domain.GenerationRunRepository
	batches *GenerationBatchService
	gen     *id.Generator
}

func NewGenerationRunService(runs domain.GenerationRunRepository, batches *GenerationBatchService, gen *id.Generator) *GenerationRunService {
	return &GenerationRunService{runs: runs, batches: batches, gen: gen}
}

// Create creates the run and one batch per service. Batch failures are isolated
// per unit; a partial run remains visible instead of being silently dropped.
func (s *GenerationRunService) Create(ctx context.Context, cmd CreateGenerationRunCommand) (*domain.GenerationRun, error) {
	if cmd.TeamID <= 0 || cmd.ProjectID <= 0 || cmd.ID <= 0 || cmd.ActorID <= 0 || strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return nil, errors.New("generation run requires tenant, project, actor, id and idempotency key")
	}
	if cmd.ScopeType != "project" && cmd.ScopeType != "repo" && cmd.ScopeType != "service" {
		return nil, errors.New("generation run scope must be project, repo or service")
	}
	if len(cmd.Units) == 0 || len(cmd.Units) > 100 {
		return nil, errors.New("generation run requires between 1 and 100 service units")
	}
	run := &domain.GenerationRun{
		ID: cmd.ID, TeamID: cmd.TeamID, ProjectID: cmd.ProjectID, ScopeType: cmd.ScopeType,
		ScopeID: cmd.ScopeID, State: domain.GenerationRunQueued, TotalUnits: len(cmd.Units),
		IdempotencyKey: cmd.IdempotencyKey, CreatedBy: cmd.ActorID,
	}
	seen := make(map[int64]struct{}, len(cmd.Units))
	for _, input := range cmd.Units {
		if input.ServiceID <= 0 || input.RepoID <= 0 {
			return nil, errors.New("generation run unit requires service and repo")
		}
		if _, exists := seen[input.ServiceID]; exists {
			return nil, errors.New("generation run contains duplicate service")
		}
		seen[input.ServiceID] = struct{}{}
		run.Units = append(run.Units, &domain.GenerationUnit{
			ID: s.gen.Next(), TeamID: cmd.TeamID, RunID: cmd.ID, ServiceID: input.ServiceID,
			RepoID: input.RepoID, State: domain.GenerationUnitQueued,
		})
	}
	if err := s.runs.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("create generation run: %w", err)
	}
	completed, failed := 0, 0
	for index, unit := range run.Units {
		input := cmd.Units[index]
		batch, _, err := s.batches.Create(ctx, CreateGenerationBatchCommand{
			ID: s.gen.Next(), TeamID: cmd.TeamID, TargetID: cmd.ProjectID, RepoID: input.RepoID,
			ServiceID: &input.ServiceID, SourcePath: input.SourcePath, RefType: input.RefType,
			Branch: input.RefName, IdempotencyKey: fmt.Sprintf("%s-%d", cmd.IdempotencyKey, input.ServiceID),
			ActorID: cmd.ActorID,
		})
		if err != nil {
			failed++
			_ = s.runs.SetUnitState(ctx, cmd.TeamID, run.ID, unit.ID, domain.GenerationUnitFailed, err.Error())
			continue
		}
		if err := s.runs.LinkBatch(ctx, cmd.TeamID, run.ID, unit.ID, batch.ID); err != nil {
			failed++
			_ = s.runs.SetUnitState(ctx, cmd.TeamID, run.ID, unit.ID, domain.GenerationUnitFailed, err.Error())
			continue
		}
		unit.BatchID = &batch.ID
		unit.State = domain.GenerationUnitRunning
		completed++
	}
	state := domain.GenerationRunRunning
	if completed == 0 {
		state = domain.GenerationRunFailed
	} else if failed > 0 {
		state = domain.GenerationRunPartialFailed
	}
	if err := s.runs.UpdateState(ctx, cmd.TeamID, run.ID, state, completed, failed); err != nil {
		return nil, fmt.Errorf("update generation run: %w", err)
	}
	return s.Get(ctx, cmd.TeamID, run.ID)
}

// Get refreshes unit states from their batch records and folds them into one
// aggregate run state.
func (s *GenerationRunService) Get(ctx context.Context, teamID, runID int64) (*domain.GenerationRun, error) {
	run, err := s.runs.Find(ctx, teamID, runID)
	if err != nil {
		return nil, err
	}
	if run.State == domain.GenerationRunCancelled {
		return run, nil
	}
	completed, failed := 0, 0
	for _, unit := range run.Units {
		if unit.State == domain.GenerationUnitCancelled {
			continue
		}
		if unit.BatchID == nil {
			if unit.State == domain.GenerationUnitFailed {
				failed++
			}
			continue
		}
		batch, err := s.batches.Find(ctx, teamID, *unit.BatchID)
		if err != nil {
			continue
		}
		switch batch.State {
		case domain.GenerationApproved:
			unit.State = domain.GenerationUnitSucceeded
			completed++
		case domain.GenerationCancelled:
			unit.State = domain.GenerationUnitCancelled
			unit.Error = "batch cancelled"
			failed++
		case domain.GenerationFailed, domain.GenerationRejected, domain.GenerationRolledBack:
			unit.State = domain.GenerationUnitFailed
			unit.Error = batch.FailureSummary
			failed++
		default:
			unit.State = domain.GenerationUnitRunning
		}
	}
	state := domain.GenerationRunRunning
	if completed+failed == run.TotalUnits {
		switch {
		case failed == 0:
			state = domain.GenerationRunSucceeded
		case completed == 0:
			state = domain.GenerationRunFailed
		default:
			state = domain.GenerationRunPartialFailed
		}
	}
	if err := s.runs.UpdateState(ctx, teamID, run.ID, state, completed, failed); err != nil {
		return nil, err
	}
	run.State = state
	run.CompletedUnits = completed
	run.FailedUnits = failed
	return run, nil
}

// Cancel cancels all active service batches and then the run.
func (s *GenerationRunService) Cancel(ctx context.Context, teamID, runID, actorID int64, requestID string) (*domain.GenerationRun, error) {
	run, err := s.runs.Find(ctx, teamID, runID)
	if err != nil {
		return nil, err
	}
	if run.State == domain.GenerationRunCancelled || run.State == domain.GenerationRunSucceeded || run.State == domain.GenerationRunFailed {
		return run, nil
	}
	completed := 0
	for _, unit := range run.Units {
		if unit.BatchID == nil {
			_ = s.runs.SetUnitState(ctx, teamID, runID, unit.ID, domain.GenerationUnitCancelled, "run cancelled before batch started")
			continue
		}
		batch, err := s.batches.Find(ctx, teamID, *unit.BatchID)
		if err != nil {
			_ = s.runs.SetUnitState(ctx, teamID, runID, unit.ID, domain.GenerationUnitCancelled, err.Error())
			continue
		}
		if batch.State == domain.GenerationApproved {
			_ = s.runs.SetUnitState(ctx, teamID, runID, unit.ID, domain.GenerationUnitSucceeded, "")
			completed++
			continue
		}
		if batch.State == domain.GenerationRejected || batch.State == domain.GenerationRolledBack || batch.State == domain.GenerationFailed || batch.State == domain.GenerationCancelled {
			_ = s.runs.SetUnitState(ctx, teamID, runID, unit.ID, domain.GenerationUnitCancelled, "batch already terminal")
			continue
		}
		_, cancelErr := s.batches.Cancel(ctx, CancelGenerationCommand{TeamID: teamID, BatchID: batch.ID, ActorID: actorID, ExpectedVersion: batch.StateVersion, RequestID: requestID + "-" + fmt.Sprint(batch.ID)})
		if cancelErr != nil {
			_ = s.runs.SetUnitState(ctx, teamID, runID, unit.ID, domain.GenerationUnitFailed, cancelErr.Error())
			continue
		}
		_ = s.runs.SetUnitState(ctx, teamID, runID, unit.ID, domain.GenerationUnitCancelled, "run cancelled")
	}
	if err := s.runs.UpdateState(ctx, teamID, runID, domain.GenerationRunCancelled, completed, 0); err != nil {
		return nil, err
	}
	return s.runs.Find(ctx, teamID, runID)
}

type ReviewGenerationRunCommand struct {
	TeamID, RunID, ActorID int64
	Decision               string
	RequestID              string
}

// Review applies one decision to every unit that is currently awaiting review.
func (s *GenerationRunService) Review(ctx context.Context, cmd ReviewGenerationRunCommand) (*domain.GenerationRun, error) {
	if cmd.TeamID <= 0 || cmd.RunID <= 0 || cmd.ActorID <= 0 || (cmd.Decision != "approve" && cmd.Decision != "reject") || strings.TrimSpace(cmd.RequestID) == "" {
		return nil, errors.New("run review requires tenant, run, actor, decision and request id")
	}
	run, err := s.runs.Find(ctx, cmd.TeamID, cmd.RunID)
	if err != nil {
		return nil, err
	}
	reviewed := 0
	for _, unit := range run.Units {
		if unit.BatchID == nil {
			continue
		}
		batch, err := s.batches.Find(ctx, cmd.TeamID, *unit.BatchID)
		if err != nil || batch.State != domain.GenerationReview {
			continue
		}
		if _, err := s.batches.Review(ctx, ReviewGenerationCommand{TeamID: cmd.TeamID, BatchID: batch.ID, ActorID: cmd.ActorID, ExpectedVersion: batch.StateVersion, Decision: GenerationReviewDecision(cmd.Decision), RequestID: cmd.RequestID + "-" + fmt.Sprint(batch.ID)}); err != nil {
			return nil, fmt.Errorf("review unit %d: %w", unit.ServiceID, err)
		}
		reviewed++
	}
	if reviewed == 0 {
		return nil, errors.New("no generation units are waiting for review")
	}
	return s.Get(ctx, cmd.TeamID, cmd.RunID)
}

// ListByProject returns aggregate runs for the UI.
func (s *GenerationRunService) ListByProject(ctx context.Context, teamID, projectID int64, limit int) ([]*domain.GenerationRun, error) {
	return s.runs.ListByProject(ctx, teamID, projectID, limit)
}
