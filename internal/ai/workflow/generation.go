// Package workflow contains only deterministic Temporal orchestration for W1.
// Activities own all DB, SCM, AI-provider, and governance I/O.
package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	ReviewSignal   = "generation-review"
	RollbackSignal = "generation-rollback"
)

// GenerationPipelineInput contains stable identifiers only. Activities load
// source and batch details through their injected application dependencies.
type GenerationPipelineInput struct {
	BatchID int64
	TeamID  int64
}

// ReviewSignalPayload is intentionally metadata-only. The HTTP/application
// command performs authorization and optimistic state checks before signaling.
type ReviewSignalPayload struct {
	BatchID      int64
	Decision     string // approve or reject
	ActorID      int64
	RequestID    string
	StateVersion int64
}

type RollbackSignalPayload struct {
	BatchID      int64
	ActorID      int64
	RequestID    string
	StateVersion int64
}

// BatchActivityInput deliberately separates activity names from implementations
// so worker wiring may register retry-safe adapters without exposing I/O in the
// workflow package.
type BatchActivityInput struct {
	BatchID int64
	TeamID  int64
}

// GenerationPipelineWorkflow runs the non-human stages, then waits durably for
// one review or rollback signal. It must not access a DB, HTTP client, or AI
// provider directly.
func GenerationPipelineWorkflow(ctx workflow.Context, input GenerationPipelineInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	activityInput := BatchActivityInput{BatchID: input.BatchID, TeamID: input.TeamID}
	for _, activityName := range []string{"FetchSource", "ParseAndNormalize", "GenerateSeeds", "QualityGate"} {
		if err := workflow.ExecuteActivity(ctx, activityName, activityInput).Get(ctx, nil); err != nil {
			return err
		}
	}

	reviewCh := workflow.GetSignalChannel(ctx, ReviewSignal)
	rollbackCh := workflow.GetSignalChannel(ctx, RollbackSignal)
	selector := workflow.NewSelector(ctx)
	var review ReviewSignalPayload
	var rollback RollbackSignalPayload
	approved := false
	selector.AddReceive(reviewCh, func(channel workflow.ReceiveChannel, more bool) {
		channel.Receive(ctx, &review)
		approved = review.Decision == "approve"
	})
	selector.AddReceive(rollbackCh, func(channel workflow.ReceiveChannel, more bool) {
		channel.Receive(ctx, &rollback)
	})
	selector.Select(ctx)

	if rollback.BatchID != 0 {
		return workflow.ExecuteActivity(ctx, "ApplyRollback", rollback).Get(ctx, nil)
	}
	if !approved {
		return workflow.ExecuteActivity(ctx, "RecordReview", review).Get(ctx, nil)
	}
	if err := workflow.ExecuteActivity(ctx, "CommitVersions", review).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, "EmitAudit", review).Get(ctx, nil)
}
