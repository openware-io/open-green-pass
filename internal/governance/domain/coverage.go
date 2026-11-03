// 覆盖账本领域模型：端点 archetype → Coverage Cell → 覆盖汇总。
// 零第三方依赖；Cell ID 稳定，不依赖 Agent 自报。
package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Endpoint 契约盘点后的端点（ctr_endpoint）。
type Endpoint struct {
	ID          int64
	TeamID      int64
	ProjectID   int64
	ServiceID   *int64
	Module      string
	Method      string
	Path        string
	OperationID string
	AuthMode    string
	SourceFile  string
	SourceLine  int
	Parameters  []EndpointParameter
	ErrorCodes  []int
}

// EndpointParameter 端点参数（ctr_parameter）。
type EndpointParameter struct {
	Location string
	Name     string
	Type     string
	Required bool
	Enum     []string
	Min      *float64
	Max      *float64
	Pattern  string
}

// CoverageCell 最小覆盖单元（cov_cell）。
type CoverageCell struct {
	ID               int64
	TeamID           int64
	ProjectID        int64
	ServiceID        *int64
	EndpointID       int64
	EndpointHash     string
	Method           string
	Path             string
	Module           string
	Dimension        string
	EquivalenceClass string
	Role             string
	State            string
	Required         bool
	Priority         string
	Status           string
	CaseID           *int64
	WaiverID         *int64
	WaiverExpiresAt  *time.Time
}

// CoverageFilter 覆盖账本筛选。
type CoverageFilter struct {
	ProjectID int64
	ServiceID *int64
	Module    string
	Status    string
	Required  *bool
}

// CoverageSummary 覆盖汇总（口径可复算）。
type CoverageSummary struct {
	TotalRequired int `json:"total_required"`
	DesignCovered int `json:"design_covered"`
	Executed      int `json:"executed"`
	Passed        int `json:"passed"`
	Waived        int `json:"waived"`
	Uncovered     int `json:"uncovered"`
}

// CoverageRepository 覆盖账本仓储端口（实现负责 RLS）。
type CoverageRepository interface {
	SaveEndpoints(ctx context.Context, teamID int64, endpoints []Endpoint) ([]Endpoint, error)
	SaveCells(ctx context.Context, teamID int64, cells []CoverageCell) ([]CoverageCell, error)
	ListCells(ctx context.Context, teamID int64, f CoverageFilter) ([]CoverageCell, error)
	Summary(ctx context.Context, teamID int64, f CoverageFilter) (CoverageSummary, error)
	AssignCase(ctx context.Context, teamID int64, cellIDs []int64, caseID int64) error
	MarkCaseResult(ctx context.Context, teamID, caseID int64, status string) error
}

// BuildCoveragePlan 根据端点 archetype 枚举必需 Cell。
func BuildCoveragePlan(projectID int64, endpoints []Endpoint) []CoverageCell {
	out := make([]CoverageCell, 0, len(endpoints)*6)
	for _, ep := range endpoints {
		hash := EndpointHash(ep.Method, ep.Path)
		for _, dimension := range requiredDimensions(ep) {
			out = append(out, CoverageCell{
				ProjectID: projectID, ServiceID: ep.ServiceID, EndpointID: ep.ID,
				EndpointHash: hash, Method: ep.Method, Path: ep.Path, Module: ep.Module,
				Dimension: dimension, EquivalenceClass: cellClass(dimension),
				Role: "any", State: "none", Required: true, Priority: priorityFor(ep, dimension),
				Status: "planned",
			})
		}
	}
	return out
}

// EndpointHash 端点稳定哈希（method + 规范化 path）。
func EndpointHash(method, path string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(method)) + " " + normalizePath(path)))
	return hex.EncodeToString(sum[:])[:12]
}

func normalizePath(path string) string {
	parts := strings.Fields(strings.TrimSpace(path))
	if len(parts) == 0 {
		return "/"
	}
	p := parts[0]
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimSuffix(p, "/")
}

// statePathKeywords 是触发 state archetype 的路径关键字。
var statePathKeywords = []string{
	"submit", "approve", "reject", "revoke", "rollout",
	"pause", "resume", "withdraw", "archive",
}

// Archetype 端点类型。
func Archetype(ep Endpoint) string {
	path := strings.ToLower(ep.Path)
	if archetype := pathArchetype(path); archetype != "" {
		return archetype
	}
	return methodArchetype(ep.Method, ep.Path)
}

func pathArchetype(path string) string {
	if strings.HasPrefix(path, "/auth") || strings.Contains(path, "/auth/") {
		return "auth"
	}
	if strings.Contains(path, "upload") || strings.Contains(path, "artifact") || strings.Contains(path, "media") {
		return "upload"
	}
	for _, keyword := range statePathKeywords {
		if strings.Contains(path, keyword) {
			return "state"
		}
	}
	return ""
}

func methodArchetype(method, path string) string {
	switch strings.ToUpper(method) {
	case "GET":
		if strings.Contains(path, "{") {
			return "detail"
		}
		return "list"
	case "POST":
		if strings.Contains(path, "{") {
			return "action"
		}
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	}
	return "action"
}

func requiredDimensions(ep Endpoint) []string {
	switch Archetype(ep) {
	case "list":
		return []string{"happy", "authn", "authz", "pagination", "empty", "invalid_filter", "sort"}
	case "detail":
		return []string{"happy", "not_found", "authz", "invalid_id", "tenant_isolation"}
	case "create":
		return []string{"happy", "required_missing", "type_format", "enum", "duplicate_conflict", "authz", "side_effect"}
	case "update":
		return []string{"happy", "not_found", "partial_invalid", "stale_version", "authz", "state_transition"}
	case "delete":
		return []string{"happy", "not_found", "repeat_idempotent", "authz", "referential_conflict", "confirm_mismatch"}
	case "action":
		return []string{"happy", "not_found", "invalid_state", "authz", "idempotency", "side_effect"}
	case "state":
		return []string{"allowed_transition", "forbidden_transition", "concurrent_transition", "authz"}
	case "upload":
		return []string{"happy", "size_limit", "mime_limit", "presign_expiry", "checksum_mismatch", "cancel_resume", "authz"}
	case "auth":
		return []string{"success", "bad_credentials", "lockout_rate_limit", "expired_token", "reset_invalid"}
	}
	return []string{"happy", "authn", "authz"}
}

func cellClass(dimension string) string {
	switch dimension {
	case "happy", "success", "allowed_transition":
		return "success"
	case "authn":
		return "missing_or_expired"
	case "authz", "tenant_isolation":
		return "role_or_tenant_denied"
	case "pagination":
		return "boundary"
	case "empty":
		return "empty"
	case "invalid_filter", "invalid_id", "type_format", "partial_invalid", "required_missing", "enum", "invalid_state":
		return "invalid_input"
	case "not_found":
		return "not_found"
	case "duplicate_conflict", "stale_version", "referential_conflict", "concurrent_transition":
		return "conflict"
	case "presign_expiry", "checksum_mismatch", "size_limit", "mime_limit", "cancel_resume":
		return "boundary_or_external"
	case "lockout_rate_limit", "expired_token", "reset_invalid", "bad_credentials":
		return "auth_failure"
	default:
		return dimension
	}
}

func priorityFor(ep Endpoint, dimension string) string {
	m := strings.ToUpper(ep.Method)
	if m == "DELETE" || strings.Contains(strings.ToLower(ep.Path), "callback") {
		return "P0"
	}
	if dimension == "authz" || dimension == "authn" || dimension == "tenant_isolation" {
		return "P1"
	}
	return "P2"
}

// WaivedAt reports whether the cell is currently excluded by a waiver.
func (c CoverageCell) WaivedAt(now time.Time) bool {
	if c.WaiverID == nil {
		return c.Status == "waived"
	}
	return c.WaiverExpiresAt == nil || c.WaiverExpiresAt.After(now)
}

type CoverageGapCount struct {
	Required int `json:"required"`
	Mapped   int `json:"mapped"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Blocked  int `json:"blocked"`
	Unmapped int `json:"unmapped"`
	Waived   int `json:"waived"`
}

type CoverageGapModule struct {
	Module string `json:"module"`
	CoverageGapCount
}

type CoverageGapDimension struct {
	Dimension string `json:"dimension"`
	CoverageGapCount
}

type CoverageGapReport struct {
	ProjectID  int64                  `json:"project_id"`
	Module     string                 `json:"module,omitempty"`
	Total      CoverageGapCount       `json:"total"`
	Modules    []CoverageGapModule    `json:"modules"`
	Dimensions []CoverageGapDimension `json:"dimensions"`
}
