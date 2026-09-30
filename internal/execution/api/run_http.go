// 执行域运行编排 HTTP 接入层：创建运行 / 版本校验前置 / 执行 / 暂停恢复 / 明细查询。
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

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
	httpx.WriteJSON(w, http.StatusCreated, toRunResponse(run))
}

func (h *runHandler) listRuns(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var targetID *int64
	if raw := query.Get("target_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			httpx.WriteErr(w, gperr.Validation("invalid target_id"))
			return
		}
		targetID = &id
	}
	limit, _ := strconv.Atoi(query.Get("limit"))
	runs, err := h.svc.ListRuns(r.Context(), targetID, query.Get("state"), limit)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	responses := make([]runResponse, 0, len(runs))
	for _, run := range runs {
		responses = append(responses, toRunResponse(run))
	}
	httpx.WriteJSON(w, http.StatusOK, responses)
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
	httpx.WriteJSON(w, http.StatusOK, toRunResponse(run))
}

// events emits the current run snapshot immediately, then emits changed snapshots.
// It polls the authoritative repository so the endpoint remains correct when the
// workflow worker and HTTP server are separate processes. Shared pub/sub can be
// added later to reduce polling for multi-replica deployments.
func (h *runHandler) events(w http.ResponseWriter, r *http.Request) {
	id, ok := runID(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming responses are not supported", http.StatusInternalServerError)
		return
	}
	initial, err := h.svc.GetRun(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrRunNotFound) {
			httpx.WriteErr(w, gperr.NotFound("run not found"))
			return
		}
		httpx.WriteErr(w, err)
		return
	}
	initialPayload, err := json.Marshal(toRunResponse(initial))
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	last := string(initialPayload)
	sequence := 1
	if _, err := fmt.Fprintf(w, "id: %d\nevent: run\ndata: %s\n\n", sequence, initialPayload); err != nil {
		return
	}
	flusher.Flush()
	send := func() error {
		run, err := h.svc.GetRun(r.Context(), id)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(toRunResponse(run))
		if err != nil {
			return err
		}
		current := string(payload)
		if current == last {
			return nil
		}
		last = current
		sequence++
		if _, err := fmt.Fprintf(w, "id: %d\nevent: run\ndata: %s\n\n", sequence, payload); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if err := send(); err != nil {
				return
			}
		}
	}
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
	httpx.WriteJSON(w, http.StatusOK, toCaseResultResponses(results))
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
	httpx.WriteJSON(w, http.StatusOK, toRunResponse(run))
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
		Run     runResponse          `json:"run"`
		Results []caseResultResponse `json:"results"`
	}{toRunResponse(run), toCaseResultResponses(results)})
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
	httpx.WriteJSON(w, http.StatusOK, toRunResponse(run))
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
	httpx.WriteJSON(w, http.StatusOK, toRunResponse(run))
}

func runID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid run id"))
		return 0, false
	}
	return id, true
}
