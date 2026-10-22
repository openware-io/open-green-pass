package dshgw

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Handler exposes the internal DSH gateway API. It is mounted only on the
// cluster-internal Service; ingress must never route to it.
type Handler struct {
	manager *Manager
	token   string
}

// NewHandler creates the internal HTTP handler.
func NewHandler(manager *Manager, token string) http.Handler {
	h := &Handler{manager: manager, token: token}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /internal/dsh/jobs", h.auth(h.create))
	mux.HandleFunc("GET /internal/dsh/jobs/{id}", h.auth(h.get))
	mux.HandleFunc("GET /internal/dsh/jobs/{id}/events", h.auth(h.events))
	mux.HandleFunc("DELETE /internal/dsh/jobs/{id}", h.auth(h.cancel))
	return mux
}

func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "gateway token required"})
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(h.token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

type createRequest struct {
	JobID          string `json:"job_id"`
	Prompt         string `json:"prompt"`
	Profile        string `json:"profile"`
	TimeoutMS      int64  `json:"timeout_ms"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.JobID) == "" || strings.TrimSpace(req.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job_id and prompt required"})
		return
	}
	job, err := h.manager.Submit(Job{
		ID: req.JobID, Prompt: req.Prompt, Profile: req.Profile, IdempotencyKey: req.IdempotencyKey,
		Timeout: time.Duration(req.TimeoutMS) * time.Millisecond,
	})
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrQueueFull {
			status = http.StatusTooManyRequests
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	job, ok := h.manager.Get(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.manager.Get(r.PathValue("id")); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	writeJSON(w, http.StatusOK, h.manager.Events(r.PathValue("id")))
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	if !h.manager.Cancel(r.PathValue("id")) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "job cannot be cancelled"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "cancelling"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
