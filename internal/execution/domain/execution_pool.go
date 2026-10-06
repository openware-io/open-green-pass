package domain

import "context"

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
}

// ExecutionPoolPort selects and releases shared runner capacity.
type ExecutionPoolPort interface {
	Reserve(context.Context, ExecutionPoolRequest) (ExecutionPoolLease, error)
	Release(context.Context, ExecutionPoolLease) error
}
