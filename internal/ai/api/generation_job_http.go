// Package api DSH 生成 Job HTTP 接入。
package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	aidomain "github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type generationJobHandler struct {
	svc *application.GenerationJobService
}

// RegisterGenerationJobs registers the DSH job control-plane routes.
func RegisterGenerationJobs(mux *http.ServeMux, svc *application.GenerationJobService) {
	h := &generationJobHandler{svc: svc}
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
	var req generationJobRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
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
	httpx.WriteJSON(w, http.StatusOK, toGenerationJobResponse(job))
}

func (h *generationJobHandler) cancel(w http.ResponseWriter, r *http.Request) {
	id, err := parseGenerationJobID(r)
	if err != nil {
		httpx.WriteErr(w, err)
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
