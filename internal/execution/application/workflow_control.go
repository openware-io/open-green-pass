package application

import "context"

// WorkflowController is the small boundary used by HTTP/application code to
// control a durable execution workflow without importing the Temporal SDK.
type WorkflowController interface {
	Signal(ctx context.Context, runID int64, teamID int64, signal string) error
}

type NoopWorkflowController struct{}

func (NoopWorkflowController) Signal(context.Context, int64, int64, string) error { return nil }
