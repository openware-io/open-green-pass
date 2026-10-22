// Package api 用例导入 HTTP 接入（DSH 生成 Job → cas_case）。
package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type caseImportHandler struct {
	svc *application.CaseImportService
}

// RegisterCaseImport registers the explicit import endpoint.
func RegisterCaseImport(mux *http.ServeMux, svc *application.CaseImportService) {
	h := &caseImportHandler{svc: svc}
	mux.HandleFunc("POST /generation/jobs/{id}/cases:import", h.importCases)
}

type caseImportRequest struct {
	TargetID int64 `json:"target_id"`
}

func (h *caseImportHandler) importCases(w http.ResponseWriter, r *http.Request) {
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
	cases, err := h.svc.Import(r.Context(), jobID, req.TargetID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toCaseResponses(cases))
}
