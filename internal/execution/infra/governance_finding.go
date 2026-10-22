// GovernanceFindingPort bridges execution triage into the governance finding
// application service. The trusted/audit boundaries remain in governance.
package infra

import (
	"context"

	execapp "github.com/openware-io/open-green-pass/internal/execution/application"
	governanceapp "github.com/openware-io/open-green-pass/internal/governance/application"
	governancedomain "github.com/openware-io/open-green-pass/internal/governance/domain"
)

// GovernanceFindingPort implements execution's FindingPort.
type GovernanceFindingPort struct {
	svc *governanceapp.FindingService
}

// NewGovernanceFindingPort creates the adapter.
func NewGovernanceFindingPort(svc *governanceapp.FindingService) *GovernanceFindingPort {
	return &GovernanceFindingPort{svc: svc}
}

// CreateFinding maps the execution draft to the governance create command.
func (p *GovernanceFindingPort) CreateFinding(ctx context.Context, draft execapp.FindingDraft) (int64, error) {
	finding, _, err := p.svc.Create(ctx, governanceapp.CreateFindingInput{
		ProjectID: draft.ProjectID, TargetID: &draft.TargetID, RepoName: draft.RepoName,
		RepoURL: draft.RepoURL, Branch: draft.Branch, CommitSHA: draft.CommitSHA,
		Module: draft.Module, Endpoint: draft.Endpoint, Title: draft.Title,
		Severity: governancedomain.Severity(draft.Severity), Category: draft.Category,
		Expected: draft.Expected, Actual: draft.Actual, RootCause: draft.RootCause,
		Impact: draft.Impact, Confidence: governancedomain.Confidence(draft.Confidence),
		SourceRef: map[string]any{"file": draft.SourceFile, "line": draft.SourceLine},
		Evidence: governancedomain.FindingEvidence{
			Request: draft.Request, Response: draft.Response, Log: draft.Log, Trace: draft.Trace,
		},
		CoverageCells: draft.Coverage, LinkedRunID: &draft.LinkedRunID, LinkedCaseID: &draft.LinkedCaseID,
	})
	if err != nil {
		return 0, err
	}
	return finding.ID, nil
}
