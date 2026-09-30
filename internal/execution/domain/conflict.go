package domain

import "context"

// ResourceClaim describes a resource reservation that may conflict with
// another run. Claims are intentionally separate from quota: quota limits
// quantities, while conflict detection protects exclusive scopes.
type ResourceClaim struct {
	TeamID       int64
	TargetID     int64
	ResourceType string
	PoolID       string
	OwnerID      int64
	Exclusive    bool
}

// ConflictPort is the scheduling boundary for mutually exclusive resources.
// Implementations must make Acquire atomic and return a stable token that is
// required for release, so one run cannot release another run's claim.
type ConflictPort interface {
	Acquire(context.Context, ResourceClaim) (string, error)
	Release(context.Context, string) error
}

// NoopConflictPort keeps single-process/P1 execution backward compatible.
// Deployments that schedule runs must inject a real implementation.
type NoopConflictPort struct{}

func (NoopConflictPort) Acquire(context.Context, ResourceClaim) (string, error) { return "", nil }
func (NoopConflictPort) Release(context.Context, string) error                  { return nil }

var _ ConflictPort = NoopConflictPort{}
