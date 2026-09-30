package pool

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

func browserClaim(owner int64) domain.ResourceClaim {
	return domain.ResourceClaim{TeamID: 100, TargetID: 1003, ResourceType: "browser", PoolID: "playwright-a", OwnerID: owner, Exclusive: true}
}

func TestConflictRegistryExclusiveClaimAndRelease(t *testing.T) {
	r := NewConflictRegistry()
	first, err := r.Acquire(context.Background(), browserClaim(1))
	if err != nil || first == "" {
		t.Fatalf("first acquire token=%q err=%v", first, err)
	}
	if _, err := r.Acquire(context.Background(), browserClaim(2)); !errors.Is(err, ErrResourceConflict) {
		t.Fatalf("second acquire err=%v, want conflict", err)
	}
	if err := r.Release(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Acquire(context.Background(), browserClaim(2)); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
}

func TestConflictRegistryAllowsNonExclusiveClaims(t *testing.T) {
	r := NewConflictRegistry()
	claim := browserClaim(1)
	claim.Exclusive = false
	if token, err := r.Acquire(context.Background(), claim); err != nil || token != "" {
		t.Fatalf("non-exclusive acquire token=%q err=%v", token, err)
	}
}

func TestConflictRegistryConcurrentAcquireHasOneWinner(t *testing.T) {
	r := NewConflictRegistry()
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	for owner := int64(1); owner <= 32; owner++ {
		wg.Add(1)
		go func(owner int64) {
			defer wg.Done()
			if token, err := r.Acquire(context.Background(), browserClaim(owner)); err == nil && token != "" {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}(owner)
	}
	wg.Wait()
	if winners != 1 {
		t.Fatalf("winners=%d, want 1", winners)
	}
}
