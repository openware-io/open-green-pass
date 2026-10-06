package domain

import "context"

// ResourceRequest is the normalized resource claim made by a run.
// Units are opaque to the domain (one browser/device/sandbox slot is 1).
type ResourceRequest struct {
	TeamID   int64
	TargetID int64
	OwnerID  int64
	Type     string
	Units    int
}

// QuotaPort is the execution scheduling boundary. Implementations must make
// Acquire atomic across team, target and owner limits.
type QuotaPort interface {
	Acquire(context.Context, ResourceRequest) error
	Release(context.Context, ResourceRequest) error
}

// NoopQuotaPort keeps quota optional at composition roots that do not dispatch
// work. Production dispatchers should replace it with a shared implementation.
type NoopQuotaPort struct{}

func (NoopQuotaPort) Acquire(context.Context, ResourceRequest) error { return nil }
func (NoopQuotaPort) Release(context.Context, ResourceRequest) error { return nil }
