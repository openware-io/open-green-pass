// 可信域报告 HTTP 接入层：生成报告 / 导出 HTML。
package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/trusted/application"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

func (h *gateHandler) generateReport(w http.ResponseWriter, r *http.Request) {
	var req application.GenerateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.RunID == 0 {
		httpx.WriteErr(w, gperr.Validation("run_id required"))
		return
	}
	rep, err := h.report.Generate(r.Context(), req)
	if err != nil {
		if err == domain.ErrRunNotFound {
			httpx.WriteErr(w, gperr.NotFound("run not found"))
			return
		}
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rep)
}

func (h *gateHandler) exportReport(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		httpx.WriteErr(w, gperr.Validation("invalid report id"))
		return
	}
	rep, err := h.report.Export(r.Context(), id)
	if err != nil {
		if err == domain.ErrReportNotFound {
			httpx.WriteErr(w, gperr.NotFound("report not found"))
			return
		}
		httpx.WriteErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="greenpass-report-`+strconv.FormatInt(id, 10)+`.html"`)
	_, _ = w.Write([]byte(rep.HTML))
}
