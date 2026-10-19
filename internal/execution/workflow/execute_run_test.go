package workflow

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

func TestExecuteRunWorkflowRunsAllStagesAfterVersionCheck(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	input := ExecuteRunInput{RunID: 41, TeamID: 7}
	env.OnActivity(ActivityRunVersionCheck, mock.Anything, input.TeamID, input.RunID).
		Return(&VersionCheckResult{Ok: true}, nil).Once()
	env.OnActivity(ActivityExecuteRun, mock.Anything, input.TeamID, input.RunID).
		Return(&ExecuteResult{State: domain.RunDone, PassCount: 2, TotalCount: 3}, nil).Once()

	env.ExecuteWorkflow(ExecuteRunWorkflow, input)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error = %v", err)
	}
	var result ExecuteRunResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("workflow result: %v", err)
	}
	if result.State != domain.RunDone || result.PassCount != 2 || result.TotalCount != 3 {
		t.Fatalf("workflow result = %+v", result)
	}
	env.AssertExpectations(t)
}

func TestExecuteRunWorkflowStopsWhenVersionCheckFails(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	input := ExecuteRunInput{RunID: 42, TeamID: 7}
	env.OnActivity(ActivityRunVersionCheck, mock.Anything, input.TeamID, input.RunID).
		Return(&VersionCheckResult{Ok: false}, nil).Once()
	env.OnActivity(ActivityMarkRunFailed, mock.Anything, input.TeamID, input.RunID).
		Return(nil).Once()

	env.ExecuteWorkflow(ExecuteRunWorkflow, input)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error = %v", err)
	}
	var result ExecuteRunResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("workflow result: %v", err)
	}
	if result.State != domain.RunFailed {
		t.Fatalf("workflow state = %s, want %s", result.State, domain.RunFailed)
	}
	env.AssertExpectations(t)
}
