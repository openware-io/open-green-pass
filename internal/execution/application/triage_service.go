// Package application 执行失败归因：把 run_case_result 转为治理域 finding。
package application

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

// FindingDraft is the execution-to-governance boundary payload.
type FindingDraft struct {
	ProjectID    int64
	ServiceID    *int64
	TargetID     int64
	RepoName     string
	RepoURL      string
	Branch       string
	CommitSHA    string
	Module       string
	Endpoint     string
	Title        string
	Severity     string
	Category     string
	Expected     string
	Actual       string
	RootCause    string
	Impact       string
	Confidence   string
	SourceFile   string
	SourceLine   int
	Request      string
	Response     string
	Log          string
	Trace        string
	Coverage     []string
	LinkedRunID  int64
	LinkedCaseID int64
}

// FindingPort creates a governance finding from execution evidence.
type FindingPort interface {
	CreateFinding(ctx context.Context, draft FindingDraft) (int64, error)
}

// TriageInput is the HTTP/application input.
type TriageInput struct {
	CaseResultID int64
	ProjectID    int64
	TargetID     int64
	RepoName     string
	RepoURL      string
	Branch       string
	CommitSHA    string
	Module       string
	Endpoint     string
	Title        string
	Severity     string
	RootCause    string
	Impact       string
	Confidence   string
	SourceFile   string
	SourceLine   int
	Expected     string
	Actual       string
}

// Triage converts a failed result into a finding and links it back.
func (s *RunService) Triage(ctx context.Context, runID int64, in TriageInput) (*domain.CaseResult, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	if s.finding == nil {
		return nil, gperr.Validation("finding port not configured")
	}
	result, err := s.findCaseResult(ctx, teamID, runID, in.CaseResultID)
	if err != nil {
		return nil, err
	}
	if result.FindingID != nil {
		return result, nil
	}
	if result.Status != domain.CaseFail && result.Status != domain.CaseBlocked {
		return nil, gperr.Conflict("only failed/blocked results can be triaged")
	}
	in = normalizeTriageInput(in)
	if err := validateTriageInput(in); err != nil {
		return nil, err
	}
	findingID, err := s.finding.CreateFinding(ctx, buildFindingDraft(in, result, runID))
	if err != nil {
		return nil, err
	}
	if err := s.markResultFinding(ctx, teamID, result, findingID); err != nil {
		return nil, err
	}
	result.FindingID = &findingID
	result.FailureClass = "product"
	return result, nil
}

func (s *RunService) findCaseResult(ctx context.Context, teamID, runID, caseResultID int64) (*domain.CaseResult, error) {
	results, err := s.repo.CaseResults(ctx, teamID, runID)
	if err != nil {
		return nil, err
	}
	for _, candidate := range results {
		if candidate.ID == caseResultID {
			return candidate, nil
		}
	}
	return nil, gperr.NotFound("case result not found")
}

// normalizeTriageInput applies the default triage taxonomy for omitted fields.
func normalizeTriageInput(in TriageInput) TriageInput {
	if strings.TrimSpace(in.Title) == "" {
		in.Title = "执行失败待归因"
	}
	if in.Severity == "" {
		in.Severity = "P2"
	}
	if in.Confidence == "" {
		in.Confidence = "medium"
	}
	return in
}

func validateTriageInput(in TriageInput) error {
	if strings.TrimSpace(in.RepoURL) == "" || strings.TrimSpace(in.CommitSHA) == "" {
		return gperr.Validation("repo_url and commit_sha required")
	}
	return nil
}

func buildFindingDraft(in TriageInput, result *domain.CaseResult, runID int64) FindingDraft {
	return FindingDraft{
		ProjectID: in.ProjectID, TargetID: in.TargetID, RepoName: in.RepoName, RepoURL: in.RepoURL,
		Branch: in.Branch, CommitSHA: in.CommitSHA, Module: in.Module, Endpoint: in.Endpoint,
		Title: in.Title, Severity: in.Severity, Category: "product", Expected: in.Expected,
		Actual: firstNonEmpty(in.Actual, result.ResultText), RootCause: in.RootCause, Impact: in.Impact,
		Confidence: in.Confidence, SourceFile: in.SourceFile, SourceLine: in.SourceLine,
		Request: requestFromResult(result), Response: result.ResultText, Log: logsFromResult(result),
		LinkedRunID: runID, LinkedCaseID: result.CaseID,
	}
}

func (s *RunService) markResultFinding(ctx context.Context, teamID int64, result *domain.CaseResult, findingID int64) error {
	if err := s.repo.MarkCaseResultFinding(ctx, teamID, result.ID, findingID, "product"); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gperr.NotFound("case result not found")
		}
		return err
	}
	return nil
}

func requestFromResult(result *domain.CaseResult) string {
	if result.Evidence == nil {
		return ""
	}
	return strings.Join(result.Evidence.Screenshots, "\n")
}

func logsFromResult(result *domain.CaseResult) string {
	if result.Evidence == nil {
		return ""
	}
	return strings.Join(result.Evidence.Logs, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
