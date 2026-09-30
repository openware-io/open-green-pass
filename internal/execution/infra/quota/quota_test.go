package quota

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

func req(team, target, owner int64, units int) domain.ResourceRequest {
	return domain.ResourceRequest{TeamID: team, TargetID: target, OwnerID: owner, Type: "browser", Units: units}
}

func TestMemoryQuotaEnforcesAllLevelsAtomically(t *testing.T) {
	q := NewMemoryQuota(map[string]Limits{"browser": {Global: 3, Team: 2, Target: 1, Owner: 1}})
	if err := q.Acquire(context.Background(), req(1, 10, 100, 1)); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(q.Acquire(context.Background(), req(1, 10, 101, 1)), ErrQuotaExceeded) {
		t.Fatal("target limit not enforced")
	}
	if err := q.Acquire(context.Background(), req(1, 11, 101, 1)); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(q.Acquire(context.Background(), req(1, 12, 102, 1)), ErrQuotaExceeded) {
		t.Fatal("team limit not enforced")
	}
	if err := q.Release(context.Background(), req(1, 10, 100, 1)); err != nil {
		t.Fatal(err)
	}
	if err := q.Acquire(context.Background(), req(2, 20, 200, 1)); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryQuotaConcurrentAcquireNeverOvershoots(t *testing.T) {
	q := NewMemoryQuota(map[string]Limits{"browser": {Global: 5}})
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if q.Acquire(context.Background(), req(int64(i+1), int64(i+1), int64(i+1), 1)) == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if success != 5 {
		t.Fatalf("successful acquires=%d, want 5", success)
	}
}

func TestFairQueueWeightedOrderAndFIFO(t *testing.T) {
	q := NewFairQueue()
	for _, item := range []QueueItem{{ID: "a1", TeamID: 1, Weight: 1}, {ID: "b1", TeamID: 2, Weight: 2}, {ID: "a2", TeamID: 1, Weight: 1}, {ID: "b2", TeamID: 2, Weight: 2}} {
		if err := q.Enqueue(item); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for q.Len() > 0 {
		i, _ := q.Dequeue()
		got = append(got, i.ID)
	}
	if len(got) != 4 || got[0] != "b1" || got[1] != "a1" || got[2] != "b2" || got[3] != "a2" {
		t.Fatalf("order=%v", got)
	}
}
