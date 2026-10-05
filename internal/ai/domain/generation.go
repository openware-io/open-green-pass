package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// GenerationBatchState is the externally visible state of a W1 generation
// batch. A batch never writes case versions before it reaches committing.
type GenerationBatchState string

const (
	GenerationEnqueued     GenerationBatchState = "enqueued"
	GenerationFetching     GenerationBatchState = "fetching"
	GenerationParsing      GenerationBatchState = "parsing"
	GenerationGenerating   GenerationBatchState = "generating"
	GenerationQualityCheck GenerationBatchState = "quality_check"
	GenerationReview       GenerationBatchState = "review"
	GenerationCommitting   GenerationBatchState = "committing"
	GenerationApproved     GenerationBatchState = "approved"
	GenerationRejected     GenerationBatchState = "rejected"
	GenerationRolledBack   GenerationBatchState = "rolled_back"
	GenerationFailed       GenerationBatchState = "failed"
)

var (
	ErrInvalidGenerationTransition = errors.New("invalid generation batch state transition")
	ErrGenerationStateConflict     = errors.New("generation batch state version conflict")
	ErrGenerationNotFound          = errors.New("generation batch not found")
)

// GenerationBatch pins the source input and records only metadata needed to
// recover the generation pipeline. Prompt/response bodies intentionally do
// not belong here; they must stay in a controlled object store.
type GenerationBatch struct {
	ID               int64
	TeamID           int64
	TargetID         int64
	RepoID           int64
	Branch           string
	RequestedVersion string
	HeadSHA          string
	State            GenerationBatchState
	StateVersion     int64
	WorkflowID       string
	IdempotencyKey   string
	ParentBatchID    *int64
	CreatedBy        int64
	FailureSummary   string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NewGenerationBatch constructs an enqueued batch. IDs are supplied by the
// application boundary so the domain has no dependency on an ID generator.
func NewGenerationBatch(id, teamID, targetID, repoID, createdBy int64, branch, requestedVersion, idempotencyKey string, now time.Time) (*GenerationBatch, error) {
	if id <= 0 || teamID <= 0 || targetID <= 0 || repoID <= 0 || branch == "" || idempotencyKey == "" {
		return nil, fmt.Errorf("generation batch requires id, tenant, target, repo, branch, and idempotency key")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &GenerationBatch{
		ID: id, TeamID: teamID, TargetID: targetID, RepoID: repoID,
		Branch: branch, RequestedVersion: requestedVersion,
		State: GenerationEnqueued, StateVersion: 1, IdempotencyKey: idempotencyKey,
		CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// WorkflowIdentity is stable across retries and is safe to use as a Temporal
// workflow ID. It deliberately uses no user-controlled text.
func (b GenerationBatch) WorkflowIdentity() string {
	return fmt.Sprintf("generation/%d/%d", b.TeamID, b.ID)
}

// Transition moves a batch through the only legal state graph. expectedVersion
// is checked by repositories atomically; this local check catches stale callers
// before they attempt persistence.
func (b *GenerationBatch) Transition(next GenerationBatchState, expectedVersion int64, now time.Time) error {
	if expectedVersion != b.StateVersion {
		return ErrGenerationStateConflict
	}
	if !canTransition(b.State, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidGenerationTransition, b.State, next)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	b.State = next
	b.StateVersion++
	b.UpdatedAt = now
	return nil
}

func canTransition(from, to GenerationBatchState) bool {
	switch from {
	case GenerationEnqueued:
		return to == GenerationFetching || to == GenerationFailed
	case GenerationFetching:
		return to == GenerationParsing || to == GenerationFailed
	case GenerationParsing:
		return to == GenerationGenerating || to == GenerationFailed
	case GenerationGenerating:
		return to == GenerationQualityCheck || to == GenerationFailed
	case GenerationQualityCheck:
		return to == GenerationReview || to == GenerationFailed
	case GenerationReview:
		return to == GenerationCommitting || to == GenerationRejected || to == GenerationFailed
	case GenerationCommitting:
		return to == GenerationApproved || to == GenerationFailed
	case GenerationApproved:
		return to == GenerationRolledBack
	default:
		return false
	}
}

// GenerationBatchRepository is the persistence boundary for generation batch
// metadata. CompareAndSwap must atomically compare state_version and persist a
// new version so concurrent review/rollback commands cannot overwrite one
// another.
type GenerationBatchRepository interface {
	Create(ctx context.Context, batch *GenerationBatch) error
	Find(ctx context.Context, teamID, batchID int64) (*GenerationBatch, error)
	FindByIdempotencyKey(ctx context.Context, teamID int64, key string) (*GenerationBatch, error)
	CompareAndSwap(ctx context.Context, batch *GenerationBatch, expectedVersion int64) error
}
