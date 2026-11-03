package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
)

// GenerationPlanServicePort is the preflight use case consumed by the HTTP
// adapter. It exists so authorization/transport tests can exercise handlers
// without a database, mirroring the execution domain handlers.
type GenerationPlanServicePort interface {
	Preflight(ctx context.Context, cmd application.PreflightGenerationPlanCommand) (*domain.GenerationPlan, error)
	Find(ctx context.Context, teamID, planID int64) (*domain.GenerationPlan, error)
}

type generationPlanHandler struct {
	svc        GenerationPlanServicePort
	authorizer iam.AuthorizationPort
}

// RegisterGenerationPlans exposes the preflight plan API. The plan route is
// new, so it always uses the authorized handler; production and embedded
// compositions both pass an authorizer (or nil only in tests).
func RegisterGenerationPlans(mux *http.ServeMux, svc GenerationPlanServicePort, authorizer iam.AuthorizationPort) {
	h := &generationPlanHandler{svc: svc, authorizer: authorizer}
	mux.HandleFunc("POST /generation/plans", h.create)
	mux.HandleFunc("GET /generation/plans/{id}", h.get)
}

type generationPlanUnitRequest struct {
	ServiceID  int64  `json:"service_id"`
	RepoID     int64  `json:"repo_id"`
	SourcePath string `json:"source_path"`
	RefType    string `json:"ref_type"`
	RefName    string `json:"ref_name"`
}

type generationPlanCreateRequest struct {
	ProjectID int64                       `json:"project_id"`
	ScopeType string                      `json:"scope_type"`
	ScopeID   int64                       `json:"scope_id"`
	Units     []generationPlanUnitRequest `json:"units"`
}

func (h *generationPlanHandler) create(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	actor, ok := generationActor(w, r)
	if !ok {
		return
	}
	var req generationPlanCreateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "invalid generation plan request")
		return
	}
	if req.ProjectID <= 0 || !domain.ValidGenerationScopeType(req.ScopeType) {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "project_id and a valid scope_type are required")
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "edit", req.ProjectID) {
		return
	}
	units := make([]domain.GenerationPlanUnitInput, 0, len(req.Units))
	for _, unit := range req.Units {
		units = append(units, domain.GenerationPlanUnitInput{
			ServiceID: unit.ServiceID, RepoID: unit.RepoID, SourcePath: unit.SourcePath,
			RefType: unit.RefType, RefName: unit.RefName,
		})
	}
	plan, err := h.svc.Preflight(r.Context(), application.PreflightGenerationPlanCommand{
		TeamID: team, ProjectID: req.ProjectID, ScopeType: req.ScopeType, ScopeID: req.ScopeID,
		ActorID: actor, RequestID: strings.TrimSpace(r.Header.Get("X-Request-ID")), Units: units,
	})
	if err != nil {
		if errors.Is(err, application.ErrGenerationPlanValidation) {
			writeGenerationError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeGenerationError(w, http.StatusInternalServerError, "internal", "generation preflight failed")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toGenerationPlanResponse(plan))
}

func (h *generationPlanHandler) get(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	planID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || planID <= 0 {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "invalid plan id")
		return
	}
	plan, err := h.svc.Find(r.Context(), team, planID)
	if err != nil {
		writeGenerationError(w, http.StatusNotFound, "not_found", "generation plan not found")
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "view", plan.ProjectID) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toGenerationPlanResponse(plan))
}

// toGenerationPlanResponse renders the frozen contract shape: units, invalid
// reasons, summary, estimate and risks are separate fields so the UI can render
// a blocked preflight without parsing prose.
func toGenerationPlanResponse(plan *domain.GenerationPlan) map[string]any {
	units := make([]map[string]any, 0, len(plan.Items))
	invalid := make([]map[string]any, 0)
	for _, item := range plan.Items {
		units = append(units, map[string]any{
			"service_id": item.ServiceID, "repo_id": item.RepoID, "repo_url": item.RepoURL,
			"source_path": item.SourcePath, "ref_type": item.RefType, "ref_name": item.RefName,
			"commit_sha": item.CommitSHA, "valid": item.Valid, "error": item.Error,
		})
		if item.Valid {
			continue
		}
		code, message := splitPlanItemError(item.Error)
		invalid = append(invalid, map[string]any{"service_id": item.ServiceID, "code": code, "message": message})
	}
	risks := make([]map[string]any, 0, len(plan.Risks))
	for _, risk := range plan.Risks {
		risks = append(risks, map[string]any{"code": risk.Code, "message": risk.Message, "severity": risk.Severity})
	}
	return map[string]any{
		"plan_id":    plan.ID,
		"plan_hash":  plan.PlanHash,
		"expires_at": plan.ExpiresAt,
		"units":      units,
		"invalid":    invalid,
		"summary": map[string]any{
			"services": plan.Summary.Services, "repos": plan.Summary.Repos, "batches": plan.Summary.Batches,
			"candidates_min": plan.Summary.CandidatesMin, "candidates_max": plan.Summary.CandidatesMax,
		},
		"estimate": map[string]any{
			"tokens_in": plan.Estimate.TokensIn, "tokens_out": plan.Estimate.TokensOut,
			"cost_amount": plan.Estimate.CostAmount, "currency": plan.Estimate.Currency,
			"duration_seconds": plan.Estimate.DurationSeconds, "unknown": plan.Estimate.Unknown,
		},
		"risks": risks,
	}
}

func splitPlanItemError(value string) (string, string) {
	if index := strings.Index(value, ":"); index > 0 {
		return strings.TrimSpace(value[:index]), strings.TrimSpace(value[index+1:])
	}
	if strings.TrimSpace(value) == "" {
		return "invalid", "unit failed preflight"
	}
	return "invalid", value
}
