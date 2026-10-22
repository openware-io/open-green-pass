// Package api 执行 fixtures / 环境租约 HTTP 接入。
package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/application"
	execdomain "github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type fixtureHandler struct {
	svc        *application.FixtureService
	runs       *application.RunService
	authorizer iam.AuthorizationPort
}

// RegisterFixtures registers fixture and environment lease routes.
func RegisterFixtures(mux *http.ServeMux, svc *application.FixtureService, runs *application.RunService, authorizer iam.AuthorizationPort) {
	h := &fixtureHandler{svc: svc, runs: runs, authorizer: authorizer}
	mux.HandleFunc("POST /runs/{id}/fixtures:prepare", h.prepare)
	mux.HandleFunc("GET /runs/{id}/fixtures", h.list)
	mux.HandleFunc("POST /runs/{id}/fixtures:cleanup", h.cleanup)
	mux.HandleFunc("POST /envs/{id}/lease", h.lease)
	mux.HandleFunc("DELETE /envs/{id}/lease", h.release)
}

type fixtureRequest struct {
	Namespace  string         `json:"namespace"`
	Manifest   map[string]any `json:"manifest"`
	TTLSeconds int64          `json:"ttl_seconds"`
	RunID      int64          `json:"run_id"`
	Owner      string         `json:"owner"`
}

type fixtureResponse struct {
	ID           int64          `json:"id"`
	TeamID       int64          `json:"team_id"`
	RunID        int64          `json:"run_id"`
	Namespace    string         `json:"namespace"`
	Manifest     map[string]any `json:"manifest"`
	State        string         `json:"state"`
	ExpiresAt    *time.Time     `json:"expires_at,omitempty"`
	CleanupState string         `json:"cleanup_state"`
}

type leaseResponse struct {
	ID         int64     `json:"id"`
	TeamID     int64     `json:"team_id"`
	EnvID      int64     `json:"env_id"`
	RunID      int64     `json:"run_id"`
	Owner      string    `json:"owner"`
	LeaseUntil time.Time `json:"lease_until"`
}

func (h *fixtureHandler) targetForRun(w http.ResponseWriter, r *http.Request, runID int64, action string) bool {
	run, err := h.runs.GetRun(r.Context(), runID)
	if err != nil {
		httpx.WriteErr(w, err)
		return false
	}
	if h.authorizer != nil {
		principal, ok := iam.PrincipalFromContext(r.Context())
		if !ok || h.authorizer.Authorize(r.Context(), principal, action, strconv.FormatInt(run.TargetID, 10)) != nil {
			httpx.WriteErr(w, gperr.Forbidden("forbidden"))
			return false
		}
	}
	return true
}

func (h *fixtureHandler) prepare(w http.ResponseWriter, r *http.Request) {
	runID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || runID <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid run id"))
		return
	}
	if !h.targetForRun(w, r, runID, "exec") {
		return
	}
	var req fixtureRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	f, err := h.svc.Prepare(r.Context(), runID, req.Namespace, req.Manifest, time.Duration(req.TTLSeconds)*time.Second)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toFixtureResponse(f))
}

func (h *fixtureHandler) list(w http.ResponseWriter, r *http.Request) {
	runID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || runID <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid run id"))
		return
	}
	if !h.targetForRun(w, r, runID, "view") {
		return
	}
	items, err := h.svc.List(r.Context(), runID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	out := make([]fixtureResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toFixtureResponse(item))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *fixtureHandler) cleanup(w http.ResponseWriter, r *http.Request) {
	runID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || runID <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid run id"))
		return
	}
	if !h.targetForRun(w, r, runID, "exec") {
		return
	}
	var req fixtureRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if err := h.svc.Cleanup(r.Context(), runID, req.Namespace); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "cleaned"})
}

func (h *fixtureHandler) lease(w http.ResponseWriter, r *http.Request) {
	envID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || envID <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid env id"))
		return
	}
	var req fixtureRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.RunID <= 0 {
		httpx.WriteErr(w, gperr.Validation("run_id required"))
		return
	}
	if !h.targetForRun(w, r, req.RunID, "exec") {
		return
	}
	lease, err := h.svc.Lease(r.Context(), envID, req.RunID, req.Owner, time.Duration(req.TTLSeconds)*time.Second)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, leaseResponse{
		ID: lease.ID, TeamID: lease.TeamID, EnvID: lease.EnvID, RunID: lease.RunID,
		Owner: lease.Owner, LeaseUntil: lease.LeaseUntil,
	})
}

func (h *fixtureHandler) release(w http.ResponseWriter, r *http.Request) {
	envID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || envID <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid env id"))
		return
	}
	if err := h.svc.Release(r.Context(), envID); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "released"})
}

func toFixtureResponse(f *execdomain.RunFixture) fixtureResponse {
	return fixtureResponse{
		ID: f.ID, TeamID: f.TeamID, RunID: f.RunID, Namespace: f.Namespace,
		Manifest: f.Manifest, State: f.State, ExpiresAt: f.ExpiresAt, CleanupState: f.CleanupState,
	}
}
