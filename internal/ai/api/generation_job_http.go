// Package api DSH 生成 Job HTTP 接入。
package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	aidomain "github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

// GenerationJobServicePort is the DSH job use case consumed by the HTTP
// adapter, so authorization/transport tests do not need a database.
type GenerationJobServicePort interface {
	Create(ctx context.Context, in application.CreateGenerationJobInput) (*aidomain.GenerationJob, bool, error)
	Get(ctx context.Context, id int64) (*aidomain.GenerationJob, error)
	Cancel(ctx context.Context, id int64) error
}

type generationJobHandler struct {
	svc        GenerationJobServicePort
	authorizer iam.AuthorizationPort
}

// RegisterGenerationJobs registers the DSH job control-plane routes without
// authorization for embedded/test compositions.
func RegisterGenerationJobs(mux *http.ServeMux, svc GenerationJobServicePort) {
	registerGenerationJobs(mux, svc, nil)
}

// RegisterGenerationJobsAuthorized registers the DSH job routes with
// project-level authorization: create/cancel require edit, get requires view.
func RegisterGenerationJobsAuthorized(mux *http.ServeMux, svc GenerationJobServicePort, authorizer iam.AuthorizationPort) {
	registerGenerationJobs(mux, svc, authorizer)
}

func registerGenerationJobs(mux *http.ServeMux, svc GenerationJobServicePort, authorizer iam.AuthorizationPort) {
	h := &generationJobHandler{svc: svc, authorizer: authorizer}
	mux.HandleFunc("POST /ai/generation-jobs", h.create)
	mux.HandleFunc("GET /ai/generation-jobs/{id}", h.get)
	mux.HandleFunc("DELETE /ai/generation-jobs/{id}", h.cancel)
}

type generationJobRequest struct {
	ProjectID      int64  `json:"project_id"`
	ServiceID      *int64 `json:"service_id"`
	Module         string `json:"module"`
	Prompt         string `json:"prompt"`
	TimeoutMS      int64  `json:"timeout_ms"`
	IdempotencyKey string `json:"idempotency_key"`
}

type generationJobResponse struct {
	ID            int64  `json:"id"`
	TeamID        int64  `json:"team_id"`
	ProjectID     int64  `json:"project_id"`
	ServiceID     *int64 `json:"service_id,omitempty"`
	Module        string `json:"module"`
	State         string `json:"state"`
	ExternalJobID string `json:"external_job_id,omitempty"`
	Output        string `json:"output,omitempty"`
	Error         string `json:"error,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func (h *generationJobHandler) create(w http.ResponseWriter, r *http.Request) {
	if _, ok := generationTenant(w, r); !ok {
		return
	}
	if _, ok := generationActor(w, r); !ok {
		return
	}
	var req generationJobRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	// Authorize before any side effect; the request project is the asset.
	if !authorizeGeneration(w, r, h.authorizer, "edit", req.ProjectID) {
		return
	}
	job, created, err := h.svc.Create(r.Context(), application.CreateGenerationJobInput{
		ProjectID: req.ProjectID, ServiceID: req.ServiceID, Module: req.Module,
		Prompt: req.Prompt, TimeoutMS: req.TimeoutMS, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	httpx.WriteJSON(w, status, toGenerationJobResponse(job))
}

func (h *generationJobHandler) get(w http.ResponseWriter, r *http.Request) {
	if _, ok := generationTenant(w, r); !ok {
		return
	}
	if _, ok := generationActor(w, r); !ok {
		return
	}
	id, err := parseGenerationJobID(r)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	// The job is read tenant-scoped first so a guessed id cannot authorize
	// against another team, then the project grant is checked.
	job, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "view", job.ProjectID) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toGenerationJobResponse(job))
}

func (h *generationJobHandler) cancel(w http.ResponseWriter, r *http.Request) {
	if _, ok := generationTenant(w, r); !ok {
		return
	}
	if _, ok := generationActor(w, r); !ok {
		return
	}
	id, err := parseGenerationJobID(r)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	job, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "edit", job.ProjectID) {
		return
	}
	if err := h.svc.Cancel(r.Context(), id); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "cancelled"})
}

func parseGenerationJobID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, gperr.Validation("invalid generation job id")
	}
	return id, nil
}

func toGenerationJobResponse(job *aidomain.GenerationJob) generationJobResponse {
	return generationJobResponse{
		ID: job.ID, TeamID: job.TeamID, ProjectID: job.ProjectID, ServiceID: job.ServiceID,
		Module: job.Module, State: string(job.State), ExternalJobID: job.ExternalJobID,
		Output: job.Output, Error: job.Error,
		CreatedAt: job.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: job.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
