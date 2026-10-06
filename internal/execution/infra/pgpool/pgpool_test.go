package pgpool

import (
	"context"
	"errors"
	"testing"
	"time"

	resourcepool "github.com/openware-io/open-green-pass/internal/execution/infra/pool"
)

func TestNewRequiresPool(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected nil postgres pool error")
	}
}

func TestValidateRegistration(t *testing.T) {
	valid := resourcepool.Registration{ID: "browser", Cluster: "kind", Resources: []string{"browser"}, Capacity: 2, Available: 2, Enabled: true}
	if err := validateRegistration(valid); err != nil {
		t.Fatal(err)
	}
	valid.Available = 3
	if !errors.Is(validateRegistration(valid), resourcepool.ErrPoolUnavailable) {
		t.Fatal("capacity overflow must be rejected")
	}
}

func TestHeartbeatArgumentsRejectStaleInputs(t *testing.T) {
	r := &Registry{}
	if err := r.Heartbeat(context.Background(), "", 1, time.Now()); !errors.Is(err, resourcepool.ErrInvalidHeartbeat) {
		t.Fatalf("err=%v", err)
	}
	if err := r.Heartbeat(context.Background(), "pool", 0, time.Now()); !errors.Is(err, resourcepool.ErrInvalidHeartbeat) {
		t.Fatalf("err=%v", err)
	}
}

func TestLeaseReleaseRejectsMissingFencingToken(t *testing.T) {
	r := &Registry{}
	if err := r.Release(context.Background(), Lease{Token: "token", PoolID: "pool"}); !errors.Is(err, ErrStaleLease) {
		t.Fatalf("err=%v", err)
	}
}
