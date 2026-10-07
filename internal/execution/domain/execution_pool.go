package domain

import (
	"context"
	"time"
)

// ExecutionPoolRequest describes shared capacity required by a run.
type ExecutionPoolRequest struct {
	Resource string
	Units    int
}

// ExecutionPoolLease identifies a fenced reservation in a selected pool.
type ExecutionPoolLease struct {
	Token        string
	PoolID       string
	Units        int
	FencingToken int64
	ExpiresAt    time.Time
}

// ExecutionPoolPort selects and releases shared runner capacity.
type ExecutionPoolPort interface {
	Reserve(context.Context, ExecutionPoolRequest) (ExecutionPoolLease, error)
	Renew(context.Context, ExecutionPoolLease) (ExecutionPoolLease, error)
	Release(context.Context, ExecutionPoolLease) error
}
