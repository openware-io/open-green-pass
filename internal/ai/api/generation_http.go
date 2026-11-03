package api

import (
	"encoding/json"
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
	svc        *application.GenerationBatchService
	gen        *id.Generator
	store      *infra.GenerationBatchStore
	authorizer iam.AuthorizationPort
}

// RegisterGeneration exposes the batch routes without authorization. It is kept
// for embedded/test compositions; production must use RegisterGenerationAuthorized.
func RegisterGeneration(mux *http.ServeMux, svc *application.GenerationBatchService, gen *id.Generator, store *infra.GenerationBatchStore) {
	registerGeneration(mux, svc, gen, store, nil)
}

// RegisterGenerationAuthorized registers all batch routes with project-level
// G55 authorization (view for reads, edit for writes).
func RegisterGenerationAuthorized(mux *http.ServeMux, svc *application.GenerationBatchService, gen *id.Generator, store *infra.GenerationBatchStore, authorizer iam.AuthorizationPort) {
	registerGeneration(mux, svc, gen, store, authorizer)
}

func registerGeneration(mux *http.ServeMux, svc *application.GenerationBatchService, gen *id.Generator, store *infra.GenerationBatchStore, authorizer iam.AuthorizationPort) {
	h := &generationHandler{svc: svc, gen: gen, store: store, authorizer: authorizer}
	mux.HandleFunc("POST /generation/batches", h.create)
	mux.HandleFunc("GET /generation/batches", h.list)
	mux.HandleFunc("GET /generation/batches/{id}", h.get)
	mux.HandleFunc("GET /generation/batches/{id}/seeds", h.seeds)
	mux.HandleFunc("POST /generation/batches/{id}/review", h.review)
	mux.HandleFunc("POST /generation/batches/{id}/rollback", h.rollback)
	mux.HandleFunc("POST /generation/batches/{id}/cancel", h.cancel)
}

// authorizeBatch loads the batch within the tenant and checks target-level RBAC.
// A guessed batch id cannot be turned into authorization against another team.
func (h *generationHandler) authorizeBatch(w http.ResponseWriter, r *http.Request, batchID int64, action string) (*domain.GenerationBatch, bool) {
	if h.authorizer == nil {
		return nil, true
	}
	batch, err := h.svc.Find(r.Context(), mustTenant(r), batchID)
	if err != nil {
		writeGenerationError(w, http.StatusNotFound, "not_found", "batch_not_found")
		return nil, false
	}
	if !authorizeGeneration(w, r, h.authorizer, action, batch.TargetID) {
		return nil, false
	}
	return batch, true
}

func mustTenant(r *http.Request) int64 {
	team, _ := rls.TenantFrom(r.Context())
	return team
}

func (h *generationHandler) list(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	targetID, err := strconv.ParseInt(r.URL.Query().Get("target_id"), 10, 64)
	if err != nil || targetID <= 0 {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_target_id"})
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "view", targetID) {
		return
	}
	limit := application.DefaultGenerationBatchLimit
	if r.URL.Query().Has("limit") {
		limit, err = strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || limit < 1 || limit > application.MaxGenerationBatchLimit {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_limit"})
			return
		}
	}
	batches, err := h.svc.ListByTarget(r.Context(), team, targetID, limit)
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "generation_batches_unavailable"})
		return
	}
	response := make([]map[string]any, 0, len(batches))
	for _, batch := range batches {
		response = append(response, batchResponse(batch))
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *generationHandler) seeds(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_batch_id"})
		return
	}
	if _, ok := h.authorizeBatch(w, r, id, "view"); !ok {
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
	RefType          string `json:"ref_type"`
	Branch           string `json:"branch"`
	RequestedVersion string `json:"version"`
	IdempotencyKey   string `json:"idempotency_key"`
}

func (h *generationHandler) create(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	actor, ok := generationActor(w, r)
	if !ok {
		return
	}
	in, ok := decodeCreateBatchRequest(w, r)
	if !ok {
		return
	}
	// Authenticate/authorize before the remaining validation so malformed
	// payloads cannot be used to probe catalog existence.
	if !authorizeGeneration(w, r, h.authorizer, "edit", in.TargetID) {
		return
	}
	if !h.validateCreateBatchRequest(w, r, team, in) {
		return
	}
	b, created, err := h.svc.Create(r.Context(), application.CreateGenerationBatchCommand{ID: h.gen.Next(), TeamID: team, TargetID: in.TargetID, RepoID: in.RepoID, ServiceID: in.ServiceID, SourcePath: in.SourcePath, RefType: in.RefType, Branch: in.Branch, RequestedVersion: in.RequestedVersion, IdempotencyKey: in.IdempotencyKey, ActorID: actor})
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

// decodeCreateBatchRequest rejects system-managed fields and decodes the batch
// payload before authorization (target id itself must be valid first).
func decodeCreateBatchRequest(w http.ResponseWriter, r *http.Request) (createBatchRequest, bool) {
	var raw map[string]json.RawMessage
	if httpx.DecodeJSON(r, &raw) != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_generation_batch"})
		return createBatchRequest{}, false
	}
	if _, supplied := raw["base_batch_id"]; supplied {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "base_batch_id_is_system_managed"})
		return createBatchRequest{}, false
	}
	encoded, _ := json.Marshal(raw)
	var in createBatchRequest
	if json.Unmarshal(encoded, &in) != nil || in.TargetID <= 0 {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_generation_batch"})
		return createBatchRequest{}, false
	}
	return in, true
}

// validateCreateBatchRequest checks the remaining create fields and the
// repository-ref binding only after authorization succeeded.
func (h *generationHandler) validateCreateBatchRequest(w http.ResponseWriter, r *http.Request, team int64, in createBatchRequest) bool {
	if in.RepoID <= 0 || in.ServiceID == nil || *in.ServiceID <= 0 || in.Branch == "" || in.IdempotencyKey == "" {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_generation_batch"})
		return false
	}
	if h.store != nil {
		refType := in.RefType
		if refType == "" {
			refType = "branch"
		}
		if err := h.store.ValidateRepositoryRef(r.Context(), team, in.RepoID, refType, in.Branch); err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return false
		}
	}
	return true
}

func (h *generationHandler) get(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
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
	if !authorizeGeneration(w, r, h.authorizer, "view", b.TargetID) {
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
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	actor, ok := generationActor(w, r)
	if !ok {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if _, ok := h.authorizeBatch(w, r, id, "edit"); !ok {
		return
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

type cancelRequest struct {
	ExpectedVersion int64  `json:"expected_state_version"`
	RequestID       string `json:"request_id"`
}

type rollbackRequest struct {
	ExpectedVersion int64  `json:"expected_state_version"`
	RequestID       string `json:"request_id"`
}

func (h *generationHandler) rollback(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	actor, ok := generationActor(w, r)
	if !ok {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if _, ok := h.authorizeBatch(w, r, id, "edit"); !ok {
		return
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

func (h *generationHandler) cancel(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	actor, ok := generationActor(w, r)
	if !ok {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if _, ok := h.authorizeBatch(w, r, id, "edit"); !ok {
		return
	}
	var in cancelRequest
	if httpx.DecodeJSON(r, &in) != nil {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_cancel"})
		return
	}
	b, err := h.svc.Cancel(r.Context(), application.CancelGenerationCommand{TeamID: team, BatchID: id, ActorID: actor, ExpectedVersion: in.ExpectedVersion, RequestID: in.RequestID})
	if err != nil {
		httpx.WriteJSON(w, 409, map[string]string{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, 200, batchResponse(b))
}

func batchResponse(b *domain.GenerationBatch) map[string]any {
	return map[string]any{"id": b.ID, "team_id": b.TeamID, "target_id": b.TargetID, "repo_id": b.RepoID, "service_id": b.ServiceID, "source_path": b.SourcePath, "ref_type": b.RefType, "branch": b.Branch, "version": b.RequestedVersion, "head_sha": b.HeadSHA, "base_batch_id": b.BaseBatchID, "state": b.State, "state_version": b.StateVersion, "workflow_id": b.WorkflowID, "idempotency_key": b.IdempotencyKey, "failure_summary": b.FailureSummary, "created_at": b.CreatedAt, "updated_at": b.UpdatedAt}
}
