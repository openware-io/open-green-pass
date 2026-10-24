package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/domain"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type waiverHandler struct{ svc *application.WaiverService }

func RegisterCoverageWaivers(mux *http.ServeMux, svc *application.WaiverService) {
	h := &waiverHandler{svc: svc}
	mux.HandleFunc("POST /coverage/waivers", h.create)
	mux.HandleFunc("GET /coverage/waivers", h.list)
	mux.HandleFunc("DELETE /coverage/waivers/{id}", h.revoke)
}

type waiverResponse struct {
	ID        int64           `json:"id"`
	CellID    int64           `json:"cell_id"`
	Reason    string          `json:"reason"`
	OwnerID   int64           `json:"owner_id"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
	Evidence  json.RawMessage `json:"evidence,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type jsonRawMessage = []byte

func toWaiverResponse(w domain.CoverageWaiver) waiverResponse {
	return waiverResponse{ID: w.ID, CellID: w.CellID, Reason: w.Reason, OwnerID: w.OwnerID, ExpiresAt: w.ExpiresAt, Evidence: w.Evidence, CreatedAt: w.CreatedAt}
}

func (h *waiverHandler) create(w http.ResponseWriter, r *http.Request) {
	var req application.CreateWaiverRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	waivers, err := h.svc.Create(r.Context(), req)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	out := make([]waiverResponse, 0, len(waivers))
	for _, waiver := range waivers {
		out = append(out, toWaiverResponse(waiver))
	}
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *waiverHandler) list(w http.ResponseWriter, r *http.Request) {
	var projectID int64
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			httpx.WriteErr(w, gperr.Validation("invalid project_id"))
			return
		}
		projectID = n
	}
	waivers, err := h.svc.List(r.Context(), projectID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	out := make([]waiverResponse, 0, len(waivers))
	for _, waiver := range waivers {
		out = append(out, toWaiverResponse(waiver))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *waiverHandler) revoke(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid waiver id"))
		return
	}
	if err := h.svc.Revoke(r.Context(), id); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
