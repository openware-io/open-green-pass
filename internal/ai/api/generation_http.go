package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/ai/infra"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type generationHandler struct {
	svc   *application.GenerationBatchService
	gen   *id.Generator
	store *infra.GenerationBatchStore
}

func RegisterGeneration(mux *http.ServeMux, svc *application.GenerationBatchService, gen *id.Generator, store *infra.GenerationBatchStore) {
	h := &generationHandler{svc: svc, gen: gen, store: store}
	mux.HandleFunc("POST /generation/batches", h.create)
	mux.HandleFunc("GET /generation/batches/{id}", h.get)
	mux.HandleFunc("GET /generation/batches/{id}/seeds", h.seeds)
	mux.HandleFunc("POST /generation/batches/{id}/review", h.review)
	mux.HandleFunc("POST /generation/batches/{id}/rollback", h.rollback)
}

func (h *generationHandler) seeds(w http.ResponseWriter, r *http.Request) {
	team, ok := rls.TenantFrom(r.Context())
	if !ok {
		httpx.WriteJSON(w, 400, map[string]string{"error": "tenant_required"})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_batch_id"})
		return
	}
	seeds, err := h.store.ListSeeds(r.Context(), team, id)
	if err != nil {
		httpx.WriteJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, 200, seeds)
}

type createBatchRequest struct {
	TargetID         int64  `json:"target_id"`
	RepoID           int64  `json:"repo_id"`
	ServiceID        *int64 `json:"service_id"`
	SourcePath       string `json:"source_path"`
	Branch           string `json:"branch"`
	RequestedVersion string `json:"version"`
	IdempotencyKey   string `json:"idempotency_key"`
}

func (h *generationHandler) create(w http.ResponseWriter, r *http.Request) {
	team, ok := rls.TenantFrom(r.Context())
	if !ok {
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
	var in createBatchRequest
	if httpx.DecodeJSON(r, &in) != nil || in.TargetID <= 0 || in.RepoID <= 0 || in.Branch == "" || in.IdempotencyKey == "" {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_generation_batch"})
		return
	}
	b, created, err := h.svc.Create(r.Context(), application.CreateGenerationBatchCommand{ID: h.gen.Next(), TeamID: team, TargetID: in.TargetID, RepoID: in.RepoID, ServiceID: in.ServiceID, SourcePath: in.SourcePath, Branch: in.Branch, RequestedVersion: in.RequestedVersion, IdempotencyKey: in.IdempotencyKey, ActorID: actor})
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	httpx.WriteJSON(w, status, batchResponse(b))
}
func (h *generationHandler) get(w http.ResponseWriter, r *http.Request) {
	team, ok := rls.TenantFrom(r.Context())
	if !ok {
		httpx.WriteJSON(w, 400, map[string]string{"error": "tenant_required"})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_batch_id"})
		return
	}
	b, err := h.svc.Find(r.Context(), team, id)
	if err != nil {
		httpx.WriteJSON(w, 404, map[string]string{"error": "batch_not_found"})
		return
	}
	httpx.WriteJSON(w, 200, batchResponse(b))
}

type reviewRequest struct {
	Decision        application.GenerationReviewDecision `json:"decision"`
	ExpectedVersion int64                                `json:"expected_state_version"`
	RequestID       string                               `json:"request_id"`
}

func (h *generationHandler) review(w http.ResponseWriter, r *http.Request) {
	team, _ := rls.TenantFrom(r.Context())
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	p, ok := iam.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	actor, _ := strconv.ParseInt(p.Subject, 10, 64)
	if actor <= 0 {
		actor = 1
	}
	var in reviewRequest
	if httpx.DecodeJSON(r, &in) != nil || in.ExpectedVersion <= 0 || in.RequestID == "" {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_review"})
		return
	}
	b, err := h.svc.Review(r.Context(), application.ReviewGenerationCommand{TeamID: team, BatchID: id, ActorID: actor, ExpectedVersion: in.ExpectedVersion, Decision: in.Decision, RequestID: in.RequestID})
	if err != nil {
		httpx.WriteJSON(w, 409, map[string]string{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, 200, batchResponse(b))
}

type rollbackRequest struct {
	ExpectedVersion int64  `json:"expected_state_version"`
	RequestID       string `json:"request_id"`
}

func (h *generationHandler) rollback(w http.ResponseWriter, r *http.Request) {
	team, _ := rls.TenantFrom(r.Context())
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	p, ok := iam.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	actor, _ := strconv.ParseInt(p.Subject, 10, 64)
	if actor <= 0 {
		actor = 1
	}
	var in rollbackRequest
	if httpx.DecodeJSON(r, &in) != nil {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_rollback"})
		return
	}
	b, err := h.svc.Rollback(r.Context(), application.RollbackGenerationCommand{TeamID: team, BatchID: id, ActorID: actor, ExpectedVersion: in.ExpectedVersion, RequestID: in.RequestID})
	if err != nil {
		httpx.WriteJSON(w, 409, map[string]string{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, 200, batchResponse(b))
}
func batchResponse(b *domain.GenerationBatch) map[string]any {
	return map[string]any{"id": b.ID, "team_id": b.TeamID, "target_id": b.TargetID, "repo_id": b.RepoID, "service_id": b.ServiceID, "source_path": b.SourcePath, "branch": b.Branch, "version": b.RequestedVersion, "state": b.State, "state_version": b.StateVersion, "workflow_id": b.WorkflowID, "idempotency_key": b.IdempotencyKey, "created_at": b.CreatedAt, "updated_at": b.UpdatedAt}
}
