// Package api 治理域 HTTP 接入层：被测对象树路由（建树/绑仓库/查询）。
// 路由注册到 gateway mux（cmd/server 组装）。
package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/iam"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type handler struct {
	svc        *application.TargetTreeService
	authorizer iam.AuthorizationPort
}

// Register 注册治理域被测对象树路由。
func Register(mux *http.ServeMux, svc *application.TargetTreeService) {
	register(mux, &handler{svc: svc})
}

// RegisterAuthorized enables target-level RBAC for production composition.
func RegisterAuthorized(mux *http.ServeMux, svc *application.TargetTreeService, authorizer iam.AuthorizationPort) {
	register(mux, &handler{svc: svc, authorizer: authorizer})
}

func register(mux *http.ServeMux, h *handler) {
	mux.HandleFunc("POST /targets", h.createTarget)
	mux.HandleFunc("POST /targets/{id}/repo", h.attachRepo)
	mux.HandleFunc("GET /targets/{id}/model-binding", h.getModelBinding)
	mux.HandleFunc("PUT /targets/{id}/model-binding", h.setModelBinding)
	mux.HandleFunc("GET /targets", h.listTargets)
}

func (h *handler) authorize(w http.ResponseWriter, r *http.Request, action string, targetID int64) bool {
	if h.authorizer == nil {
		return true
	}
	principal, ok := iam.PrincipalFromContext(r.Context())
	if !ok || h.authorizer.Authorize(r.Context(), principal, action, strconv.FormatInt(targetID, 10)) != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

type modelBindingRequest struct {
	Model    string            `json:"model"`
	Provider string            `json:"provider"`
	Params   map[string]string `json:"params,omitempty"`
}

func (h *handler) getModelBinding(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid target id"))
		return
	}
	if !h.authorize(w, r, "view", id) {
		return
	}
	target, err := h.svc.FindTarget(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if target.ModelBinding == nil {
		httpx.WriteJSON(w, http.StatusOK, nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, modelBindingResponse{Model: target.ModelBinding.Model, Provider: target.ModelBinding.Provider, Params: target.ModelBinding.Params})
}

func (h *handler) setModelBinding(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteErr(w, gperr.Validation("invalid target id"))
		return
	}
	if !h.authorize(w, r, "edit", id) {
		return
	}
	var req modelBindingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	target, err := h.svc.SetModelBinding(r.Context(), id, domain.ModelBinding{Model: req.Model, Provider: req.Provider, Params: req.Params})
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toTargetResponse(target))
}

// createTargetRequest 建树节点请求。
type createTargetRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // project / service_group / service / module
	ParentID *int64 `json:"parent_id,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

func (h *handler) createTarget(w http.ResponseWriter, r *http.Request) {
	var req createTargetRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.Name == "" {
		httpx.WriteErr(w, gperr.Validation("name required"))
		return
	}
	node := domain.TargetNode{Name: req.Name, Type: domain.TargetNodeType(req.Type), ParentID: req.ParentID, Kind: req.Kind}
	tgt, err := h.svc.CreateTarget(r.Context(), node, nil)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toTargetResponse(tgt))
}

// attachRepoRequest 绑定仓库/分支/版本请求。
type attachRepoRequest struct {
	URL           string  `json:"url"`
	DefaultBranch string  `json:"default_branch,omitempty"`
	Branch        string  `json:"branch,omitempty"`
	Version       string  `json:"version,omitempty"`
	HeadSHA       *string `json:"head_sha,omitempty"`
}

func (h *handler) attachRepo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteErr(w, gperr.Validation("invalid target id"))
		return
	}
	if !h.authorize(w, r, "edit", id) {
		return
	}
	var req attachRepoRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if req.URL == "" {
		httpx.WriteErr(w, gperr.Validation("url required"))
		return
	}
	repo := domain.Repo{URL: req.URL, DefaultBranch: req.DefaultBranch}
	branch := domain.RepoBranch{Branch: req.Branch, Version: req.Version, HeadSHA: req.HeadSHA}
	if err := h.svc.AttachRepo(r.Context(), id, repo, branch); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) listTargets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := domain.TargetFilter{Name: q.Get("name")}
	if lv := q.Get("level"); lv != "" {
		if n, err := strconv.Atoi(lv); err == nil {
			f.Level = &n
		}
	}
	tgts, err := h.svc.ListTargets(r.Context(), f)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toTargetResponses(tgts))
}
