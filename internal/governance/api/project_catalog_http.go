package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	aiinfra "github.com/openware-io/open-green-pass/internal/ai/infra"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/infra"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

type batchRepoInput struct {
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
}
type batchRepoRequest struct {
	Repositories []batchRepoInput `json:"repositories"`
}
type batchServiceInput struct {
	Language       string  `json:"language"`
	Framework      string  `json:"framework"`
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
	Kind             string `json:"kind"`
	GenerationStatus string `json:"generation_status"`
	Language         string `json:"language"`
	Framework        string `json:"framework"`
}
type batchResult struct {
	Index int    `json:"index"`
	ID    *int64 `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}
type bindSCMConnectionRequest struct {
	SCMConnectionID int64 `json:"scm_connection_id"`
}

func RegisterProjectCatalog(mux *http.ServeMux, store *infra.ProjectCatalog, generationStore *aiinfra.GenerationBatchStore) {
	mux.HandleFunc("PATCH /projects/{id}/repositories/{repoID}/scm-connection", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		projectID, projectErr := strconv.ParseInt(r.PathValue("id"), 10, 64)
		repoID, repoErr := strconv.ParseInt(r.PathValue("repoID"), 10, 64)
		var in bindSCMConnectionRequest
		if projectErr != nil || repoErr != nil || httpx.DecodeJSON(r, &in) != nil || in.SCMConnectionID <= 0 {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_scm_connection"})
			return
		}
		if err := store.BindSCMConnection(r.Context(), projectID, repoID, in.SCMConnectionID); err != nil {
			httpx.WriteJSON(w, 404, map[string]string{"error": "repository_or_scm_connection_not_found"})
			return
		}
		httpx.WriteJSON(w, 200, map[string]string{"status": "bound"})
	})
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
	mux.HandleFunc("GET /projects/{id}/repositories/{repoID}/refs", func(w http.ResponseWriter, r *http.Request) {
		projectID, projectErr := strconv.ParseInt(r.PathValue("id"), 10, 64)
		repoID, repoErr := strconv.ParseInt(r.PathValue("repoID"), 10, 64)
		teamID := principalTenantID(r)
		if projectErr != nil || repoErr != nil || teamID <= 0 {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_repository"})
			return
		}
		refs, err := generationStore.ListRepositoryRefs(r.Context(), teamID, projectID, repoID)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, 404, map[string]string{"error": "repository_not_found"})
			return
		}
		if err != nil {
			httpx.WriteJSON(w, 500, map[string]string{"error": "repository_refs_unavailable"})
			return
		}
		httpx.WriteJSON(w, 200, refs)
	})
	mux.HandleFunc("POST /projects/{id}/repositories/{repoID}/refs:refresh", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		projectID, projectErr := strconv.ParseInt(r.PathValue("id"), 10, 64)
		repoID, repoErr := strconv.ParseInt(r.PathValue("repoID"), 10, 64)
		teamID := principalTenantID(r)
		if projectErr != nil || repoErr != nil || teamID <= 0 {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_repository"})
			return
		}
		refs, err := generationStore.RefreshRepositoryRefs(r.Context(), teamID, projectID, repoID)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, 404, map[string]string{"error": "repository_not_found"})
			return
		}
		if err != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, 200, refs)
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
			v, e := store.AddService(r.Context(), infra.ProjectService{ProjectID: id, RepoID: item.RepoID, Name: item.Name, SourcePath: item.SourcePath, Kind: item.Kind, BuildContext: item.BuildContext, Language: item.Language, Framework: item.Framework, DockerfilePath: item.DockerfilePath, ManifestPath: item.ManifestPath})
			if e != nil {
				results = append(results, batchResult{Index: i, Error: e.Error()})
			} else {
				// Registration immediately starts the AI technical-profile analysis.
				// A failed analysis does not roll back the catalog row; the service is
				// visible with profile_status=failed and can be retried explicitly.
				teamID := principalTenantID(r)
				go func(serviceID int64, tenantID int64) {
					analysisCtx := rls.WithTenant(context.Background(), tenantID)
					_ = generationStore.AnalyzeServiceProfile(analysisCtx, tenantID, id, serviceID)
				}(v.ID, teamID)
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
		if projectErr != nil || serviceErr != nil || httpx.DecodeJSON(r, &in) != nil || (in.Kind == "" && in.GenerationStatus == "" && in.Language == "" && in.Framework == "") || (in.Kind != "" && !validServiceKind(in.Kind)) || (in.GenerationStatus != "" && !validGenerationStatus(in.GenerationStatus)) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_service_kind"})
			return
		}
		if err := store.UpdateService(r.Context(), projectID, serviceID, in.Kind, in.GenerationStatus, in.Language, in.Framework); err != nil {
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "service_not_found"})
			case errors.Is(err, infra.ErrServiceProfileIncomplete):
				httpx.WriteJSON(w, http.StatusConflict, map[string]string{"error": "service_profile_incomplete", "message": "请先补齐并确认服务技术画像，再启用测试用例生成"})
			default:
				httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "service_update_failed"})
			}
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	})
	mux.HandleFunc("POST /projects/{id}/services/{serviceID}/analyze-profile", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		projectID, projectErr := strconv.ParseInt(r.PathValue("id"), 10, 64)
		serviceID, serviceErr := strconv.ParseInt(r.PathValue("serviceID"), 10, 64)
		principal, ok := iam.PrincipalFromContext(r.Context())
		if projectErr != nil || serviceErr != nil || !ok {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_service"})
			return
		}
		teamID, err := strconv.ParseInt(principal.TenantID, 10, 64)
		if err != nil || teamID <= 0 {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant_required"})
			return
		}
		go func() {
			_ = generationStore.AnalyzeServiceProfile(rls.WithTenant(context.Background(), teamID), teamID, projectID, serviceID)
		}()
		httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "analyzing"})
	})
}

func principalTenantID(r *http.Request) int64 {
	p, ok := iam.PrincipalFromContext(r.Context())
	if !ok {
		return 0
	}
	id, _ := strconv.ParseInt(p.TenantID, 10, 64)
	return id
}

func validGenerationStatus(status string) bool {
	return status == "pending" || status == "confirmed"
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
