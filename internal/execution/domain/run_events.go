package domain

import "context"

// RunEventSubscription is a process-local view of shared run notifications.
// Notifications are hints only; consumers must reload the authoritative run.
type RunEventSubscription interface {
	Events() <-chan struct{}
	Close() error
}

// RunEventBus wakes SSE connections across server replicas after a run write.
type RunEventBus interface {
	Publish(ctx context.Context, teamID, runID int64) error
	Subscribe(ctx context.Context, teamID, runID int64) (RunEventSubscription, error)
}
