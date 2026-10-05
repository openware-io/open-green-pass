package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// CreateGenerationBatchCommand contains only source identifiers and no source
// content. The source is frozen later by the FetchSource activity.
type CreateGenerationBatchCommand struct {
	ID               int64
	TeamID           int64
	TargetID         int64
	RepoID           int64
	Branch           string
	RequestedVersion string
	IdempotencyKey   string
	ActorID          int64
}

// GenerationBatchService owns idempotent batch creation and optimistic state
// transitions. It intentionally has no dependency on Temporal, SCM, AI, or
// governance case storage.
type GenerationBatchService struct {
	batches domain.GenerationBatchRepository
	now     func() time.Time
}

func NewGenerationBatchService(batches domain.GenerationBatchRepository) *GenerationBatchService {
	return &GenerationBatchService{batches: batches, now: func() time.Time { return time.Now().UTC() }}
}

// Create returns the existing batch for a repeated tenant-scoped idempotency
// key. Repository implementations must enforce the same uniqueness in durable
// storage to preserve this guarantee across processes.
func (s *GenerationBatchService) Create(ctx context.Context, cmd CreateGenerationBatchCommand) (*domain.GenerationBatch, bool, error) {
	if s == nil || s.batches == nil {
		return nil, false, errors.New("generation batch repository is required")
	}
	existing, err := s.batches.FindByIdempotencyKey(ctx, cmd.TeamID, cmd.IdempotencyKey)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, domain.ErrGenerationNotFound) {
		return nil, false, err
	}
	batch, err := domain.NewGenerationBatch(cmd.ID, cmd.TeamID, cmd.TargetID, cmd.RepoID, cmd.ActorID, cmd.Branch, cmd.RequestedVersion, cmd.IdempotencyKey, s.now())
	if err != nil {
		return nil, false, err
	}
	if err := s.batches.Create(ctx, batch); err != nil {
		// A competing create may have won. Re-read instead of leaking a
		// duplicate-key error as a failed idempotent request.
		if got, findErr := s.batches.FindByIdempotencyKey(ctx, cmd.TeamID, cmd.IdempotencyKey); findErr == nil {
			return got, false, nil
		}
		return nil, false, err
	}
	return batch, true, nil
}

func (s *GenerationBatchService) Transition(ctx context.Context, teamID, batchID, expectedVersion int64, next domain.GenerationBatchState) (*domain.GenerationBatch, error) {
	if s == nil || s.batches == nil {
		return nil, errors.New("generation batch repository is required")
	}
	batch, err := s.batches.Find(ctx, teamID, batchID)
	if err != nil {
		return nil, err
	}
	if err := batch.Transition(next, expectedVersion, s.now()); err != nil {
		return nil, err
	}
	if err := s.batches.CompareAndSwap(ctx, batch, expectedVersion); err != nil {
		return nil, fmt.Errorf("persist generation batch transition: %w", err)
	}
	return batch, nil
}
