package domain

import (
	"context"
	"encoding/json"
	"time"
)

// CoverageWaiver records an explicit, auditable decision not to cover a cell.
type CoverageWaiver struct {
	ID         int64
	TeamID     int64
	CellID     int64
	Reason     string
	OwnerID    int64
	ApprovedBy *int64
	ExpiresAt  *time.Time
	Evidence   json.RawMessage
	CreatedAt  time.Time
}

// WaiverRepository persists coverage waivers and keeps cell waiver_id in sync.
type WaiverRepository interface {
	CreateWaivers(ctx context.Context, teamID int64, waivers []CoverageWaiver) ([]CoverageWaiver, error)
	ListWaivers(ctx context.Context, teamID, projectID int64) ([]CoverageWaiver, error)
	RevokeWaiver(ctx context.Context, teamID, waiverID int64) error
}
