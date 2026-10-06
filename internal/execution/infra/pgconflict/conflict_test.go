package pgconflict

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

func TestLockKeyUsesConflictScopeNotOwner(t *testing.T) {
	first := validClaim()
	second := first
	second.OwnerID++
	if lockKey(first) != lockKey(second) {
		t.Fatal("owners in the same resource scope must contend for one lock")
	}
	second.TargetID++
	if lockKey(first) == lockKey(second) {
		t.Fatal("different target scopes must use different locks")
	}
}

func TestValidateClaim(t *testing.T) {
	claim := validClaim()
	if err := validateClaim(context.Background(), claim); err != nil {
		t.Fatalf("valid claim: %v", err)
	}
	claim.OwnerID = 0
	if err := validateClaim(context.Background(), claim); !errors.Is(err, ErrInvalidClaim) {
		t.Fatalf("error=%v, want invalid claim", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateClaim(canceled, validClaim()); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want context canceled", err)
	}
}

func TestNewRequiresPool(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected nil pool error")
	}
}

func TestRegistryExcludesAcrossPools(t *testing.T) {
	dsn := os.Getenv("GP_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("GP_TEST_DB_DSN is not configured")
	}
	ctx := context.Background()
	firstPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("first pool: %v", err)
	}
	defer firstPool.Close()
	secondPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("second pool: %v", err)
	}
	defer secondPool.Close()
	first, err := New(firstPool)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(secondPool)
	if err != nil {
		t.Fatal(err)
	}
	token, err := first.Acquire(ctx, validClaim())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := second.Acquire(ctx, validClaim()); !errors.Is(err, domain.ErrResourceConflict) {
		t.Fatalf("second acquire error=%v, want resource conflict", err)
	}
	if err := first.Release(ctx, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	secondToken, err := second.Acquire(ctx, validClaim())
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := second.Release(ctx, secondToken); err != nil {
		t.Fatalf("second release: %v", err)
	}
}

func validClaim() domain.ResourceClaim {
	return domain.ResourceClaim{TeamID: 1001, TargetID: 2001, ResourceType: "target", PoolID: "default", OwnerID: 3001, Exclusive: true}
}
