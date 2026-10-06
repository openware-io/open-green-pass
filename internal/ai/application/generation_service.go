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
	signals GenerationSignalPort
	audit   GenerationAuditPort
	now     func() time.Time
}

type GenerationReviewDecision string

const (
	GenerationReviewApprove GenerationReviewDecision = "approve"
	GenerationReviewReject  GenerationReviewDecision = "reject"
)

type ReviewGenerationCommand struct {
	TeamID, BatchID, ActorID, ExpectedVersion int64
	Decision                                  GenerationReviewDecision
	RequestID                                 string
}

type RollbackGenerationCommand struct {
	TeamID, BatchID, ActorID, ExpectedVersion int64
	RequestID                                 string
}

type GenerationSignal struct {
	TeamID, BatchID, ActorID, StateVersion int64
	Decision                               GenerationReviewDecision
	RequestID                              string
}

type GenerationSignalPort interface {
	SignalReview(context.Context, GenerationSignal) error
	SignalRollback(context.Context, GenerationSignal) error
}

type GenerationAuditEvent struct {
	TeamID, BatchID, ActorID, StateVersion int64
	Action, RequestID                      string
	OccurredAt                             time.Time
}

type GenerationAuditPort interface {
	AppendGenerationAudit(context.Context, GenerationAuditEvent) error
}

func NewGenerationBatchService(batches domain.GenerationBatchRepository) *GenerationBatchService {
	return &GenerationBatchService{batches: batches, now: func() time.Time { return time.Now().UTC() }}
}

func (s *GenerationBatchService) SetReviewPorts(signals GenerationSignalPort, audit GenerationAuditPort) {
	s.signals, s.audit = signals, audit
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

func (s *GenerationBatchService) Review(ctx context.Context, cmd ReviewGenerationCommand) (*domain.GenerationBatch, error) {
	if cmd.TeamID <= 0 || cmd.BatchID <= 0 || cmd.ActorID <= 0 || cmd.ExpectedVersion <= 0 || cmd.RequestID == "" {
		return nil, errors.New("review requires tenant, batch, actor, version, and request id")
	}
	var next domain.GenerationBatchState
	switch cmd.Decision {
	case GenerationReviewApprove:
		next = domain.GenerationCommitting
	case GenerationReviewReject:
		next = domain.GenerationRejected
	default:
		return nil, errors.New("review decision must be approve or reject")
	}
	batch, err := s.Transition(ctx, cmd.TeamID, cmd.BatchID, cmd.ExpectedVersion, next)
	if err != nil {
		return nil, err
	}
	event := GenerationAuditEvent{TeamID: cmd.TeamID, BatchID: cmd.BatchID, ActorID: cmd.ActorID, StateVersion: batch.StateVersion, Action: "generation.review." + string(cmd.Decision), RequestID: cmd.RequestID, OccurredAt: s.now()}
	if s.audit != nil {
		if err := s.audit.AppendGenerationAudit(ctx, event); err != nil {
			return nil, fmt.Errorf("append generation review audit: %w", err)
		}
	}
	if s.signals != nil {
		if err := s.signals.SignalReview(ctx, GenerationSignal{TeamID: cmd.TeamID, BatchID: cmd.BatchID, ActorID: cmd.ActorID, StateVersion: batch.StateVersion, Decision: cmd.Decision, RequestID: cmd.RequestID}); err != nil {
			return nil, fmt.Errorf("signal generation review: %w", err)
		}
	}
	return batch, nil
}

func (s *GenerationBatchService) Rollback(ctx context.Context, cmd RollbackGenerationCommand) (*domain.GenerationBatch, error) {
	if cmd.TeamID <= 0 || cmd.BatchID <= 0 || cmd.ActorID <= 0 || cmd.ExpectedVersion <= 0 || cmd.RequestID == "" {
		return nil, errors.New("rollback requires tenant, batch, actor, version, and request id")
	}
	batch, err := s.Transition(ctx, cmd.TeamID, cmd.BatchID, cmd.ExpectedVersion, domain.GenerationRolledBack)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		if err := s.audit.AppendGenerationAudit(ctx, GenerationAuditEvent{TeamID: cmd.TeamID, BatchID: cmd.BatchID, ActorID: cmd.ActorID, StateVersion: batch.StateVersion, Action: "generation.rollback", RequestID: cmd.RequestID, OccurredAt: s.now()}); err != nil {
			return nil, fmt.Errorf("append generation rollback audit: %w", err)
		}
	}
	if s.signals != nil {
		if err := s.signals.SignalRollback(ctx, GenerationSignal{TeamID: cmd.TeamID, BatchID: cmd.BatchID, ActorID: cmd.ActorID, StateVersion: batch.StateVersion, RequestID: cmd.RequestID}); err != nil {
			return nil, fmt.Errorf("signal generation rollback: %w", err)
		}
	}
	return batch, nil
}
