// Package api 治理域 HTTP 接入层：测试用例版本化路由（新增/更新/删除/回退/历史）。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/iam"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type caseHandler struct {
	svc        *application.CaseService
	authorizer iam.AuthorizationPort
}

// RegisterCases 注册用例版本化路由。
func RegisterCases(mux *http.ServeMux, svc *application.CaseService) {
	registerCases(mux, &caseHandler{svc: svc})
}

// RegisterCasesAuthorized enables target-level RBAC for case management.
// Case permissions are evaluated against the target owning the case, rather
// than against a case ID, because IAM grants are target-scoped.
func RegisterCasesAuthorized(mux *http.ServeMux, svc *application.CaseService, authorizer iam.AuthorizationPort) {
	registerCases(mux, &caseHandler{svc: svc, authorizer: authorizer})
}

func registerCases(mux *http.ServeMux, h *caseHandler) {
	mux.HandleFunc("POST /cases", h.createCase)
	mux.HandleFunc("POST /cases/{id}/versions", h.createVersion)
	mux.HandleFunc("POST /cases/{id}/rollback", h.rollback)
	mux.HandleFunc("DELETE /cases/{id}", h.deleteCase)
	mux.HandleFunc("GET /cases", h.listCases)
	mux.HandleFunc("GET /cases/{id}/history", h.history)
}

func (h *caseHandler) authorizeTarget(w http.ResponseWriter, r *http.Request, action string, targetID int64) bool {
	if h.authorizer == nil {
		return true
	}
	principal, ok := iam.PrincipalFromContext(r.Context())
	if !ok || h.authorizer.Authorize(r.Context(), principal, action, strconv.FormatInt(targetID, 10)) != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func (h *caseHandler) authorizeCase(w http.ResponseWriter, r *http.Request, action string, caseID int64) bool {
	if h.authorizer == nil {
		return true
	}
	// Reject unauthenticated requests before loading the case. Besides failing
	// closed, this avoids exposing case existence through a 404 side channel.
	if _, ok := iam.PrincipalFromContext(r.Context()); !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	c, err := h.svc.FindCase(r.Context(), caseID)
	if err != nil {
		httpx.WriteErr(w, err)
		return false
	}
	return h.authorizeTarget(w, r, action, c.TargetID)
}

// createCaseRequest 新建用例请求。
type createCaseRequest struct {
	TargetID int64           `json:"target_id"`
	Code     string          `json:"code"`
	Title    string          `json:"title"`
	Kind     string          `json:"kind,omitempty"`
	Script   json.RawMessage `json:"script,omitempty"`
}

func (h *caseHandler) createCase(w http.ResponseWriter, r *http.Request) {
	var req createCaseRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.TargetID == 0 || req.Code == "" || req.Title == "" {
		httpx.WriteErr(w, gperr.Validation("target_id, code, title required"))
		return
	}
	if !h.authorizeTarget(w, r, "edit", req.TargetID) {
		return
	}
	cc := domain.Case{
		TargetID: req.TargetID, Code: req.Code, Title: req.Title, Kind: req.Kind,
	}
	created, err := h.svc.CreateCase(r.Context(), cc, req.Script)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toCaseResponse(created))
}

// updateVersionRequest 新增用例版本请求。
type updateVersionRequest struct {
	Script       json.RawMessage `json:"script,omitempty"`
	SourceRepoID *int64          `json:"source_repo_id,omitempty"`
	SourceBranch *string         `json:"source_branch,omitempty"`
}

func (h *caseHandler) createVersion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid case id"))
		return
	}
	if !h.authorizeCase(w, r, "edit", id) {
		return
	}
	var req updateVersionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	c, err := h.svc.CreateVersion(r.Context(), id, req.Script, req.SourceRepoID, req.SourceBranch)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCaseResponse(c))
}

// rollbackRequest 回退到指定版本请求。
type rollbackRequest struct {
	Version int `json:"version"`
}

func (h *caseHandler) rollback(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid case id"))
		return
	}
	if !h.authorizeCase(w, r, "edit", id) {
		return
	}
	var req rollbackRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.Version <= 0 {
		httpx.WriteErr(w, gperr.Validation("version required"))
		return
	}
	c, err := h.svc.RollbackTo(r.Context(), id, req.Version)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCaseResponse(c))
}

func (h *caseHandler) deleteCase(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid case id"))
		return
	}
	if !h.authorizeCase(w, r, "edit", id) {
		return
	}
	if err := h.svc.MarkDeleted(r.Context(), id); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *caseHandler) listCases(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := domain.CaseFilter{Kind: q.Get("kind"), Code: q.Get("code"), Status: q.Get("status")}
	if tid := q.Get("target_id"); tid != "" {
		if n, err := strconv.ParseInt(tid, 10, 64); err == nil {
			f.TargetID = &n
		}
	}
	if h.authorizer != nil {
		if f.TargetID == nil || *f.TargetID <= 0 {
			httpx.WriteErr(w, gperr.Validation("target_id required when authorization is enabled"))
			return
		}
		if !h.authorizeTarget(w, r, "view", *f.TargetID) {
			return
		}
	}
	cases, err := h.svc.ListCases(r.Context(), f)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCaseResponses(cases))
}

func (h *caseHandler) history(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid case id"))
		return
	}
	if !h.authorizeCase(w, r, "view", id) {
		return
	}
	hist, err := h.svc.History(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCaseVersionResponses(hist))
}
