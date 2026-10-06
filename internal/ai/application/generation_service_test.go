package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/ai/infra"
)

func TestGenerationBatchCreateIsIdempotentPerTenant(t *testing.T) {
	repo := infra.NewMemoryGenerationBatchRepository()
	svc := NewGenerationBatchService(repo)
	svc.now = func() time.Time { return time.Unix(100, 0).UTC() }
	cmd := CreateGenerationBatchCommand{ID: 1, TeamID: 10, TargetID: 20, RepoID: 30, Branch: "main", IdempotencyKey: "same", ActorID: 40}
	first, created, err := svc.Create(context.Background(), cmd)
	if err != nil || !created {
		t.Fatalf("first create = %#v, %v, %v", first, created, err)
	}
	cmd.ID = 2
	second, created, err := svc.Create(context.Background(), cmd)
	if err != nil || created || second.ID != first.ID {
		t.Fatalf("repeat create = %#v, %v, %v", second, created, err)
	}
	cmd.TeamID = 11
	cmd.ID = 2
	third, created, err := svc.Create(context.Background(), cmd)
	if err != nil || !created || third.ID != 2 {
		t.Fatalf("cross-tenant create = %#v, %v, %v", third, created, err)
	}
}

func TestGenerationBatchTransitionUsesOptimisticVersion(t *testing.T) {
	repo := infra.NewMemoryGenerationBatchRepository()
	svc := NewGenerationBatchService(repo)
	batch, _, err := svc.Create(context.Background(), CreateGenerationBatchCommand{ID: 1, TeamID: 1, TargetID: 1, RepoID: 1, Branch: "main", IdempotencyKey: "k", ActorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Transition(context.Background(), 1, batch.ID, 99, domain.GenerationFetching); !errors.Is(err, domain.ErrGenerationStateConflict) {
		t.Fatalf("stale version error = %v", err)
	}
	updated, err := svc.Transition(context.Background(), 1, batch.ID, 1, domain.GenerationFetching)
	if err != nil || updated.State != domain.GenerationFetching || updated.StateVersion != 2 {
		t.Fatalf("transition = %#v, %v", updated, err)
	}
}

func TestGenerationReviewSignalsAndAuditsApprovedTransition(t *testing.T) {
	repo := infra.NewMemoryGenerationBatchRepository()
	svc := NewGenerationBatchService(repo)
	ports := &generationReviewPorts{}
	svc.SetReviewPorts(ports, ports)
	batch, _, err := svc.Create(context.Background(), CreateGenerationBatchCommand{ID: 1, TeamID: 1, TargetID: 1, RepoID: 1, Branch: "main", IdempotencyKey: "review", ActorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []domain.GenerationBatchState{domain.GenerationFetching, domain.GenerationParsing, domain.GenerationGenerating, domain.GenerationQualityCheck, domain.GenerationReview} {
		batch, err = svc.Transition(context.Background(), 1, batch.ID, batch.StateVersion, state)
		if err != nil {
			t.Fatal(err)
		}
	}
	batch, err = svc.Review(context.Background(), ReviewGenerationCommand{TeamID: 1, BatchID: 1, ActorID: 9, ExpectedVersion: batch.StateVersion, Decision: GenerationReviewApprove, RequestID: "request-1"})
	if err != nil || batch.State != domain.GenerationCommitting {
		t.Fatalf("review = %#v, %v", batch, err)
	}
	if len(ports.events) != 1 || ports.events[0].Action != "generation.review.approve" || len(ports.reviews) != 1 {
		t.Fatalf("ports = %#v", ports)
	}
	if _, err := svc.Review(context.Background(), ReviewGenerationCommand{TeamID: 1, BatchID: 1, ActorID: 9, ExpectedVersion: batch.StateVersion, Decision: GenerationReviewReject, RequestID: "request-2"}); !errors.Is(err, domain.ErrInvalidGenerationTransition) {
		t.Fatalf("second review error = %v", err)
	}
}

func TestGenerationRollbackRequiresApprovedBatch(t *testing.T) {
	repo := infra.NewMemoryGenerationBatchRepository()
	svc := NewGenerationBatchService(repo)
	ports := &generationReviewPorts{}
	svc.SetReviewPorts(ports, ports)
	batch, _, err := svc.Create(context.Background(), CreateGenerationBatchCommand{ID: 1, TeamID: 1, TargetID: 1, RepoID: 1, Branch: "main", IdempotencyKey: "rollback", ActorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rollback(context.Background(), RollbackGenerationCommand{TeamID: 1, BatchID: 1, ActorID: 9, ExpectedVersion: batch.StateVersion, RequestID: "request-1"}); !errors.Is(err, domain.ErrInvalidGenerationTransition) {
		t.Fatalf("premature rollback error = %v", err)
	}
	for _, state := range []domain.GenerationBatchState{domain.GenerationFetching, domain.GenerationParsing, domain.GenerationGenerating, domain.GenerationQualityCheck, domain.GenerationReview, domain.GenerationCommitting, domain.GenerationApproved} {
		batch, err = svc.Transition(context.Background(), 1, batch.ID, batch.StateVersion, state)
		if err != nil {
			t.Fatal(err)
		}
	}
	batch, err = svc.Rollback(context.Background(), RollbackGenerationCommand{TeamID: 1, BatchID: 1, ActorID: 9, ExpectedVersion: batch.StateVersion, RequestID: "request-2"})
	if err != nil || batch.State != domain.GenerationRolledBack || len(ports.rollbacks) != 1 || ports.events[len(ports.events)-1].Action != "generation.rollback" {
		t.Fatalf("rollback = %#v, ports = %#v, err = %v", batch, ports, err)
	}
}

type generationReviewPorts struct {
	reviews, rollbacks []GenerationSignal
	events             []GenerationAuditEvent
}

func (p *generationReviewPorts) SignalReview(_ context.Context, signal GenerationSignal) error {
	p.reviews = append(p.reviews, signal)
	return nil
}

func (p *generationReviewPorts) SignalRollback(_ context.Context, signal GenerationSignal) error {
	p.rollbacks = append(p.rollbacks, signal)
	return nil
}

func (p *generationReviewPorts) AppendGenerationAudit(_ context.Context, event GenerationAuditEvent) error {
	p.events = append(p.events, event)
	return nil
}
