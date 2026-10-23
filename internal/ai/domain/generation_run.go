package domain

import (
	"context"
	"errors"
	"time"
)

// GenerationRunState is the aggregate state of one operator action.
type GenerationRunState string

const (
	GenerationRunQueued        GenerationRunState = "queued"
	GenerationRunRunning       GenerationRunState = "running"
	GenerationRunSucceeded     GenerationRunState = "succeeded"
	GenerationRunPartialFailed GenerationRunState = "partial_failed"
	GenerationRunFailed        GenerationRunState = "failed"
	GenerationRunCancelled     GenerationRunState = "cancelled"
)

// GenerationUnitState tracks one service inside a run.
type GenerationUnitState string

const (
	GenerationUnitQueued    GenerationUnitState = "queued"
	GenerationUnitRunning   GenerationUnitState = "running"
	GenerationUnitSucceeded GenerationUnitState = "succeeded"
	GenerationUnitFailed    GenerationUnitState = "failed"
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
	TokensIn       int64
	TokensOut      int64
	CostAmount     float64
	IdempotencyKey string
	CreatedBy      int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Units          []*GenerationUnit
}

// GenerationUnit is one service inside a run. BatchID is the existing durable
// generation batch that owns the DSH/Temporal pipeline.
type GenerationUnit struct {
	ID        int64
	TeamID    int64
	RunID     int64
	ServiceID int64
	RepoID    int64
	BatchID   *int64
	State     GenerationUnitState
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
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

// GenerationRunRepository persists run and unit state.
type GenerationRunRepository interface {
	Create(ctx context.Context, run *GenerationRun) error
	Find(ctx context.Context, teamID, runID int64) (*GenerationRun, error)
	ListByProject(ctx context.Context, teamID, projectID int64, limit int) ([]*GenerationRun, error)
	LinkBatch(ctx context.Context, teamID, runID, unitID, batchID int64) error
	SetUnitState(ctx context.Context, teamID, runID, unitID int64, state GenerationUnitState, errorSummary string) error
	UpdateState(ctx context.Context, teamID, runID int64, state GenerationRunState, completed, failed int) error
}
