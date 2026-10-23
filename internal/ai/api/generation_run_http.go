package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type generationRunHandler struct {
	svc *application.GenerationRunService
	gen *id.Generator
}

// RegisterGenerationRuns exposes run-level creation, progress, and cancellation.
func RegisterGenerationRuns(mux *http.ServeMux, svc *application.GenerationRunService, gen *id.Generator) {
	h := &generationRunHandler{svc: svc, gen: gen}
	mux.HandleFunc("POST /generation/runs", h.create)
	mux.HandleFunc("GET /generation/runs", h.list)
	mux.HandleFunc("GET /generation/runs/{id}", h.get)
	mux.HandleFunc("POST /generation/runs/{id}/cancel", h.cancel)
}

type generationRunUnitRequest struct {
	ServiceID  int64  `json:"service_id"`
	RepoID     int64  `json:"repo_id"`
	SourcePath string `json:"source_path"`
	RefType    string `json:"ref_type"`
	RefName    string `json:"ref_name"`
}

type generationRunCreateRequest struct {
	ProjectID      int64                      `json:"project_id"`
	ScopeType      string                     `json:"scope_type"`
	ScopeID        int64                      `json:"scope_id"`
	IdempotencyKey string                     `json:"idempotency_key"`
	Units          []generationRunUnitRequest `json:"units"`
}

func (h *generationRunHandler) create(w http.ResponseWriter, r *http.Request) {
	team, ok := rls.TenantFrom(r.Context())
	if !ok || team <= 0 {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant_required"})
		return
	}
	principal, ok := iam.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	actor, _ := strconv.ParseInt(principal.Subject, 10, 64)
	if actor <= 0 {
		actor = 1
	}
	var req generationRunCreateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_generation_run"})
		return
	}
	units := make([]application.GenerationRunUnitInput, 0, len(req.Units))
	for _, unit := range req.Units {
		units = append(units, application.GenerationRunUnitInput{
			ServiceID: unit.ServiceID, RepoID: unit.RepoID, SourcePath: unit.SourcePath,
			RefType: unit.RefType, RefName: unit.RefName,
		})
	}
	run, err := h.svc.Create(r.Context(), application.CreateGenerationRunCommand{
		ID: h.gen.Next(), TeamID: team, ProjectID: req.ProjectID, ScopeType: req.ScopeType,
		ScopeID: req.ScopeID, ActorID: actor, IdempotencyKey: req.IdempotencyKey, Units: units,
	})
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, toGenerationRunResponse(run))
}

func (h *generationRunHandler) get(w http.ResponseWriter, r *http.Request) {
	team, _ := rls.TenantFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_generation_run"})
		return
	}
	run, err := h.svc.Get(r.Context(), team, id)
	if err != nil {
		httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toGenerationRunResponse(run))
}

func (h *generationRunHandler) list(w http.ResponseWriter, r *http.Request) {
	team, _ := rls.TenantFrom(r.Context())
	projectID, err := strconv.ParseInt(r.URL.Query().Get("project_id"), 10, 64)
	if err != nil || projectID <= 0 {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_project"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := h.svc.ListByProject(r.Context(), team, projectID, limit)
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(runs))
	for _, run := range runs {
		out = append(out, toGenerationRunResponse(run))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *generationRunHandler) cancel(w http.ResponseWriter, r *http.Request) {
	team, _ := rls.TenantFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_generation_run"})
		return
	}
	principal, ok := iam.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	actor, _ := strconv.ParseInt(principal.Subject, 10, 64)
	if actor <= 0 {
		actor = 1
	}
	run, err := h.svc.Cancel(r.Context(), team, id, actor, "run-cancel-"+strconv.FormatInt(id, 10))
	if err != nil {
		httpx.WriteJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toGenerationRunResponse(run))
}

func toGenerationRunResponse(run *domain.GenerationRun) map[string]any {
	units := make([]map[string]any, 0, len(run.Units))
	for _, unit := range run.Units {
		units = append(units, map[string]any{
			"id": unit.ID, "service_id": unit.ServiceID, "repo_id": unit.RepoID,
			"batch_id": unit.BatchID, "state": unit.State, "error": unit.Error,
		})
	}
	return map[string]any{
		"id": run.ID, "team_id": run.TeamID, "project_id": run.ProjectID,
		"scope_type": run.ScopeType, "scope_id": run.ScopeID, "state": run.State,
		"total_units": run.TotalUnits, "completed_units": run.CompletedUnits,
		"failed_units": run.FailedUnits, "tokens_in": run.TokensIn, "tokens_out": run.TokensOut,
		"cost_amount": run.CostAmount, "created_at": run.CreatedAt, "updated_at": run.UpdatedAt,
		"units": units,
	}
}
