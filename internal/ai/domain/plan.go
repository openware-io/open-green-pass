package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

// GenerationPlanState is the lifecycle of a persisted generation plan. A plan
// is a read-only preflight snapshot; it never creates a run until the operator
// confirms it, and it can be consumed exactly once.
type GenerationPlanState string

const (
	// GenerationPlanDraft is a plan that is still confirmable inside its TTL.
	GenerationPlanDraft GenerationPlanState = "draft"
	// GenerationPlanConsumed is a plan already used to create one run.
	GenerationPlanConsumed GenerationPlanState = "consumed"
	// GenerationPlanExpired is a plan past its TTL that can no longer be used.
	GenerationPlanExpired GenerationPlanState = "expired"
)

// DefaultGenerationPlanTTL is the frozen contract TTL for a preflight plan.
const DefaultGenerationPlanTTL = 30 * time.Minute

var (
	// ErrGenerationPlanNotFound means the plan does not exist for the tenant.
	ErrGenerationPlanNotFound = errors.New("generation plan not found")
	// ErrGenerationPlanExpired means the plan TTL elapsed.
	ErrGenerationPlanExpired = errors.New("generation plan expired")
	// ErrGenerationPlanStale means the plan hash, tenant, project, or single-use
	// state no longer matches the confirm request; the caller must re-plan.
	ErrGenerationPlanStale = errors.New("generation plan stale")
	// ErrGenerationPlanInvalid means the plan contains units that failed
	// preflight and therefore cannot be used to create a run.
	ErrGenerationPlanInvalid = errors.New("generation plan contains invalid units")
	// ErrGenerationRunIdempotencyConflict means an idempotency key was already
	// used for a different scope or unit set.
	ErrGenerationRunIdempotencyConflict = errors.New("generation run idempotency key conflict")
)

// GenerationPlanUnitInput is one requested service/reference pair. It is both
// the wire shape and the persisted request snapshot, so retries stay auditable.
type GenerationPlanUnitInput struct {
	ServiceID  int64  `json:"service_id"`
	RepoID     int64  `json:"repo_id"`
	SourcePath string `json:"source_path"`
	RefType    string `json:"ref_type"`
	RefName    string `json:"ref_name"`
}

// GenerationPlanRequestSnapshot records exactly what was requested. It is
// persisted with the plan so a later audit can compare request and outcome.
type GenerationPlanRequestSnapshot struct {
	ProjectID int64                     `json:"project_id"`
	ScopeType string                    `json:"scope_type"`
	ScopeID   int64                     `json:"scope_id"`
	Units     []GenerationPlanUnitInput `json:"units"`
}

// GenerationPlanItem is one resolved service inside a plan. Valid=false means
// the service cannot be generated until the reported error is fixed.
type GenerationPlanItem struct {
	ServiceID  int64  `json:"service_id"`
	RepoID     int64  `json:"repo_id"`
	RepoURL    string `json:"repo_url"`
	SourcePath string `json:"source_path"`
	RefType    string `json:"ref_type"`
	RefName    string `json:"ref_name"`
	CommitSHA  string `json:"commit_sha"`
	Valid      bool   `json:"valid"`
	Error      string `json:"error"`
}

// GenerationPlanSummary aggregates plan counts. It contains no estimates; those
// live in GenerationPlanEstimate so "unknown" is never confused with zero.
type GenerationPlanSummary struct {
	Services      int `json:"services"`
	Repos         int `json:"repos"`
	Batches       int `json:"batches"`
	CandidatesMin int `json:"candidates_min"`
	CandidatesMax int `json:"candidates_max"`
}

// GenerationPlanEstimate is an explainable, historical-data-only estimate. If
// there is no approved history, Unknown stays true and the numeric fields are
// not presented as a measurement.
type GenerationPlanEstimate struct {
	TokensIn        int64   `json:"tokens_in"`
	TokensOut       int64   `json:"tokens_out"`
	CostAmount      float64 `json:"cost_amount"`
	Currency        string  `json:"currency"`
	DurationSeconds int     `json:"duration_seconds"`
	Unknown         bool    `json:"unknown"`
}

// GenerationPlanRisk is one actionable preflight warning.
type GenerationPlanRisk struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// GenerationPlan is the persisted preflight snapshot. It owns no run/batch
// state; the plan is only read again at confirmation time.
type GenerationPlan struct {
	ID        int64                         `json:"id"`
	TeamID    int64                         `json:"team_id"`
	ProjectID int64                         `json:"project_id"`
	ScopeType string                        `json:"scope_type"`
	ScopeID   int64                         `json:"scope_id"`
	PlanHash  string                        `json:"plan_hash"`
	State     GenerationPlanState           `json:"state"`
	Request   GenerationPlanRequestSnapshot `json:"request"`
	Items     []GenerationPlanItem          `json:"items"`
	Summary   GenerationPlanSummary         `json:"summary"`
	Estimate  GenerationPlanEstimate        `json:"estimate"`
	Risks     []GenerationPlanRisk          `json:"risks"`
	ExpiresAt time.Time                     `json:"expires_at"`
	// ConfirmedRunID links the plan to the run it created. It is nil until the
	// plan is consumed, which keeps single-use plans auditable.
	ConfirmedRunID *int64    `json:"confirmed_run_id,omitempty"`
	CreatedBy      int64     `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ValidGenerationScopeType reports whether scope_type is one of the three
// frozen plan scopes.
func ValidGenerationScopeType(scopeType string) bool {
	switch scopeType {
	case "project", "repo", "service":
		return true
	}
	return false
}

// EffectiveGenerationPlanUnit normalizes one requested unit before validation.
func EffectiveGenerationPlanUnit(unit GenerationPlanUnitInput) GenerationPlanUnitInput {
	unit.SourcePath = strings.TrimSpace(strings.ReplaceAll(unit.SourcePath, "\\", "/"))
	unit.RefType = strings.TrimSpace(unit.RefType)
	if unit.RefType == "" {
		unit.RefType = "branch"
	}
	unit.RefName = strings.TrimSpace(unit.RefName)
	return unit
}

// Expired reports whether the plan is past its TTL. A zero ExpiresAt is treated
// as already expired so a malformed row can never be confirmed.
func (p *GenerationPlan) Expired(now time.Time) bool {
	if p == nil || p.ExpiresAt.IsZero() {
		return true
	}
	return !now.Before(p.ExpiresAt)
}

// ValidItems returns only the confirmable units of the plan.
func (p *GenerationPlan) ValidItems() []GenerationPlanItem {
	out := make([]GenerationPlanItem, 0, len(p.Items))
	for _, item := range p.Items {
		if item.Valid {
			out = append(out, item)
		}
	}
	return out
}

// CanonicalGenerationPlanHash computes the stable plan fingerprint. It covers
// the project/scope and every resolved unit, and sorts the units first so the
// operator's selection order can never change the hash.
func CanonicalGenerationPlanHash(projectID int64, scopeType string, scopeID int64, items []GenerationPlanItem) string {
	sorted := make([]GenerationPlanItem, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.ServiceID != b.ServiceID {
			return a.ServiceID < b.ServiceID
		}
		if a.RepoID != b.RepoID {
			return a.RepoID < b.RepoID
		}
		if a.SourcePath != b.SourcePath {
			return a.SourcePath < b.SourcePath
		}
		if a.RefType != b.RefType {
			return a.RefType < b.RefType
		}
		return a.RefName < b.RefName
	})
	type hashUnit struct {
		ServiceID  int64  `json:"service_id"`
		RepoID     int64  `json:"repo_id"`
		SourcePath string `json:"source_path"`
		RefType    string `json:"ref_type"`
		RefName    string `json:"ref_name"`
		CommitSHA  string `json:"commit_sha"`
		Valid      bool   `json:"valid"`
	}
	payload := struct {
		ProjectID int64      `json:"project_id"`
		ScopeType string     `json:"scope_type"`
		ScopeID   int64      `json:"scope_id"`
		Units     []hashUnit `json:"units"`
	}{ProjectID: projectID, ScopeType: scopeType, ScopeID: scopeID, Units: make([]hashUnit, 0, len(sorted))}
	for _, item := range sorted {
		payload.Units = append(payload.Units, hashUnit{
			ServiceID: item.ServiceID, RepoID: item.RepoID, SourcePath: item.SourcePath,
			RefType: item.RefType, RefName: item.RefName, CommitSHA: item.CommitSHA, Valid: item.Valid,
		})
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		// The payload is composed of structs only, so marshaling cannot fail in
		// practice; an empty hash would make confirmation fail closed.
		return "sha256:"
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}
