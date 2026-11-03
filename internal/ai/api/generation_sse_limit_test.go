package api

import (
	"context"
	"testing"
)

func TestMemoryGenerationSSELimiterBoundsPerTenant(t *testing.T) {
	limiter := NewMemoryGenerationSSELimiter(2)
	ctx := context.Background()
	if ok, err := limiter.Acquire(ctx, 1001, "lease-a"); err != nil || !ok {
		t.Fatal("first stream for a tenant must be admitted")
	}
	if ok, err := limiter.Acquire(ctx, 1001, "lease-b"); err != nil || !ok {
		t.Fatal("second stream for a tenant must be admitted")
	}
	if ok, err := limiter.Acquire(ctx, 1001, "lease-c"); err != nil || ok {
		t.Fatalf("third stream for the same tenant must be rejected: ok=%v err=%v", ok, err)
	}
	if ok, _ := limiter.Acquire(ctx, 1002, "lease-a"); !ok {
		t.Fatal("budget must be isolated per tenant")
	}
	if err := limiter.Release(ctx, 1001, "lease-a"); err != nil {
		t.Fatal(err)
	}
	// Releasing one lease must not free the other live lease.
	if ok, _ := limiter.Acquire(ctx, 1001, "lease-c"); !ok {
		t.Fatal("released capacity must be reusable")
	}
	if limiter.count(1001) != 2 {
		t.Fatalf("active count = %d, want 2", limiter.count(1001))
	}
	// Release is lease-specific: removing an unknown lease is a no-op.
	if err := limiter.Release(ctx, 1001, "missing-lease"); err != nil {
		t.Fatal(err)
	}
	if limiter.count(1001) != 2 {
		t.Fatalf("unknown release changed active count: %d", limiter.count(1001))
	}
	if err := limiter.Renew(ctx, 1001, "lease-b"); err != nil {
		t.Fatalf("memory renew must be a no-op: %v", err)
	}
}

func TestMemoryGenerationSSELimiterDefaults(t *testing.T) {
	limiter := NewMemoryGenerationSSELimiter(0)
	if limiter.MaxLiveStreams() != 8 {
		t.Fatalf("default max = %d, want 8", limiter.MaxLiveStreams())
	}
}

func TestNewGenerationSSELeaseIDIsUnique(t *testing.T) {
	seen := make(map[string]struct{}, 128)
	for i := 0; i < 128; i++ {
		leaseID := newGenerationSSELeaseID()
		if leaseID == "" {
			t.Fatal("empty lease id")
		}
		if _, exists := seen[leaseID]; exists {
			t.Fatalf("duplicate lease id %q", leaseID)
		}
		seen[leaseID] = struct{}{}
	}
}
