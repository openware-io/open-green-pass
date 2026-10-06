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
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type runHandler struct {
	svc        *application.RunService
	authorizer iam.AuthorizationPort
}

// authorizeTarget checks the target-level permission of a run request. For
// run-id routes the run is first read through the tenant-scoped service, so a
// caller cannot turn a guessed run ID into authorization against another team.
func (h *runHandler) authorizeTarget(w http.ResponseWriter, r *http.Request, action string, targetID int64) bool {
	if h.authorizer == nil {
		return true
	}
	principal, ok := iam.PrincipalFromContext(r.Context())
	if !ok || h.authorizer.Authorize(r.Context(), principal, action, strconv.FormatInt(targetID, 10)) != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func (h *runHandler) findAndAuthorize(w http.ResponseWriter, r *http.Request, id int64, action string) (*domain.Run, bool) {
	run, err := h.svc.GetRun(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrRunNotFound) {
			httpx.WriteErr(w, gperr.NotFound("run not found"))
		} else {
			httpx.WriteErr(w, err)
		}
		return nil, false
	}
	if !h.authorizeTarget(w, r, action, run.TargetID) {
		return nil, false
	}
	return run, true
}

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
	if !h.authorizeTarget(w, r, "exec", req.TargetID) {
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
		if !h.authorizeTarget(w, r, "view", id) {
			return
		}
	}
	// Listing every run has no single target against which to make an explicit
	// asset decision. Require callers to scope the request by target whenever
	// authorization is enabled, rather than accidentally exposing all history.
	if h.authorizer != nil && targetID == nil {
		http.Error(w, "target_id required", http.StatusForbidden)
		return
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
	run, ok := h.findAndAuthorize(w, r, id, "view")
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toRunResponse(run))
}

// events emits authoritative snapshots. Redis notifications wake connections
// across replicas; periodic reads remain as a correctness fallback.
func (h *runHandler) events(w http.ResponseWriter, r *http.Request) {
	id, ok := runID(w, r)
	if !ok {
		return
	}
	if _, ok := h.findAndAuthorize(w, r, id, "view"); !ok {
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
	teamID, _ := rls.TenantFrom(r.Context())
	subscription, subscribeErr := h.svc.SubscribeRunEvents(r.Context(), teamID, id)
	var notifications <-chan struct{}
	if subscribeErr == nil && subscription != nil {
		defer subscription.Close()
		notifications = subscription.Events()
	}
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
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case _, open := <-notifications:
			if !open {
				notifications = nil
				continue
			}
			if err := send(); err != nil {
				return
			}
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
	if _, ok := h.findAndAuthorize(w, r, id, "view"); !ok {
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
	if _, ok := h.findAndAuthorize(w, r, id, "exec"); !ok {
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
	if _, ok := h.findAndAuthorize(w, r, id, "exec"); !ok {
		return
	}
	run, results, err := h.svc.ExecuteRun(r.Context(), id)
	if err != nil {
		if errors.Is(err, application.ErrResourceConflict) {
			httpx.WriteErr(w, gperr.Conflict("run conflicts with an active exclusive resource"))
			return
		}
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
	if _, ok := h.findAndAuthorize(w, r, id, "exec"); !ok {
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
	if _, ok := h.findAndAuthorize(w, r, id, "exec"); !ok {
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
