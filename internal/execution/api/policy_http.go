// 执行域截图开关策略 HTTP 接入层（PRD R-TEST-13）。
package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/execution/application"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type policyHandler struct{ svc *application.PolicyService }

// setScreenshotPolicy 下发服务级截图开关策略。
func (h *policyHandler) setScreenshotPolicy(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid target id"))
		return
	}
	var req application.SetPolicyRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	req.TargetID = id
	p, err := h.svc.Set(r.Context(), req)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

// getScreenshotPolicy 读取截图开关策略。
func (h *policyHandler) getScreenshotPolicy(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid target id"))
		return
	}
	scenario, _ := strconv.ParseInt(r.URL.Query().Get("scenario_id"), 10, 64)
	teamID, ok := rls.TenantFrom(r.Context())
	if !ok {
		httpx.WriteErr(w, gperr.Validation("tenant required"))
		return
	}
	p, err := h.svc.Get(r.Context(), teamID, id, scenario)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}
