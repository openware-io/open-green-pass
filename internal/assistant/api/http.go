// Package api exposes the assistant through explicit, auditable HTTP operations.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/openware-io/open-green-pass/internal/assistant/application"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
)

type handler struct{ svc *application.Service }

func Register(mux *http.ServeMux, svc *application.Service) {
	h := &handler{svc: svc}
	mux.HandleFunc("POST /assistant/sessions", h.createSession)
	mux.HandleFunc("GET /assistant/sessions/{id}/turns", h.turns)
	mux.HandleFunc("POST /assistant/sessions/{id}/turns", h.addTurn)
	mux.HandleFunc("POST /assistant/sessions/{id}/guidance", h.startGuidance)
	mux.HandleFunc("GET /assistant/sessions/{id}/actions", h.actions)
	mux.HandleFunc("POST /assistant/sessions/{id}/actions/{actionId}/complete", h.completeAction)
	mux.HandleFunc("GET /assistant/artifacts", h.artifacts)
}

func identity(w http.ResponseWriter, r *http.Request) (int64, string, bool) {
	p, ok := iam.PrincipalFromContext(r.Context())
	if !ok || strings.TrimSpace(p.Subject) == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return 0, "", false
	}
	actor, err := strconv.ParseInt(p.Subject, 10, 64)
	if err != nil || actor <= 0 {
		http.Error(w, "invalid authenticated actor", http.StatusForbidden)
		return 0, "", false
	}
	return actor, p.Subject, true
}

func sessionID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	v, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || v <= 0 {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return 0, false
	}
	return v, true
}

func actionID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	v, e := strconv.ParseInt(r.PathValue("actionId"), 10, 64)
	if e != nil || v <= 0 {
		http.Error(w, "invalid action id", http.StatusBadRequest)
		return 0, false
	}
	return v, true
}

func writeServiceErr(w http.ResponseWriter, err error) {
	if errors.Is(err, application.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, application.ErrInvalidInput) || errors.Is(err, application.ErrInvalidAction) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if errors.Is(err, application.ErrModelUnavailable) {
		httpx.WriteJSON(w, http.StatusBadGateway, map[string]string{"code": "AI_PROVIDER_UNAVAILABLE", "msg": "AI 模型暂时不可用，请重试"})
		return
	}
	httpx.WriteErr(w, err)
}

func (h *handler) createSession(w http.ResponseWriter, r *http.Request) {
	actor, subject, ok := identity(w, r)
	if !ok {
		return
	}
	var req struct {
		Page string `json:"page"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	v, err := h.svc.CreateSession(r.Context(), actor, subject, req.Page)
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, v)
}

func (h *handler) turns(w http.ResponseWriter, r *http.Request) {
	_, subject, ok := identity(w, r)
	if !ok {
		return
	}
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	v, err := h.svc.Turns(r.Context(), subject, id)
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (h *handler) addTurn(w http.ResponseWriter, r *http.Request) {
	actor, subject, ok := identity(w, r)
	if !ok {
		return
	}
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	user, assistant, artifact, err := h.svc.AddTurn(r.Context(), actor, subject, id, req.Text)
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"user": user, "assistant": assistant, "artifact": artifact})
}

func (h *handler) startGuidance(w http.ResponseWriter, r *http.Request) {
	actor, subject, ok := identity(w, r)
	if !ok {
		return
	}
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	v, err := h.svc.StartGuidance(r.Context(), actor, subject, id)
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, v)
}

func (h *handler) actions(w http.ResponseWriter, r *http.Request) {
	_, subject, ok := identity(w, r)
	if !ok {
		return
	}
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	v, err := h.svc.Actions(r.Context(), subject, id)
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (h *handler) completeAction(w http.ResponseWriter, r *http.Request) {
	actor, subject, ok := identity(w, r)
	if !ok {
		return
	}
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	aid, ok := actionID(w, r)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	v, err := h.svc.CompleteAction(r.Context(), actor, subject, id, aid, req.Status)
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, v)
}

func (h *handler) artifacts(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := identity(w, r); !ok {
		return
	}
	v, err := h.svc.Artifacts(r.Context())
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}
