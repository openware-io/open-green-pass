package domain

import (
	"context"
	"errors"
	"time"
)

// GenerationRunState is the aggregate state of one operator action.
type GenerationRunState string

const (
	GenerationRunQueued  GenerationRunState = "queued"
	GenerationRunRunning GenerationRunState = "running"
	// GenerationRunWaitingReview means at least one unit reached the batch review
	// gate and the operator must approve or reject it.
	GenerationRunWaitingReview GenerationRunState = "waiting_review"
	GenerationRunSucceeded     GenerationRunState = "succeeded"
	GenerationRunPartialFailed GenerationRunState = "partial_failed"
	GenerationRunFailed        GenerationRunState = "failed"
	GenerationRunCancelled     GenerationRunState = "cancelled"
)

// GenerationUnitState tracks one service inside a run.
type GenerationUnitState string

const (
	GenerationUnitQueued        GenerationUnitState = "queued"
	GenerationUnitRunning       GenerationUnitState = "running"
	GenerationUnitWaitingReview GenerationUnitState = "waiting_review"
	GenerationUnitSucceeded     GenerationUnitState = "succeeded"
	GenerationUnitFailed        GenerationUnitState = "failed"
	// GenerationUnitRejected means the operator rejected the batch at review.
	GenerationUnitRejected  GenerationUnitState = "rejected"
	GenerationUnitCancelled GenerationUnitState = "cancelled"
)

var (
	ErrGenerationRunNotFound = errors.New("generation run not found")
	ErrGenerationRunConflict = errors.New("generation run state conflict")
)

// GenerationRun groups all service-level units created by one action.
type GenerationRun struct {
	ID             int64
	TeamID         int64
	ProjectID      int64
	ScopeType      string
	ScopeID        int64
	State          GenerationRunState
	TotalUnits     int
	CompletedUnits int
	FailedUnits    int
	// WaitingReviewUnits is the number of units currently awaiting human review.
	WaitingReviewUnits int
	TokensIn           int64
	TokensOut          int64
	CostAmount         float64
	PlanID             *int64
	PlanHash           string
	IdempotencyKey     string
	CreatedBy          int64
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CancelledAt        *time.Time
	CancelledBy        *int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Units              []*GenerationUnit
}

// GenerationUnit is one service inside a run. BatchID is the existing durable
// generation batch that owns the DSH/Temporal pipeline. BatchState/Stage are the
// batch-derived projection reconciled by the worker.
type GenerationUnit struct {
	ID             int64
	TeamID         int64
	RunID          int64
	ServiceID      int64
	ServiceName    string
	RepoID         int64
	RepoURL        string
	SourcePath     string
	RefType        string
	RefName        string
	CommitSHA      string
	BatchID        *int64
	BatchState     string
	Stage          string
	ReviewRequired bool
	State          GenerationUnitState
	Error          string
	StartedAt      *time.Time
	FinishedAt     *time.Time
	LastSyncedAt   *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// GenerationRunUnitUpdate is the persisted batch-derived projection of a unit.
type GenerationRunUnitUpdate struct {
	State          GenerationUnitState
	BatchState     string
	Stage          string
	ReviewRequired bool
	Error          string
	StartedAt      *time.Time
	FinishedAt     *time.Time
	SyncedAt       time.Time
}

// GenerationRunProgress is the persisted aggregate of a run.
type GenerationRunProgress struct {
	State         GenerationRunState
	Completed     int
	Failed        int
	WaitingReview int
	StartedAt     *time.Time
	FinishedAt    *time.Time
	CancelledAt   *time.Time
	CancelledBy   *int64
}

// GenerationReconcileCandidate is one non-terminal run found by the background
// reconciler. It carries team_id because the reconciler has no request tenant.
type GenerationReconcileCandidate struct {
	TeamID int64
	RunID  int64
}

// GenerationRunPage is one stable id-descending page of project runs.
// NextCursor is set only when at least one older run exists; it is the id of the
// last run in this page and is passed back as the next request's cursor.
type GenerationRunPage struct {
	Runs       []*GenerationRun
	NextCursor *int64
}

// GenerationRunScope is one eligible service resolved from the catalog.
type GenerationRunScope struct {
	ServiceID     int64
	RepoID        int64
	SourcePath    string
	DefaultBranch string
}

// GenerationRunScopePort expands a run scope into eligible services.
type GenerationRunScopePort interface {
	ResolveGenerationRunScope(ctx context.Context, teamID, projectID int64, scopeType string, scopeID int64) ([]GenerationRunScope, error)
}

// GenerationRunRepository persists run and unit state. UpdateUnit persists the
// authoritative unit projection; Get never writes it. ListReconcileCandidates
// is a background-maintenance read owned by the worker reconciler.
type GenerationRunRepository interface {
	Create(ctx context.Context, run *GenerationRun) error
	Find(ctx context.Context, teamID, runID int64) (*GenerationRun, error)
	// FindByIdempotencyKey is the durable half of create idempotency. A repeated
	// request must read this row instead of creating a second run/batch.
	FindByIdempotencyKey(ctx context.Context, teamID int64, key string) (*GenerationRun, error)
	ListByProject(ctx context.Context, teamID, projectID int64, limit int) ([]*GenerationRun, error)
	// ListProjectRunsPage returns runs strictly older than beforeID (0 = first
	// page) ordered by id DESC. state optionally filters the run state. The page
	// must report NextCursor only when more rows exist.
	ListProjectRunsPage(ctx context.Context, teamID, projectID, beforeID int64, state string, limit int) (*GenerationRunPage, error)
	LinkBatch(ctx context.Context, teamID, runID, unitID, batchID int64) error
	SetUnitState(ctx context.Context, teamID, runID, unitID int64, state GenerationUnitState, errorSummary string) error
	UpdateState(ctx context.Context, teamID, runID int64, state GenerationRunState, completed, failed int) error
	// UpdateUnit persists the batch-derived unit projection (state, batch_state,
	// stage, review flag, timestamps, error).
	UpdateUnit(ctx context.Context, teamID, runID, unitID int64, update GenerationRunUnitUpdate) error
	// UpdateProgress persists run aggregates and lifecycle timestamps.
	UpdateProgress(ctx context.Context, teamID, runID int64, progress GenerationRunProgress) error
	// ListReconcileCandidates returns non-terminal runs across tenants for the
	// worker reconciler. It is a bounded background-maintenance read because the
	// background job has no request tenant.
	ListReconcileCandidates(ctx context.Context, limit int) ([]GenerationReconcileCandidate, error)
}

// GenerationUnitProjection maps the authoritative batch state to the run-unit
// projection. It is the single mapping used by both review and reconciler.
func GenerationUnitProjection(batchState GenerationBatchState) (state GenerationUnitState, stage string, reviewRequired bool) {
	switch batchState {
	case GenerationEnqueued:
		return GenerationUnitQueued, "enqueued", false
	case GenerationFetching, GenerationParsing, GenerationGenerating, GenerationQualityCheck, GenerationCommitting:
		return GenerationUnitRunning, string(batchState), false
	case GenerationReview:
		return GenerationUnitWaitingReview, "review", true
	case GenerationApproved:
		return GenerationUnitSucceeded, "approved", false
	case GenerationRejected:
		return GenerationUnitRejected, "rejected", false
	case GenerationRolledBack:
		return GenerationUnitFailed, "rolled_back", false
	case GenerationCancelled:
		return GenerationUnitCancelled, "cancelled", false
	case GenerationFailed:
		return GenerationUnitFailed, "failed", false
	default:
		return GenerationUnitQueued, string(batchState), false
	}
}

// unitTerminal reports whether no further batch transition can change the unit.
func unitTerminal(state GenerationUnitState) bool {
	switch state {
	case GenerationUnitSucceeded, GenerationUnitFailed, GenerationUnitRejected, GenerationUnitCancelled:
		return true
	}
	return false
}

// IsTerminalUnitState reports whether a unit state is final.
func IsTerminalUnitState(state GenerationUnitState) bool { return unitTerminal(state) }

// FoldGenerationRunState derives the aggregate run state and counters from the
// persisted unit states. The caller persists the result; GET must never write.
//
//nolint:gocyclo // State folding is intentionally exhaustive over unit/batch states.
func FoldGenerationRunState(cancelled bool, units []*GenerationUnit) (state GenerationRunState, completed, failed, waiting int) {
	total := 0
	terminal := 0
	running := 0
	cancelledUnits := 0
	for _, unit := range units {
		if unit == nil {
			continue
		}
		total++
		switch unit.State {
		case GenerationUnitSucceeded:
			completed++
		case GenerationUnitFailed, GenerationUnitRejected:
			failed++
		case GenerationUnitWaitingReview:
			waiting++
		case GenerationUnitRunning:
			running++
		case GenerationUnitCancelled:
			cancelledUnits++
		}
		if unitTerminal(unit.State) {
			terminal++
		}
	}
	if total == 0 {
		return GenerationRunQueued, completed, failed, waiting
	}
	if waiting > 0 {
		// A human decision is required before the aggregate can finish.
		return GenerationRunWaitingReview, completed, failed, waiting
	}
	if terminal == total {
		switch {
		case cancelled || cancelledUnits == total:
			return GenerationRunCancelled, completed, failed, waiting
		case completed == total:
			return GenerationRunSucceeded, completed, failed, waiting
		case completed == 0:
			return GenerationRunFailed, completed, failed, waiting
		default:
			return GenerationRunPartialFailed, completed, failed, waiting
		}
	}
	if running > 0 || completed+failed+cancelledUnits > 0 {
		return GenerationRunRunning, completed, failed, waiting
	}
	return GenerationRunQueued, completed, failed, waiting
}

// IsTerminalRunState reports whether a run state stops all reconciliation.
func IsTerminalRunState(state GenerationRunState) bool {
	switch state {
	case GenerationRunSucceeded, GenerationRunPartialFailed, GenerationRunFailed, GenerationRunCancelled:
		return true
	}
	return false
}

// ProgressPercent derives the completed-work percentage from the persisted
// counters. Only succeeded and failed units count as completed work: cancelled
// units are intentionally excluded so a cancelled run reports 0% with
// completed_units=0 instead of a misleading full bar.
func ProgressPercent(total, completed, failed int) int {
	if total <= 0 {
		return 0
	}
	finished := completed + failed
	if finished > total {
		finished = total
	}
	return finished * 100 / total
}
