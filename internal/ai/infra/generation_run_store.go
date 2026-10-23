package infra

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/platform/db"
)

// GenerationRunStore persists generation runs and their service units.
type GenerationRunStore struct {
	db *db.DB
}

// NewGenerationRunStore creates the store.
func NewGenerationRunStore(database *db.DB) *GenerationRunStore {
	return &GenerationRunStore{db: database}
}

var _ domain.GenerationRunRepository = (*GenerationRunStore)(nil)

func (s *GenerationRunStore) Create(ctx context.Context, run *domain.GenerationRun) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO gen_run(id,team_id,project_id,scope_type,scope_id,state,total_units,completed_units,failed_units,idempotency_key,created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,0,0,$8,$9)`,
			run.ID, run.TeamID, run.ProjectID, run.ScopeType, run.ScopeID, string(run.State), run.TotalUnits, run.IdempotencyKey, run.CreatedBy); err != nil {
			return err
		}
		for _, unit := range run.Units {
			if _, err := tx.Exec(ctx, `
				INSERT INTO gen_run_unit(id,team_id,run_id,service_id,repo_id,batch_id,state)
				VALUES ($1,$2,$3,$4,$5,NULL,$6)`,
				unit.ID, run.TeamID, run.ID, unit.ServiceID, unit.RepoID, string(unit.State)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GenerationRunStore) Find(ctx context.Context, teamID, runID int64) (*domain.GenerationRun, error) {
	var out *domain.GenerationRun
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var run domain.GenerationRun
		var state string
		if err := tx.QueryRow(ctx, `
			SELECT id,team_id,project_id,scope_type,scope_id,state,total_units,completed_units,failed_units,
				tokens_in,tokens_out,cost_amount,idempotency_key,created_by,created_at,updated_at
			FROM gen_run WHERE team_id=$1 AND id=$2`, teamID, runID).Scan(
			&run.ID, &run.TeamID, &run.ProjectID, &run.ScopeType, &run.ScopeID, &state,
			&run.TotalUnits, &run.CompletedUnits, &run.FailedUnits, &run.TokensIn, &run.TokensOut,
			&run.CostAmount, &run.IdempotencyKey, &run.CreatedBy, &run.CreatedAt, &run.UpdatedAt); err != nil {
			if err == pgx.ErrNoRows {
				return domain.ErrGenerationRunNotFound
			}
			return err
		}
		run.State = domain.GenerationRunState(state)
		rows, err := tx.Query(ctx, `
			SELECT id,team_id,run_id,service_id,repo_id,batch_id,state,COALESCE(error_summary,''),created_at,updated_at
			FROM gen_run_unit WHERE team_id=$1 AND run_id=$2 ORDER BY id`, teamID, runID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var unit domain.GenerationUnit
			var unitState string
			if err := rows.Scan(&unit.ID, &unit.TeamID, &unit.RunID, &unit.ServiceID, &unit.RepoID,
				&unit.BatchID, &unitState, &unit.Error, &unit.CreatedAt, &unit.UpdatedAt); err != nil {
				return err
			}
			unit.State = domain.GenerationUnitState(unitState)
			run.Units = append(run.Units, &unit)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		out = &run
		return nil
	})
	return out, err
}

func (s *GenerationRunStore) ListByProject(ctx context.Context, teamID, projectID int64, limit int) ([]*domain.GenerationRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var ids []int64
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id FROM gen_run WHERE team_id=$1 AND project_id=$2 ORDER BY created_at DESC,id DESC LIMIT $3`, teamID, projectID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.GenerationRun, 0, len(ids))
	for _, id := range ids {
		run, err := s.Find(ctx, teamID, id)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

func (s *GenerationRunStore) LinkBatch(ctx context.Context, teamID, runID, unitID, batchID int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE gen_run_unit SET batch_id=$1,state='running',updated_at=now() WHERE team_id=$2 AND run_id=$3 AND id=$4`, batchID, teamID, runID, unitID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrGenerationRunNotFound
		}
		return nil
	})
}

func (s *GenerationRunStore) SetUnitState(ctx context.Context, teamID, runID, unitID int64, state domain.GenerationUnitState, errorSummary string) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE gen_run_unit SET state=$1,error_summary=$2,updated_at=now() WHERE team_id=$3 AND run_id=$4 AND id=$5`, string(state), errorSummary, teamID, runID, unitID)
		return err
	})
}

func (s *GenerationRunStore) UpdateState(ctx context.Context, teamID, runID int64, state domain.GenerationRunState, completed, failed int) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE gen_run SET state=$1,completed_units=$2,failed_units=$3,updated_at=now() WHERE team_id=$4 AND id=$5`, string(state), completed, failed, teamID, runID)
		return err
	})
}
