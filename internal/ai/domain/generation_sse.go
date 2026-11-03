package domain

import (
	"context"
	"errors"
)

// ErrGenerationSSELeaseLost means the limiter no longer knows this lease (its
// score expired or the key was reclaimed). Callers must re-acquire instead of
// silently assuming the stream is still counted.
var ErrGenerationSSELeaseLost = errors.New("generation sse lease lost")

// GenerationSSELimiter bounds concurrent SSE streams per tenant across all
// server replicas. Each stream owns a unique lease ID: Acquire admits it,
// Renew extends its expiry, and Release removes it. Implementations with a
// lease TTL reclaim the slots of a crashed server replica automatically.
type GenerationSSELimiter interface {
	Acquire(ctx context.Context, teamID int64, leaseID string) (bool, error)
	Release(ctx context.Context, teamID int64, leaseID string) error
	Renew(ctx context.Context, teamID int64, leaseID string) error
}
