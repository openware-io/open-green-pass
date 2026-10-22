// Package api 覆盖/缺陷门禁 HTTP 接入。
package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type coverageGateHandler struct {
	svc *application.CoverageGateService
}

// RegisterCoverageGate registers the coverage/defect gate.
func RegisterCoverageGate(mux *http.ServeMux, svc *application.CoverageGateService) {
	h := &coverageGateHandler{svc: svc}
	mux.HandleFunc("GET /gates/coverage", h.evaluate)
}

func (h *coverageGateHandler) evaluate(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	projectID, err := strconv.ParseInt(q.Get("project_id"), 10, 64)
	if err != nil || projectID <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid project_id"))
		return
	}
	result, err := h.svc.Evaluate(r.Context(), projectID, q.Get("module"))
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}
