package infra

import (
	"context"
	"testing"
)

type fakeBatchCostReader struct {
	teamID  int64
	batchID []int64
	result  map[int64]float64
	calls   int
}

func (f *fakeBatchCostReader) BatchCosts(_ context.Context, teamID int64, batchIDs []int64) (map[int64]float64, error) {
	f.calls++
	f.teamID = teamID
	f.batchID = append([]int64(nil), batchIDs...)
	return f.result, nil
}

func TestGenerationCostReadAdapterDelegatesProviderContract(t *testing.T) {
	reader := &fakeBatchCostReader{result: map[int64]float64{11: 0.25}}
	adapter := NewGenerationCostReadAdapter(reader)
	result, err := adapter.BatchCosts(context.Background(), 7, []int64{11, 12})
	if err != nil {
		t.Fatal(err)
	}
	if result[11] != 0.25 {
		t.Fatalf("batch cost = %+v", result)
	}
	if reader.calls != 1 || reader.teamID != 7 || len(reader.batchID) != 2 {
		t.Fatalf("provider call = %d team=%d ids=%v", reader.calls, reader.teamID, reader.batchID)
	}
}
