// Package api 治理域缺陷中心 HTTP 接入：列表/详情/创建/状态流转。
package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/domain"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type findingHandler struct {
	svc *application.FindingService
}

// RegisterFindings 注册缺陷中心路由。
func RegisterFindings(mux *http.ServeMux, svc *application.FindingService) {
	h := &findingHandler{svc: svc}
	mux.HandleFunc("GET /findings", h.list)
	mux.HandleFunc("POST /findings", h.create)
	mux.HandleFunc("GET /findings/{id}", h.get)
	mux.HandleFunc("POST /findings/{id}/transition", h.transition)
}

type findingEvidenceResponse struct {
	Request  string `json:"request,omitempty"`
	Response string `json:"response,omitempty"`
	Log      string `json:"log,omitempty"`
	Trace    string `json:"trace,omitempty"`
}

type findingRepositoryResponse struct {
	ID        *int64 `json:"id,omitempty"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Branch    string `json:"branch"`
	CommitSHA string `json:"commit_sha"`
}

type findingResponse struct {
	ID            int64                     `json:"id"`
	TeamID        int64                     `json:"team_id"`
	ProjectID     int64                     `json:"project_id"`
	ServiceID     *int64                    `json:"service_id,omitempty"`
	EndpointID    *int64                    `json:"endpoint_id,omitempty"`
	TargetID      *int64                    `json:"target_id,omitempty"`
	Repository    findingRepositoryResponse `json:"repository"`
	Module        string                    `json:"module"`
	Endpoint      string                    `json:"endpoint"`
	Title         string                    `json:"title"`
	Description   string                    `json:"description"`
	Severity      string                    `json:"severity"`
	Category      string                    `json:"category"`
	Status        string                    `json:"status"`
	Expected      string                    `json:"expected"`
	Actual        string                    `json:"actual"`
	RootCause     string                    `json:"root_cause"`
	Impact        string                    `json:"impact"`
	Confidence    string                    `json:"confidence"`
	SourceFile    string                    `json:"source_file,omitempty"`
	SourceLine    int                       `json:"source_line,omitempty"`
	Evidence      findingEvidenceResponse   `json:"evidence"`
	CoverageCells []string                  `json:"coverage_cells"`
	OwnerID       *int64                    `json:"owner_id,omitempty"`
	Owner         string                    `json:"owner"`
	LinkedRunID   *int64                    `json:"linked_run_id,omitempty"`
	LinkedCaseID  *int64                    `json:"linked_case_id,omitempty"`
	DedupeHash    string                    `json:"dedupe_hash"`
	CreatedAt     string                    `json:"created_at"`
	UpdatedAt     string                    `json:"updated_at"`
	Timeline      []findingEventResponse    `json:"timeline,omitempty"`
}

type findingEventResponse struct {
	Time   string `json:"time"`
	Text   string `json:"text"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
}

type createFindingRequest struct {
	ProjectID     int64                   `json:"project_id"`
	ServiceID     *int64                  `json:"service_id,omitempty"`
	EndpointID    *int64                  `json:"endpoint_id,omitempty"`
	TargetID      *int64                  `json:"target_id,omitempty"`
	RepositoryID  *int64                  `json:"repository_id,omitempty"`
	RepoName      string                  `json:"repo_name"`
	RepoURL       string                  `json:"repo_url"`
	Branch        string                  `json:"branch"`
	CommitSHA     string                  `json:"commit_sha"`
	Module        string                  `json:"module"`
	Endpoint      string                  `json:"endpoint"`
	Title         string                  `json:"title"`
	Description   string                  `json:"description"`
	Severity      string                  `json:"severity"`
	Category      string                  `json:"category"`
	Expected      string                  `json:"expected"`
	Actual        string                  `json:"actual"`
	RootCause     string                  `json:"root_cause"`
	Impact        string                  `json:"impact"`
	Confidence    string                  `json:"confidence"`
	SourceFile    string                  `json:"source_file"`
	SourceLine    int                     `json:"source_line"`
	Evidence      findingEvidenceResponse `json:"evidence"`
	CoverageCells []string                `json:"coverage_cells"`
	OwnerID       *int64                  `json:"owner_id,omitempty"`
	LinkedRunID   *int64                  `json:"linked_run_id,omitempty"`
	LinkedCaseID  *int64                  `json:"linked_case_id,omitempty"`
}

type transitionRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

func (h *findingHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := domain.FindingFilter{
		Severity: q.Get("severity"), Status: q.Get("status"), Query: strings.TrimSpace(q.Get("q")),
		Module: q.Get("module"),
	}
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			httpx.WriteErr(w, gperr.Validation("limit must be between 1 and 100"))
			return
		}
		filter.Limit = n
	}
	if raw := q.Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			httpx.WriteErr(w, gperr.Validation("offset must be >= 0"))
			return
		}
		filter.Offset = n
	}
	items, err := h.svc.List(r.Context(), filter)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	out := make([]findingResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toFindingResponse(item, false))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *findingHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseFindingID(r)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	f, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toFindingResponse(f, true))
}

func (h *findingHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createFindingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	in := application.CreateFindingInput{
		ProjectID: req.ProjectID, ServiceID: req.ServiceID, EndpointID: req.EndpointID,
		TargetID: req.TargetID, RepositoryID: req.RepositoryID, RepoName: req.RepoName,
		RepoURL: req.RepoURL, Branch: req.Branch, CommitSHA: req.CommitSHA, Module: req.Module,
		Endpoint: req.Endpoint, Title: req.Title, Description: req.Description,
		Severity: domain.Severity(req.Severity), Category: req.Category, Expected: req.Expected,
		Actual: req.Actual, RootCause: req.RootCause, Impact: req.Impact,
		Confidence: domain.Confidence(req.Confidence),
		SourceRef:  map[string]any{"file": req.SourceFile, "line": req.SourceLine},
		Evidence:   findingEvidenceDomain(req.Evidence), CoverageCells: req.CoverageCells,
		OwnerID: req.OwnerID, LinkedRunID: req.LinkedRunID, LinkedCaseID: req.LinkedCaseID,
	}
	if in.RepoName == "" {
		in.RepoName = repoNameFromURL(in.RepoURL)
	}
	f, created, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, toFindingResponse(f, true))
}

func (h *findingHandler) transition(w http.ResponseWriter, r *http.Request) {
	id, err := parseFindingID(r)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	var req transitionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if strings.TrimSpace(req.Action) == "" {
		httpx.WriteErr(w, gperr.Validation("action required"))
		return
	}
	f, err := h.svc.Transition(r.Context(), id, domain.FindingAction(req.Action), strings.TrimSpace(req.Reason))
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toFindingResponse(f, true))
}

func parseFindingID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, gperr.Validation("invalid finding id")
	}
	return id, nil
}

func toFindingResponse(f *domain.Finding, withTimeline bool) findingResponse {
	if f == nil {
		return findingResponse{}
	}
	owner := "未分配"
	if f.OwnerID != nil {
		owner = "成员 #" + strconv.FormatInt(*f.OwnerID, 10)
	}
	sourceFile, _ := f.SourceRef["file"].(string)
	sourceLine := 0
	switch v := f.SourceRef["line"].(type) {
	case int:
		sourceLine = v
	case int64:
		sourceLine = int(v)
	case float64:
		sourceLine = int(v)
	}
	out := findingResponse{
		ID: f.ID, TeamID: f.TeamID, ProjectID: f.ProjectID, ServiceID: f.ServiceID,
		EndpointID: f.EndpointID, TargetID: f.TargetID,
		Repository: findingRepositoryResponse{
			ID: f.Repository.ID, Name: f.Repository.Name, URL: f.Repository.URL,
			Branch: f.Repository.Branch, CommitSHA: f.Repository.CommitSHA,
		},
		Module: f.Module, Endpoint: f.Endpoint, Title: f.Title, Description: f.Description,
		Severity: string(f.Severity), Category: f.Category, Status: string(f.Status),
		Expected: f.Expected, Actual: f.Actual, RootCause: f.RootCause, Impact: f.Impact,
		Confidence: string(f.Confidence), SourceFile: sourceFile, SourceLine: sourceLine,
		Evidence: findingEvidenceResponse{
			Request: f.Evidence.Request, Response: f.Evidence.Response,
			Log: f.Evidence.Log, Trace: f.Evidence.Trace,
		},
		CoverageCells: f.CoverageCells, OwnerID: f.OwnerID, Owner: owner,
		LinkedRunID: f.LinkedRunID, LinkedCaseID: f.LinkedCaseID, DedupeHash: f.DedupeHash,
		CreatedAt: f.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: f.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if withTimeline {
		for _, e := range f.Events {
			out.Timeline = append(out.Timeline, findingEventResponse{
				Time: e.CreatedAt.UTC().Format("15:04"), Text: findingEventText(e),
				Actor: actorLabel(e.ActorID), Action: e.Action, From: e.FromStatus, To: e.ToStatus,
			})
		}
	}
	return out
}

func findingEventText(e domain.FindingEvent) string {
	switch e.Action {
	case "created":
		return "执行失败，创建缺陷候选"
	case "triage":
		return "已归因，进入待确认"
	case "confirm":
		return "确认产品缺陷"
	case "false_positive":
		return "标记为误报"
	case "pending_regression":
		return "标记已修复，待回归"
	case "close":
		return "回归通过，关闭缺陷"
	case "reopen":
		return "缺陷重开"
	default:
		return e.Action
	}
}

func actorLabel(actor int64) string {
	if actor <= 0 {
		return "系统"
	}
	return "成员 #" + strconv.FormatInt(actor, 10)
}

func repoNameFromURL(raw string) string {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), "/")
	raw = strings.TrimSuffix(raw, ".git")
	if i := strings.LastIndex(raw, "/"); i >= 0 {
		return raw[i+1:]
	}
	return raw
}

func findingEvidenceDomain(e findingEvidenceResponse) domain.FindingEvidence {
	return domain.FindingEvidence{Request: e.Request, Response: e.Response, Log: e.Log, Trace: e.Trace}
}
