// Package api 用例导入 HTTP 接入（DSH 生成 Job → cas_case）。
package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

// CaseImportServicePort is the use case consumed by the HTTP adapter; it exists
// so authorization tests do not need the full case/coverage stack.
type CaseImportServicePort interface {
	Import(ctx context.Context, jobID, targetID int64) ([]*domain.Case, error)
	JobProject(ctx context.Context, jobID int64) (int64, error)
}

type caseImportHandler struct {
	svc        CaseImportServicePort
	authorizer iam.AuthorizationPort
}

// RegisterCaseImport registers the explicit import endpoint without
// authorization for embedded/test compositions.
func RegisterCaseImport(mux *http.ServeMux, svc CaseImportServicePort) {
	registerCaseImport(mux, svc, nil)
}

// RegisterCaseImportAuthorized registers the import endpoint with project-level
// edit authorization. The job's own project is authoritative; a client-supplied
// target_id must match it.
func RegisterCaseImportAuthorized(mux *http.ServeMux, svc CaseImportServicePort, authorizer iam.AuthorizationPort) {
	registerCaseImport(mux, svc, authorizer)
}

func registerCaseImport(mux *http.ServeMux, svc CaseImportServicePort, authorizer iam.AuthorizationPort) {
	h := &caseImportHandler{svc: svc, authorizer: authorizer}
	mux.HandleFunc("POST /generation/jobs/{id}/cases:import", h.importCases)
}

type caseImportRequest struct {
	TargetID int64 `json:"target_id"`
}

// authorizeCaseImport applies the same edit/view policy as the rest of the
// generation surface, returning 401 for a missing principal and 403 for a
// denied grant.
func (h *caseImportHandler) authorizeCaseImport(w http.ResponseWriter, r *http.Request, projectID int64) bool {
	if h.authorizer == nil {
		return true
	}
	principal, ok := iam.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	if projectID <= 0 {
		httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return false
	}
	if err := h.authorizer.Authorize(r.Context(), principal, "edit", strconv.FormatInt(projectID, 10)); err != nil {
		if errors.Is(err, iam.ErrUnauthenticated) || errors.Is(err, iam.ErrInvalidPrincipal) {
			httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return false
		}
		httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return false
	}
	return true
}

func (h *caseImportHandler) importCases(w http.ResponseWriter, r *http.Request) {
	if _, ok := rls.TenantFrom(r.Context()); !ok {
		httpx.WriteErr(w, gperr.Unauthorized("tenant required"))
		return
	}
	jobID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || jobID <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid generation job id"))
		return
	}
	var req caseImportRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	projectID, err := h.svc.JobProject(r.Context(), jobID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.TargetID > 0 && req.TargetID != projectID {
		httpx.WriteErr(w, gperr.Validation("target_id does not match the generation job project"))
		return
	}
	if !h.authorizeCaseImport(w, r, projectID) {
		return
	}
	cases, err := h.svc.Import(r.Context(), jobID, projectID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toCaseResponses(cases))
}
