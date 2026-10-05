package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGenerationBatchStateMachine(t *testing.T) {
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	b, err := NewGenerationBatch(1, 10, 20, 30, 40, "main", "v1", "req-1", now)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []GenerationBatchState{GenerationFetching, GenerationParsing, GenerationGenerating, GenerationQualityCheck, GenerationReview, GenerationCommitting, GenerationApproved} {
		if err := b.Transition(state, b.StateVersion, now); err != nil {
			t.Fatalf("transition to %s: %v", state, err)
		}
	}
	if b.StateVersion != 8 {
		t.Fatalf("state version = %d, want 8", b.StateVersion)
	}
	if err := b.Transition(GenerationFetching, b.StateVersion, now); !errors.Is(err, ErrInvalidGenerationTransition) {
		t.Fatalf("terminal transition error = %v", err)
	}
}

func TestGenerationBatchRejectAndRollbackPaths(t *testing.T) {
	now := time.Now().UTC()
	b, _ := NewGenerationBatch(1, 1, 1, 1, 1, "main", "", "k", now)
	for _, state := range []GenerationBatchState{GenerationFetching, GenerationParsing, GenerationGenerating, GenerationQualityCheck, GenerationReview} {
		if err := b.Transition(state, b.StateVersion, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Transition(GenerationRejected, b.StateVersion, now); err != nil {
		t.Fatal(err)
	}
	if err := b.Transition(GenerationFailed, b.StateVersion, now); !errors.Is(err, ErrInvalidGenerationTransition) {
		t.Fatalf("rejected should be terminal: %v", err)
	}

	approved, _ := NewGenerationBatch(2, 1, 1, 1, 1, "main", "", "k2", now)
	for _, state := range []GenerationBatchState{GenerationFetching, GenerationParsing, GenerationGenerating, GenerationQualityCheck, GenerationReview, GenerationCommitting, GenerationApproved} {
		if err := approved.Transition(state, approved.StateVersion, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := approved.Transition(GenerationRolledBack, approved.StateVersion, now); err != nil {
		t.Fatal(err)
	}
}

func TestGenerationBatchRepositoryContractIsContextBound(t *testing.T) {
	var _ GenerationBatchRepository = contextRepositoryProbe{}
}

type contextRepositoryProbe struct{}

func (contextRepositoryProbe) Create(context.Context, *GenerationBatch) error { return nil }
func (contextRepositoryProbe) Find(context.Context, int64, int64) (*GenerationBatch, error) {
	return nil, ErrGenerationNotFound
}
func (contextRepositoryProbe) FindByIdempotencyKey(context.Context, int64, string) (*GenerationBatch, error) {
	return nil, ErrGenerationNotFound
}
func (contextRepositoryProbe) CompareAndSwap(context.Context, *GenerationBatch, int64) error {
	return nil
}
