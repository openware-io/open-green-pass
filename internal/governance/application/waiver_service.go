package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type WaiverService struct {
	repo domain.WaiverRepository
}

func NewWaiverService(repo domain.WaiverRepository) *WaiverService {
	return &WaiverService{repo: repo}
}

type CreateWaiverRequest struct {
	CellIDs   []int64         `json:"cell_ids"`
	Reason    string          `json:"reason"`
	OwnerID   int64           `json:"owner_id"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
	Evidence  json.RawMessage `json:"evidence,omitempty"`
}

func (s *WaiverService) Create(ctx context.Context, in CreateWaiverRequest) ([]domain.CoverageWaiver, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	if len(in.CellIDs) == 0 {
		return nil, gperr.Validation("cell_ids required")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" {
		return nil, gperr.Validation("reason required")
	}
	if len(in.Reason) > 2000 {
		return nil, gperr.Validation("reason too long")
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now().UTC()) {
		return nil, gperr.Validation("expires_at must be in the future")
	}
	actor, _ := rls.UserFrom(ctx)
	if in.OwnerID == 0 {
		in.OwnerID = actor
	}
	if in.OwnerID == 0 {
		return nil, gperr.Validation("owner_id required")
	}
	if len(in.Evidence) == 0 {
		in.Evidence = json.RawMessage(`{}`)
	}
	var evidence any
	if err := json.Unmarshal(in.Evidence, &evidence); err != nil {
		return nil, gperr.Validation("evidence must be valid JSON")
	}
	waivers := make([]domain.CoverageWaiver, 0, len(in.CellIDs))
	for _, cellID := range in.CellIDs {
		if cellID <= 0 {
			return nil, gperr.Validation("invalid cell id")
		}
		waivers = append(waivers, domain.CoverageWaiver{
			CellID: cellID, Reason: in.Reason, OwnerID: in.OwnerID,
			ApprovedBy: &actor, ExpiresAt: in.ExpiresAt, Evidence: in.Evidence,
		})
	}
	_ = teamID
	return s.repo.CreateWaivers(ctx, teamID, waivers)
}

func (s *WaiverService) List(ctx context.Context, projectID int64) ([]domain.CoverageWaiver, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	return s.repo.ListWaivers(ctx, teamID, projectID)
}

func (s *WaiverService) Revoke(ctx context.Context, waiverID int64) error {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return gperr.Unauthorized("tenant required")
	}
	if waiverID <= 0 {
		return gperr.Validation("invalid waiver id")
	}
	return s.repo.RevokeWaiver(ctx, teamID, waiverID)
}
