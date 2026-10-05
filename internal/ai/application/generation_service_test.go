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
