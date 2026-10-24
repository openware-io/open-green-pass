// Package api 治理域覆盖账本 HTTP 接入：计划、Cell 查询、汇总。
package api

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/domain"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type coverageHandler struct {
	svc *application.CoverageService
}

// RegisterCoverage 注册覆盖账本路由。
func RegisterCoverage(mux *http.ServeMux, svc *application.CoverageService) {
	h := &coverageHandler{svc: svc}
	mux.HandleFunc("POST /coverage/plan", h.plan)
	mux.HandleFunc("GET /coverage/cells", h.cells)
	mux.HandleFunc("GET /coverage/summary", h.summary)
	mux.HandleFunc("GET /coverage/gaps", h.gaps)
}

type endpointInput struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Module      string `json:"module"`
	OperationID string `json:"operation_id"`
	AuthMode    string `json:"auth_mode"`
	SourceFile  string `json:"source_file"`
	SourceLine  int    `json:"source_line"`
}

type coveragePlanRequest struct {
	ProjectID int64           `json:"project_id"`
	ServiceID *int64          `json:"service_id,omitempty"`
	Module    string          `json:"module"`
	Endpoints []endpointInput `json:"endpoints"`
}

type coverageCellResponse struct {
	ID               int64  `json:"id"`
	TeamID           int64  `json:"team_id"`
	ProjectID        int64  `json:"project_id"`
	ServiceID        *int64 `json:"service_id,omitempty"`
	EndpointID       int64  `json:"endpoint_id,omitempty"`
	EndpointHash     string `json:"endpoint_hash"`
	Method           string `json:"method"`
	Path             string `json:"path"`
	Module           string `json:"module"`
	Dimension        string `json:"dimension"`
	EquivalenceClass string `json:"equivalence_class"`
	Role             string `json:"role"`
	State            string `json:"state"`
	Required         bool   `json:"required"`
	Priority         string `json:"priority"`
	Status           string `json:"status"`
	CaseID           *int64 `json:"case_id,omitempty"`
	WaiverID         *int64 `json:"waiver_id,omitempty"`
}

func (h *coverageHandler) plan(w http.ResponseWriter, r *http.Request) {
	var req coveragePlanRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	endpoints := make([]domain.Endpoint, 0, len(req.Endpoints))
	for _, in := range req.Endpoints {
		endpoints = append(endpoints, domain.Endpoint{
			Module: in.Module, Method: in.Method, Path: in.Path, OperationID: in.OperationID,
			AuthMode: in.AuthMode, SourceFile: in.SourceFile, SourceLine: in.SourceLine,
		})
	}
	cells, summary, err := h.svc.Plan(r.Context(), req.ProjectID, req.ServiceID, req.Module, endpoints)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"cells": toCoverageCellResponses(cells), "summary": summary,
	})
}

func (h *coverageHandler) cells(w http.ResponseWriter, r *http.Request) {
	filter, err := coverageFilter(r)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	cells, err := h.svc.List(r.Context(), filter)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCoverageCellResponses(cells))
}

func (h *coverageHandler) summary(w http.ResponseWriter, r *http.Request) {
	filter, err := coverageFilter(r)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	summary, err := h.svc.Summary(r.Context(), filter)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, summary)
}

func (h *coverageHandler) gaps(w http.ResponseWriter, r *http.Request) {
	filter, err := coverageFilter(r)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	report, err := h.svc.Gaps(r.Context(), filter)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, report)
}

func coverageFilter(r *http.Request) (domain.CoverageFilter, error) {
	q := r.URL.Query()
	filter := domain.CoverageFilter{Module: q.Get("module"), Status: q.Get("status")}
	if raw := q.Get("project_id"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			return filter, gperr.Validation("invalid project_id")
		}
		filter.ProjectID = n
	}
	if raw := q.Get("service_id"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			return filter, gperr.Validation("invalid service_id")
		}
		filter.ServiceID = &n
	}
	if raw := q.Get("required"); raw == "true" || raw == "false" {
		v := raw == "true"
		filter.Required = &v
	}
	return filter, nil
}

func toCoverageCellResponses(cells []domain.CoverageCell) []coverageCellResponse {
	out := make([]coverageCellResponse, 0, len(cells))
	for _, c := range cells {
		out = append(out, coverageCellResponse{
			ID: c.ID, TeamID: c.TeamID, ProjectID: c.ProjectID, ServiceID: c.ServiceID,
			EndpointID: c.EndpointID, EndpointHash: c.EndpointHash, Method: c.Method, Path: c.Path,
			Module: c.Module, Dimension: c.Dimension, EquivalenceClass: c.EquivalenceClass,
			Role: c.Role, State: c.State, Required: c.Required, Priority: c.Priority,
			Status: c.Status, CaseID: c.CaseID, WaiverID: c.WaiverID,
		})
	}
	return out
}
