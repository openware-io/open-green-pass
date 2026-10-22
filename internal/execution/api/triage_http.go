// Package api 执行失败归因：run_case_result → governance finding。
package api

import (
	"net/http"

	"github.com/openware-io/open-green-pass/internal/execution/application"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type triageRequest struct {
	CaseResultID int64  `json:"case_result_id"`
	ProjectID    int64  `json:"project_id"`
	TargetID     int64  `json:"target_id"`
	RepoName     string `json:"repo_name"`
	RepoURL      string `json:"repo_url"`
	Branch       string `json:"branch"`
	CommitSHA    string `json:"commit_sha"`
	Module       string `json:"module"`
	Endpoint     string `json:"endpoint"`
	Title        string `json:"title"`
	Severity     string `json:"severity"`
	RootCause    string `json:"root_cause"`
	Impact       string `json:"impact"`
	Confidence   string `json:"confidence"`
	SourceFile   string `json:"source_file"`
	SourceLine   int    `json:"source_line"`
	Expected     string `json:"expected"`
	Actual       string `json:"actual"`
}

func (h *runHandler) triage(w http.ResponseWriter, r *http.Request) {
	id, ok := runID(w, r)
	if !ok {
		return
	}
	if _, ok := h.findAndAuthorize(w, r, id, "exec"); !ok {
		return
	}
	var req triageRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.CaseResultID <= 0 {
		httpx.WriteErr(w, gperr.Validation("case_result_id required"))
		return
	}
	result, err := h.svc.Triage(r.Context(), id, application.TriageInput{
		CaseResultID: req.CaseResultID, ProjectID: req.ProjectID, TargetID: req.TargetID,
		RepoName: req.RepoName, RepoURL: req.RepoURL, Branch: req.Branch, CommitSHA: req.CommitSHA,
		Module: req.Module, Endpoint: req.Endpoint, Title: req.Title, Severity: req.Severity,
		RootCause: req.RootCause, Impact: req.Impact, Confidence: req.Confidence,
		SourceFile: req.SourceFile, SourceLine: req.SourceLine, Expected: req.Expected, Actual: req.Actual,
	})
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toCaseResultResponse(result))
}
