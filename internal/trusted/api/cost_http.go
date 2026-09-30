// 可信域成本 HTTP 接入层：计量写入（mock meter）/ 总览 / 明细 / 历史对比。
package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/trusted/application"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

func (h *gateHandler) recordCost(w http.ResponseWriter, r *http.Request) {
	var req application.RecordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.RequestID == "" || req.BizPoint == "" {
		httpx.WriteErr(w, gperr.Validation("request_id, biz_point required"))
		return
	}
	inserted, err := h.cost.Record(r.Context(), req)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"inserted": inserted})
}

func (h *gateHandler) costOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := h.cost.Overview(r.Context())
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCostOverviewResponse(ov))
}

func (h *gateHandler) costItems(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	point := r.URL.Query().Get("point")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.cost.Items(r.Context(), category, point, limit)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCostLineResponses(items))
}

func (h *gateHandler) costCompare(w http.ResponseWriter, r *http.Request) {
	caseID, err := strconv.ParseInt(r.PathValue("caseId"), 10, 64)
	if err != nil || caseID == 0 {
		httpx.WriteErr(w, gperr.Validation("invalid case id"))
		return
	}
	items, err := h.cost.CompareHistory(r.Context(), caseID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCostLineResponses(items))
}
