// Package api 治理域 HTTP 接入层：被测对象树路由（建树/绑仓库/查询）。
// 路由注册到 gateway mux（cmd/server 组装）。
package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/domain"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type handler struct {
	svc *application.TargetTreeService
}

// Register 注册治理域被测对象树路由。
func Register(mux *http.ServeMux, svc *application.TargetTreeService) {
	h := &handler{svc: svc}
	mux.HandleFunc("POST /targets", h.createTarget)
	mux.HandleFunc("POST /targets/{id}/repo", h.attachRepo)
	mux.HandleFunc("GET /targets", h.listTargets)
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
