// Package api 执行域 HTTP 接入层：测试环境版本校验路由（登记运行版本/校验/历史）。
package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/execution/application"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type envHandler struct{ svc *application.EnvService }

// Register 注册执行域路由（版本校验 + 运行编排）。
func Register(mux *http.ServeMux, envSvc *application.EnvService, runSvc *application.RunService, policySvc *application.PolicyService) {
	h := &envHandler{svc: envSvc}
	rh := &runHandler{svc: runSvc}
	ph := &policyHandler{svc: policySvc}
	mux.HandleFunc("POST /targets/{id}/env/runtime", h.registerRuntime)
	mux.HandleFunc("POST /targets/{id}/version-check", h.checkVersion)
	mux.HandleFunc("GET /targets/{id}/version-checks", h.recentChecks)
	mux.HandleFunc("PUT /targets/{id}/screenshot-policy", ph.setScreenshotPolicy)
	mux.HandleFunc("GET /targets/{id}/screenshot-policy", ph.getScreenshotPolicy)
	mux.HandleFunc("POST /runs", rh.createRun)
	mux.HandleFunc("GET /runs", rh.listRuns)
	mux.HandleFunc("GET /runs/{id}", rh.getRun)
	mux.HandleFunc("GET /runs/{id}/events", rh.events)
	mux.HandleFunc("GET /runs/{id}/case-results", rh.caseResults)
	mux.HandleFunc("POST /runs/{id}/version-check", rh.startVersionCheck)
	mux.HandleFunc("POST /runs/{id}/execute", rh.executeRun)
	mux.HandleFunc("POST /runs/{id}/pause", rh.pauseRun)
	mux.HandleFunc("POST /runs/{id}/resume", rh.resumeRun)
}

// registerRuntimeRequest 登记环境运行版本请求。
type registerRuntimeRequest struct {
	Env     string `json:"env"`
	Version string `json:"version"`
}

func (h *envHandler) registerRuntime(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid target id"))
		return
	}
	var req registerRuntimeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.Env == "" || req.Version == "" {
		httpx.WriteErr(w, gperr.Validation("env, version required"))
		return
	}
	rt, err := h.svc.RegisterRuntime(r.Context(), id, req.Env, req.Version)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toEnvRuntimeResponse(rt))
}

// checkVersionRequest 版本校验请求。
type checkVersionRequest struct {
	Env           string `json:"env"`
	TargetVersion string `json:"target_version"`
}

func (h *envHandler) checkVersion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid target id"))
		return
	}
	var req checkVersionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.Env == "" || req.TargetVersion == "" {
		httpx.WriteErr(w, gperr.Validation("env, target_version required"))
		return
	}
	c, err := h.svc.CheckVersion(r.Context(), id, req.Env, req.TargetVersion)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toEnvCheckResponse(c))
}

func (h *envHandler) recentChecks(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid target id"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	checks, err := h.svc.RecentChecks(r.Context(), id, limit)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toEnvCheckResponses(checks))
}
