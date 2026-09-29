// 执行域运行编排 HTTP 接入层：创建运行 / 版本校验前置 / 执行 / 暂停恢复 / 明细查询。
package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/execution/application"
	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type runHandler struct{ svc *application.RunService }

func (h *runHandler) createRun(w http.ResponseWriter, r *http.Request) {
	var req application.CreateRunRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.ScenarioID == 0 || req.TargetID == 0 || req.TargetVersion == "" {
		httpx.WriteErr(w, gperr.Validation("scenario_id, target_id, target_version required"))
		return
	}
	run, err := h.svc.CreateRun(r.Context(), req)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, run)
}

func (h *runHandler) getRun(w http.ResponseWriter, r *http.Request) {
	id, ok := runID(w, r)
	if !ok {
		return
	}
	run, err := h.svc.GetRun(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrRunNotFound) {
			httpx.WriteErr(w, gperr.NotFound("run not found"))
			return
		}
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, run)
}

func (h *runHandler) caseResults(w http.ResponseWriter, r *http.Request) {
	id, ok := runID(w, r)
	if !ok {
		return
	}
	results, err := h.svc.CaseResults(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, results)
}

func (h *runHandler) startVersionCheck(w http.ResponseWriter, r *http.Request) {
	id, ok := runID(w, r)
	if !ok {
		return
	}
	run, err := h.svc.StartVersionCheck(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, run)
}

func (h *runHandler) executeRun(w http.ResponseWriter, r *http.Request) {
	id, ok := runID(w, r)
	if !ok {
		return
	}
	run, results, err := h.svc.ExecuteRun(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		Run     any `json:"run"`
		Results any `json:"results"`
	}{run, results})
}

func (h *runHandler) pauseRun(w http.ResponseWriter, r *http.Request) {
	id, ok := runID(w, r)
	if !ok {
		return
	}
	run, err := h.svc.PauseRun(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, run)
}

func (h *runHandler) resumeRun(w http.ResponseWriter, r *http.Request) {
	id, ok := runID(w, r)
	if !ok {
		return
	}
	run, err := h.svc.ResumeRun(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, run)
}

func runID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid run id"))
		return 0, false
	}
	return id, true
}
