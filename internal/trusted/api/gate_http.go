// Package api 可信域 HTTP 接入层：门禁策略装载 / 门禁判定 / 判定与审计查询。
package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/trusted/application"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type gateHandler struct {
	gate   *application.GateService
	audit  *application.AuditService
	cost   *application.CostService
	report *application.ReportService
}

// Register 注册可信域路由（门禁 + 审计 + 成本 + 报告）。
func Register(mux *http.ServeMux, gate *application.GateService, audit *application.AuditService, cost *application.CostService, report *application.ReportService) {
	h := &gateHandler{gate: gate, audit: audit, cost: cost, report: report}
	mux.HandleFunc("PUT /gates/rules", h.upsertRule)
	mux.HandleFunc("POST /gates/evaluate", h.evaluate)
	mux.HandleFunc("GET /gates/results", h.resultsByRun)
	// 成本
	mux.HandleFunc("POST /cost/record", h.recordCost)
	mux.HandleFunc("GET /cost/overview", h.costOverview)
	mux.HandleFunc("GET /cost/items", h.costItems)
	mux.HandleFunc("GET /cost/compare/{caseId}", h.costCompare)
	// 报告
	mux.HandleFunc("POST /reports/generate", h.generateReport)
	mux.HandleFunc("GET /reports/{id}/export", h.exportReport)
}

// upsertRuleRequest 装载门禁规则请求。
type upsertRuleRequest struct {
	TargetID   int64  `json:"target_id"`
	ScenarioID int64  `json:"scenario_id"`
	Rego       string `json:"rego"`
	Enabled    *bool  `json:"enabled"`
}

func (h *gateHandler) upsertRule(w http.ResponseWriter, r *http.Request) {
	var req upsertRuleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.TargetID == 0 || req.ScenarioID == 0 || req.Rego == "" {
		httpx.WriteErr(w, gperr.Validation("target_id, scenario_id, rego required"))
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule, err := h.gate.UpsertRule(r.Context(), req.TargetID, req.ScenarioID, req.Rego, enabled)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rule)
}

// evaluateRequest 门禁判定请求。
type evaluateRequest struct {
	RunID int64 `json:"run_id"`
}

func (h *gateHandler) evaluate(w http.ResponseWriter, r *http.Request) {
	var req evaluateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.RunID == 0 {
		httpx.WriteErr(w, gperr.Validation("run_id required"))
		return
	}
	res, err := h.gate.Evaluate(r.Context(), req.RunID)
	if err != nil {
		if errors.Is(err, domain.ErrRunNotFound) {
			httpx.WriteErr(w, gperr.NotFound("run not found"))
			return
		}
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *gateHandler) resultsByRun(w http.ResponseWriter, r *http.Request) {
	runID, err := strconv.ParseInt(r.URL.Query().Get("run_id"), 10, 64)
	if err != nil || runID == 0 {
		httpx.WriteErr(w, gperr.Validation("run_id required"))
		return
	}
	results, err := h.gate.ResultsByRun(r.Context(), runID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, results)
}
