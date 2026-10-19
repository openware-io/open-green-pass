package workflow

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestGenerationPipelineWorkflowWaitsForReviewThenCommits(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	input := GenerationPipelineInput{BatchID: 81, TeamID: 9}
	activityInput := BatchActivityInput{BatchID: input.BatchID, TeamID: input.TeamID}
	env.OnActivity(FetchSource, mock.Anything, activityInput).Return(nil).Once()
	env.OnActivity(ParseAndNormalize, mock.Anything, activityInput).Return(nil).Once()
	env.OnActivity(GenerateSeeds, mock.Anything, activityInput).Return(nil).Once()
	env.OnActivity(QualityGate, mock.Anything, activityInput).Return(nil).Once()
	payload := ReviewSignalPayload{BatchID: input.BatchID, TeamID: input.TeamID, Decision: "approve", ActorID: 12, RequestID: "review-81", StateVersion: 6}
	env.OnActivity(CommitVersions, mock.Anything, payload).Return(nil).Once()
	env.OnActivity(EmitAudit, mock.Anything, payload).Return(nil).Once()
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(ReviewSignal, payload) }, time.Second)

	env.ExecuteWorkflow(GenerationPipelineWorkflow, input)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error = %v", err)
	}
	env.AssertExpectations(t)
}

func TestGenerationPipelineWorkflowRecordsRejectionWithoutCommitting(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	input := GenerationPipelineInput{BatchID: 82, TeamID: 9}
	activityInput := BatchActivityInput{BatchID: input.BatchID, TeamID: input.TeamID}
	env.OnActivity(FetchSource, mock.Anything, activityInput).Return(nil).Once()
	env.OnActivity(ParseAndNormalize, mock.Anything, activityInput).Return(nil).Once()
	env.OnActivity(GenerateSeeds, mock.Anything, activityInput).Return(nil).Once()
	env.OnActivity(QualityGate, mock.Anything, activityInput).Return(nil).Once()
	payload := ReviewSignalPayload{BatchID: input.BatchID, TeamID: input.TeamID, Decision: "reject", ActorID: 12, RequestID: "review-82", StateVersion: 6}
	env.OnActivity(RecordReview, mock.Anything, payload).Return(nil).Once()
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(ReviewSignal, payload) }, time.Second)

	env.ExecuteWorkflow(GenerationPipelineWorkflow, input)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error = %v", err)
	}
	env.AssertExpectations(t)
}

func TestGenerationPipelineWorkflowMarksCommitFailureAsFailed(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	input := GenerationPipelineInput{BatchID: 83, TeamID: 9}
	activityInput := BatchActivityInput{BatchID: input.BatchID, TeamID: input.TeamID}
	env.OnActivity(FetchSource, mock.Anything, activityInput).Return(nil).Once()
	env.OnActivity(ParseAndNormalize, mock.Anything, activityInput).Return(nil).Once()
	env.OnActivity(GenerateSeeds, mock.Anything, activityInput).Return(nil).Once()
	env.OnActivity(QualityGate, mock.Anything, activityInput).Return(nil).Once()
	payload := ReviewSignalPayload{BatchID: input.BatchID, TeamID: input.TeamID, Decision: "approve", ActorID: 12, RequestID: "review-83", StateVersion: 6}
	env.OnActivity(CommitVersions, mock.Anything, payload).Return(errors.New("quality recheck failed")).Once()
	failureInput := activityInput
	failureInput.FailureStage = "CommitVersions"
	env.OnActivity(FailGeneration, mock.Anything, failureInput).Return(nil).Once()
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(ReviewSignal, payload) }, time.Second)

	env.ExecuteWorkflow(GenerationPipelineWorkflow, input)
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected approval commit to fail")
	}
	env.AssertExpectations(t)
}

func TestGenerationPipelineWorkflowRecordsSourceFailureStage(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	input := GenerationPipelineInput{BatchID: 84, TeamID: 9}
	activityInput := BatchActivityInput{BatchID: input.BatchID, TeamID: input.TeamID}
	env.OnActivity(FetchSource, mock.Anything, activityInput).Return(errors.New("repository inaccessible")).Times(3)
	failureInput := activityInput
	failureInput.FailureStage = "FetchSource"
	env.OnActivity(FailGeneration, mock.Anything, failureInput).Return(nil).Once()
	env.ExecuteWorkflow(GenerationPipelineWorkflow, input)
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected source fetch failure")
	}
	env.AssertExpectations(t)
}
