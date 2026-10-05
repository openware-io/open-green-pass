package infra

import (
	"context"
	"sync"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// MemoryGenerationBatchRepository is a deterministic fake for application and
// workflow-adjacent tests. Production code must use a PostgreSQL repository
// with the same idempotency and compare-and-swap semantics.
type MemoryGenerationBatchRepository struct {
	mu        sync.Mutex
	byID      map[generationBatchKey]*domain.GenerationBatch
	byRequest map[generationRequestKey]int64
}

type generationBatchKey struct{ teamID, batchID int64 }
type generationRequestKey struct {
	teamID int64
	key    string
}

func NewMemoryGenerationBatchRepository() *MemoryGenerationBatchRepository {
	return &MemoryGenerationBatchRepository{
		byID:      make(map[generationBatchKey]*domain.GenerationBatch),
		byRequest: make(map[generationRequestKey]int64),
	}
}

func (r *MemoryGenerationBatchRepository) Create(_ context.Context, batch *domain.GenerationBatch) error {
	if batch == nil {
		return domain.ErrGenerationNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := generationBatchKey{batch.TeamID, batch.ID}
	requestKey := generationRequestKey{batch.TeamID, batch.IdempotencyKey}
	if _, exists := r.byID[key]; exists {
		return domain.ErrGenerationStateConflict
	}
	if _, exists := r.byRequest[requestKey]; exists {
		return domain.ErrGenerationStateConflict
	}
	r.byID[key] = cloneGenerationBatch(batch)
	r.byRequest[requestKey] = batch.ID
	return nil
}

func (r *MemoryGenerationBatchRepository) Find(_ context.Context, teamID, batchID int64) (*domain.GenerationBatch, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	batch, ok := r.byID[generationBatchKey{teamID, batchID}]
	if !ok {
		return nil, domain.ErrGenerationNotFound
	}
	return cloneGenerationBatch(batch), nil
}

func (r *MemoryGenerationBatchRepository) FindByIdempotencyKey(_ context.Context, teamID int64, key string) (*domain.GenerationBatch, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byRequest[generationRequestKey{teamID, key}]
	if !ok {
		return nil, domain.ErrGenerationNotFound
	}
	return cloneGenerationBatch(r.byID[generationBatchKey{teamID, id}]), nil
}

func (r *MemoryGenerationBatchRepository) CompareAndSwap(_ context.Context, batch *domain.GenerationBatch, expectedVersion int64) error {
	if batch == nil {
		return domain.ErrGenerationNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := generationBatchKey{batch.TeamID, batch.ID}
	current, ok := r.byID[key]
	if !ok {
		return domain.ErrGenerationNotFound
	}
	if current.StateVersion != expectedVersion || batch.StateVersion != expectedVersion+1 {
		return domain.ErrGenerationStateConflict
	}
	r.byID[key] = cloneGenerationBatch(batch)
	return nil
}

func cloneGenerationBatch(in *domain.GenerationBatch) *domain.GenerationBatch {
	if in == nil {
		return nil
	}
	out := *in
	if in.ParentBatchID != nil {
		parent := *in.ParentBatchID
		out.ParentBatchID = &parent
	}
	return &out
}
