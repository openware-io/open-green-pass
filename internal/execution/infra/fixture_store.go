// Package infra fixture/lease 仓储（run_fixture/env_lease，RLS）。
package infra

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/db"
)

// FixtureStore fixture/lease 仓储。
type FixtureStore struct {
	db *db.DB
}

// NewFixtureStore 创建仓储。
func NewFixtureStore(database *db.DB) *FixtureStore { return &FixtureStore{db: database} }

// Save upsert fixture。
func (s *FixtureStore) Save(ctx context.Context, f *domain.RunFixture) error {
	manifest, err := json.Marshal(f.Manifest)
	if err != nil {
		return err
	}
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO run_fixture(id, team_id, run_id, namespace, manifest, state, expires_at, cleanup_state)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (team_id, run_id, namespace) DO UPDATE SET
				manifest=EXCLUDED.manifest, state=EXCLUDED.state,
				expires_at=EXCLUDED.expires_at, cleanup_state=EXCLUDED.cleanup_state, updated_at=now()`,
			f.ID, f.TeamID, f.RunID, f.Namespace, manifest, f.State, f.ExpiresAt, f.CleanupState)
		return err
	})
}

// List 列出 run 的 fixtures。
func (s *FixtureStore) List(ctx context.Context, teamID, runID int64) ([]*domain.RunFixture, error) {
	var out []*domain.RunFixture
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, team_id, run_id, namespace, manifest, state, expires_at, cleanup_state, created_at, updated_at
			FROM run_fixture WHERE team_id=$1 AND run_id=$2 ORDER BY created_at`, teamID, runID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f domain.RunFixture
			var manifest []byte
			var expires *time.Time
			if err := rows.Scan(&f.ID, &f.TeamID, &f.RunID, &f.Namespace, &manifest, &f.State,
				&expires, &f.CleanupState, &f.CreatedAt, &f.UpdatedAt); err != nil {
				return err
			}
			_ = json.Unmarshal(manifest, &f.Manifest)
			f.ExpiresAt = expires
			out = append(out, &f)
		}
		return rows.Err()
	})
	return out, err
}

// Cleanup 标记 fixture 清理完成。
func (s *FixtureStore) Cleanup(ctx context.Context, teamID, runID int64, namespace string) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE run_fixture SET state='cleaned', cleanup_state='done', updated_at=now()
			WHERE team_id=$1 AND run_id=$2 AND ($3='' OR namespace=$3)`, teamID, runID, namespace)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
}

// Lease 创建/续租环境租约。
func (s *FixtureStore) Lease(ctx context.Context, lease *domain.EnvLease) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO env_lease(id, team_id, env_id, run_id, owner, lease_until)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			lease.ID, lease.TeamID, lease.EnvID, lease.RunID, lease.Owner, lease.LeaseUntil)
		return err
	})
}

// Release 释放环境租约。
func (s *FixtureStore) Release(ctx context.Context, teamID, envID int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM env_lease WHERE team_id=$1 AND env_id=$2`, teamID, envID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
}
