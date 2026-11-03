package application

import "context"

// WorkflowController is the small boundary used by HTTP/application code to
// control a durable execution workflow without importing the Temporal SDK.
//
// Cancel must be idempotent: cancelling a run whose workflow never started (or
// has already closed) is not an error, because the run row is the source of
// truth and the user's stop request must always win.
type WorkflowController interface {
	Signal(ctx context.Context, runID int64, teamID int64, signal string) error
	Cancel(ctx context.Context, runID int64, teamID int64) error
}

type NoopWorkflowController struct{}

func (NoopWorkflowController) Signal(context.Context, int64, int64, string) error { return nil }
func (NoopWorkflowController) Cancel(context.Context, int64, int64) error         { return nil }
