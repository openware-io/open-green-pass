package infra

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
)

type WaiverStore struct {
	db  *DB
	gen interface{ Next() int64 }
}

func NewWaiverStore(db *DB, gen interface{ Next() int64 }) *WaiverStore {
	return &WaiverStore{db: db, gen: gen}
}

func (s *WaiverStore) CreateWaivers(ctx context.Context, teamID int64, waivers []domain.CoverageWaiver) ([]domain.CoverageWaiver, error) {
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		for i := range waivers {
			w := &waivers[i]
			if w.ID == 0 {
				w.ID = s.gen.Next()
			}
			evidence := w.Evidence
			if len(evidence) == 0 {
				evidence = []byte(`{}`)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO cov_waiver(id, team_id, cell_id, reason, owner_id, approved_by, expires_at, evidence)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
				w.ID, teamID, w.CellID, w.Reason, w.OwnerID, w.ApprovedBy, w.ExpiresAt, evidence); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `
				UPDATE cov_cell SET waiver_id=$1, status='waived', updated_at=now()
				WHERE team_id=$2 AND id=$3`, w.ID, teamID, w.CellID)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("coverage cell %d not found", w.CellID)
			}
			w.TeamID = teamID
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return waivers, nil
}

func (s *WaiverStore) ListWaivers(ctx context.Context, teamID, projectID int64) ([]domain.CoverageWaiver, error) {
	var out []domain.CoverageWaiver
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT w.id, w.team_id, w.cell_id, w.reason, w.owner_id, w.approved_by, w.expires_at, w.evidence, w.created_at
			FROM cov_waiver w
			JOIN cov_cell c ON c.id = w.cell_id
			WHERE w.team_id=$1 AND c.team_id=$1 AND ($2=0 OR c.project_id=$2)
			ORDER BY w.created_at DESC`, teamID, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var w domain.CoverageWaiver
			if err := rows.Scan(&w.ID, &w.TeamID, &w.CellID, &w.Reason, &w.OwnerID, &w.ApprovedBy, &w.ExpiresAt, &w.Evidence, &w.CreatedAt); err != nil {
				return err
			}
			out = append(out, w)
		}
		return rows.Err()
	})
	return out, err
}

func (s *WaiverStore) RevokeWaiver(ctx context.Context, teamID, waiverID int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var cellID int64
		if err := tx.QueryRow(ctx, `DELETE FROM cov_waiver WHERE team_id=$1 AND id=$2 RETURNING cell_id`, teamID, waiverID).Scan(&cellID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE cov_cell SET waiver_id=NULL, status='planned', updated_at=now() WHERE team_id=$1 AND id=$2`, teamID, cellID)
		return err
	})
}
