package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/infra"
	"github.com/openware-io/open-green-pass/internal/iam"
)

type batchRepoInput struct {
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
}
type batchRepoRequest struct {
	Repositories []batchRepoInput `json:"repositories"`
}
type batchServiceInput struct {
	RepoID         int64   `json:"repo_id"`
	Name           string  `json:"name"`
	SourcePath     string  `json:"source_path"`
	Kind           string  `json:"kind"`
	BuildContext   string  `json:"build_context"`
	DockerfilePath *string `json:"dockerfile_path"`
	ManifestPath   *string `json:"manifest_path"`
}
type batchServiceRequest struct {
	Services []batchServiceInput `json:"services"`
}
type updateServiceRequest struct {
	Kind string `json:"kind"`
}
type batchResult struct {
	Index int    `json:"index"`
	ID    *int64 `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

func RegisterProjectCatalog(mux *http.ServeMux, store *infra.ProjectCatalog) {
	mux.HandleFunc("GET /projects/{id}/repositories", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_project"})
			return
		}
		v, err := store.ListRepositories(r.Context(), id)
		if err != nil {
			httpx.WriteJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, 200, v)
	})
	mux.HandleFunc("POST /projects/{id}/repositories:batch", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_project"})
			return
		}
		var in batchRepoRequest
		if httpx.DecodeJSON(r, &in) != nil || len(in.Repositories) == 0 || len(in.Repositories) > 100 {
			httpx.WriteJSON(w, 400, map[string]string{"error": "repositories_required"})
			return
		}
		results := make([]batchResult, 0, len(in.Repositories))
		for i, item := range in.Repositories {
			if strings.TrimSpace(item.URL) == "" {
				results = append(results, batchResult{Index: i, Error: "url required"})
				continue
			}
			v, e := store.AddRepository(r.Context(), id, item.URL, item.DefaultBranch)
			if e != nil {
				results = append(results, batchResult{Index: i, Error: e.Error()})
			} else {
				results = append(results, batchResult{Index: i, ID: &v.ID})
			}
		}
		httpx.WriteJSON(w, 207, map[string]any{"results": results})
	})
	mux.HandleFunc("GET /projects/{id}/services", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_project"})
			return
		}
		v, err := store.ListServices(r.Context(), id)
		if err != nil {
			httpx.WriteJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, 200, v)
	})
	mux.HandleFunc("POST /projects/{id}/services:batch", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_project"})
			return
		}
		var in batchServiceRequest
		if httpx.DecodeJSON(r, &in) != nil || len(in.Services) == 0 || len(in.Services) > 200 {
			httpx.WriteJSON(w, 400, map[string]string{"error": "services_required"})
			return
		}
		results := make([]batchResult, 0, len(in.Services))
		for i, item := range in.Services {
			v, e := store.AddService(r.Context(), infra.ProjectService{ProjectID: id, RepoID: item.RepoID, Name: item.Name, SourcePath: item.SourcePath, Kind: item.Kind, BuildContext: item.BuildContext, DockerfilePath: item.DockerfilePath, ManifestPath: item.ManifestPath})
			if e != nil {
				results = append(results, batchResult{Index: i, Error: e.Error()})
			} else {
				results = append(results, batchResult{Index: i, ID: &v.ID})
			}
		}
		httpx.WriteJSON(w, 207, map[string]any{"results": results})
	})
	mux.HandleFunc("PATCH /projects/{id}/services/{serviceID}", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		projectID, projectErr := strconv.ParseInt(r.PathValue("id"), 10, 64)
		serviceID, serviceErr := strconv.ParseInt(r.PathValue("serviceID"), 10, 64)
		var in updateServiceRequest
		if projectErr != nil || serviceErr != nil || httpx.DecodeJSON(r, &in) != nil || !validServiceKind(in.Kind) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_service_kind"})
			return
		}
		if err := store.UpdateServiceKind(r.Context(), projectID, serviceID, in.Kind); err != nil {
			httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "service_not_found"})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	})
}

func validServiceKind(kind string) bool {
	switch kind {
	case "api_service", "gateway", "websocket_service", "web_app", "mobile_app", "desktop_app", "other":
		return true
	default:
		return false
	}
}

func catalogEditor(w http.ResponseWriter, r *http.Request) bool {
	principal, ok := iam.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	for _, role := range principal.Roles {
		if role == "owner" || role == "admin" {
			return true
		}
	}
	httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	return false
}
