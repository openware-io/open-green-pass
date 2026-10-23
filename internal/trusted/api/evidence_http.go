// Package api 缺陷证据元数据 HTTP 接入（Trusted）。
package api

import (
	"io"
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/trusted/application"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type evidenceHandler struct{ svc *application.EvidenceService }

// RegisterEvidence registers evidence metadata routes.
func RegisterEvidence(mux *http.ServeMux, svc *application.EvidenceService) {
	h := &evidenceHandler{svc: svc}
	mux.HandleFunc("POST /findings/{id}/evidence", h.create)
	mux.HandleFunc("GET /findings/{id}/evidence", h.list)
	mux.HandleFunc("GET /findings/{id}/evidence/{eid}", h.get)
	mux.HandleFunc("POST /findings/{id}/evidence/presign", h.presign)
	mux.HandleFunc("GET /findings/{id}/evidence/{eid}/download-url", h.downloadURL)
	mux.HandleFunc("POST /findings/{id}/evidence/upload", h.upload)
	mux.HandleFunc("GET /findings/{id}/evidence/{eid}/content", h.content)
}

type evidenceRequest struct {
	Kind         string  `json:"kind"`
	StorageURI   string  `json:"storage_uri"`
	MimeType     string  `json:"mime_type"`
	SizeBytes    int64   `json:"size_bytes"`
	Width        *int    `json:"width,omitempty"`
	Height       *int    `json:"height,omitempty"`
	DurationMS   *int    `json:"duration_ms,omitempty"`
	SHA256       string  `json:"sha256"`
	ThumbnailURI *string `json:"thumbnail_uri,omitempty"`
	Redacted     bool    `json:"redacted"`
}

type evidenceResponse struct {
	ID           int64   `json:"id"`
	TeamID       int64   `json:"team_id"`
	FindingID    int64   `json:"finding_id"`
	Kind         string  `json:"kind"`
	StorageURI   string  `json:"storage_uri"`
	MimeType     string  `json:"mime_type"`
	SizeBytes    int64   `json:"size_bytes"`
	Width        *int    `json:"width,omitempty"`
	Height       *int    `json:"height,omitempty"`
	DurationMS   *int    `json:"duration_ms,omitempty"`
	SHA256       string  `json:"sha256"`
	ThumbnailURI *string `json:"thumbnail_uri,omitempty"`
	Redacted     bool    `json:"redacted"`
	CreatedBy    int64   `json:"created_by"`
}

func (h *evidenceHandler) create(w http.ResponseWriter, r *http.Request) {
	findingID, ok := findingID(w, r)
	if !ok {
		return
	}
	var req evidenceRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	e, err := h.svc.Create(r.Context(), findingID, application.CreateEvidenceInput{
		Kind: req.Kind, StorageURI: req.StorageURI, MimeType: req.MimeType,
		SizeBytes: req.SizeBytes, Width: req.Width, Height: req.Height, DurationMS: req.DurationMS,
		SHA256: req.SHA256, ThumbnailURI: req.ThumbnailURI, Redacted: req.Redacted,
	})
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toEvidenceResponse(e))
}

func (h *evidenceHandler) list(w http.ResponseWriter, r *http.Request) {
	findingID, ok := findingID(w, r)
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), findingID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	out := make([]evidenceResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toEvidenceResponse(item))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type presignEvidenceRequest struct {
	Kind      string `json:"kind"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

func (h *evidenceHandler) presign(w http.ResponseWriter, r *http.Request) {
	findingID, ok := findingID(w, r)
	if !ok {
		return
	}
	var req presignEvidenceRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	result, err := h.svc.PresignUpload(r.Context(), findingID, application.PresignEvidenceInput{
		Kind: req.Kind, MimeType: req.MimeType, SizeBytes: req.SizeBytes, SHA256: req.SHA256,
	})
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, result)
}

func (h *evidenceHandler) upload(w http.ResponseWriter, r *http.Request) {
	findingID, ok := findingID(w, r)
	if !ok {
		return
	}
	kind := r.URL.Query().Get("kind")
	sha := r.URL.Query().Get("sha256")
	mimeType := r.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	limit := int64(10 << 20)
	if len(mimeType) >= 6 && mimeType[:6] == "video/" {
		limit = 100 << 20
	}
	if r.ContentLength <= 0 || r.ContentLength > limit {
		httpx.WriteErr(w, gperr.Validation("content-length is required and must be within the evidence limit"))
		return
	}
	e, err := h.svc.Upload(r.Context(), findingID, kind, mimeType, r.ContentLength, sha, http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toEvidenceResponse(e))
}

func (h *evidenceHandler) content(w http.ResponseWriter, r *http.Request) {
	findingID, ok := findingID(w, r)
	if !ok {
		return
	}
	evidenceID, err := strconv.ParseInt(r.PathValue("eid"), 10, 64)
	if err != nil || evidenceID <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid evidence id"))
		return
	}
	reader, mimeType, err := h.svc.Download(r.Context(), findingID, evidenceID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

func (h *evidenceHandler) downloadURL(w http.ResponseWriter, r *http.Request) {
	findingID, ok := findingID(w, r)
	if !ok {
		return
	}
	evidenceID, err := strconv.ParseInt(r.PathValue("eid"), 10, 64)
	if err != nil || evidenceID <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid evidence id"))
		return
	}
	url, err := h.svc.DownloadURL(r.Context(), findingID, evidenceID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"download_url": url})
}

func (h *evidenceHandler) get(w http.ResponseWriter, r *http.Request) {
	findingID, ok := findingID(w, r)
	if !ok {
		return
	}
	evidenceID, err := strconv.ParseInt(r.PathValue("eid"), 10, 64)
	if err != nil || evidenceID <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid evidence id"))
		return
	}
	e, err := h.svc.Get(r.Context(), findingID, evidenceID)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toEvidenceResponse(e))
}

func findingID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid finding id"))
		return 0, false
	}
	return id, true
}

func toEvidenceResponse(e *domain.Evidence) evidenceResponse {
	return evidenceResponse{
		ID: e.ID, TeamID: e.TeamID, FindingID: e.FindingID, Kind: e.Kind,
		StorageURI: e.StorageURI, MimeType: e.MimeType, SizeBytes: e.SizeBytes,
		Width: e.Width, Height: e.Height, DurationMS: e.DurationMS, SHA256: e.SHA256,
		ThumbnailURI: e.ThumbnailURI, Redacted: e.Redacted, CreatedBy: e.CreatedBy,
	}
}
