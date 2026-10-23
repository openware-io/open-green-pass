package workflow

import (
	"context"
	"errors"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	"github.com/openware-io/open-green-pass/internal/ai/application"
)

// TemporalController is the AI generation workflow adapter used by the
// application service. It owns only workflow start/signal operations.
type TemporalController struct{ client client.Client }

func NewTemporalController(c client.Client) *TemporalController {
	return &TemporalController{client: c}
}

func (c *TemporalController) Start(ctx context.Context, batchID, teamID int64) error {
	_, err := c.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID: workflowID(teamID, batchID), TaskQueue: "gp-generation",
	}, GenerationPipelineWorkflow, GenerationPipelineInput{BatchID: batchID, TeamID: teamID})
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	return err
}

func (c *TemporalController) SignalReview(ctx context.Context, signal application.GenerationSignal) error {
	return c.client.SignalWorkflow(ctx, workflowID(signal.TeamID, signal.BatchID), "", ReviewSignal, ReviewSignalPayload{TeamID: signal.TeamID, BatchID: signal.BatchID, Decision: string(signal.Decision), ActorID: signal.ActorID, RequestID: signal.RequestID, StateVersion: signal.StateVersion})
}

func (c *TemporalController) SignalRollback(ctx context.Context, signal application.GenerationSignal) error {
	return c.client.SignalWorkflow(ctx, workflowID(signal.TeamID, signal.BatchID), "", RollbackSignal, RollbackSignalPayload{TeamID: signal.TeamID, BatchID: signal.BatchID, ActorID: signal.ActorID, RequestID: signal.RequestID, StateVersion: signal.StateVersion})
}

// Cancel requests Temporal cancellation; a workflow that already finished is
// treated as successfully cancelled for cleanup purposes.
func (c *TemporalController) Cancel(ctx context.Context, signal application.GenerationSignal) error {
	err := c.client.CancelWorkflow(ctx, workflowID(signal.TeamID, signal.BatchID), "")
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return nil
	}
	return err
}

func workflowID(teamID, batchID int64) string {
	return "generation-" + itoa(teamID) + "-" + itoa(batchID)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
