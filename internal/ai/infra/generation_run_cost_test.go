package infra

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

type fakeGenerationCostReadPort struct {
	calls    int
	batchIDs []int64
	costs    map[int64]float64
	err      error
}

func (f *fakeGenerationCostReadPort) BatchCosts(_ context.Context, _ int64, batchIDs []int64) (map[int64]float64, error) {
	f.calls++
	f.batchIDs = append([]int64(nil), batchIDs...)
	if f.err != nil {
		return nil, f.err
	}
	return f.costs, nil
}

func batchID(value int64) *int64 { return &value }

func TestGenerationRunStoreAppliesBatchedBatchCosts(t *testing.T) {
	store := NewGenerationRunStore(nil)
	port := &fakeGenerationCostReadPort{costs: map[int64]float64{11: 0.1, 12: 0.2, 13: 0.3}}
	store.SetCostReadPort(port)
	first := &domain.GenerationRun{ID: 1, Units: []*domain.GenerationUnit{{ID: 1, BatchID: batchID(11)}, {ID: 2, BatchID: batchID(12)}}}
	second := &domain.GenerationRun{ID: 2, Units: []*domain.GenerationUnit{{ID: 3, BatchID: batchID(12)}, {ID: 4, BatchID: batchID(13)}}}
	if err := store.applyRunCosts(context.Background(), 9, []*domain.GenerationRun{first, second}); err != nil {
		t.Fatal(err)
	}
	if port.calls != 1 {
		t.Fatalf("cost port calls = %d, want one batched call", port.calls)
	}
	if len(port.batchIDs) != 3 {
		t.Fatalf("batch ids = %v, want 3 distinct ids", port.batchIDs)
	}
	if first.CostAmount < 0.299 || first.CostAmount > 0.301 {
		t.Fatalf("first run cost = %v, want 0.3", first.CostAmount)
	}
	if second.CostAmount < 0.499 || second.CostAmount > 0.501 {
		t.Fatalf("second run cost = %v, want 0.5", second.CostAmount)
	}
}

func TestGenerationRunStoreWithoutCostPortLeavesZeroCost(t *testing.T) {
	store := NewGenerationRunStore(nil)
	run := &domain.GenerationRun{ID: 1, Units: []*domain.GenerationUnit{{ID: 1, BatchID: batchID(11)}}}
	if err := store.applyRunCosts(context.Background(), 9, []*domain.GenerationRun{run}); err != nil {
		t.Fatal(err)
	}
	if run.CostAmount != 0 {
		t.Fatalf("cost without port = %v, want 0", run.CostAmount)
	}
}
