// Package infra 执行域基础设施：测试环境版本校验仓储实现（env_runtime/env_check，RLS 经 platform/db 注入租户）。
package infra

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// EnvStore 环境版本校验仓储实现。
type EnvStore struct {
	db  *db.DB
	gen *id.Generator
}

// NewEnvStore 创建环境版本校验仓储。
func NewEnvStore(db *db.DB, gen *id.Generator) *EnvStore {
	return &EnvStore{db: db, gen: gen}
}

// UpsertRuntime 登记/更新环境运行版本。
func (s *EnvStore) UpsertRuntime(ctx context.Context, r *domain.EnvRuntime) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO env_runtime(id, team_id, target_id, env, running_version, checked_at, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (team_id, target_id, env) DO UPDATE SET
				running_version = EXCLUDED.running_version, checked_at = EXCLUDED.checked_at,
				updated_at = now()`,
			r.ID, r.TeamID, r.TargetID, r.Env, r.RunningVersion, r.CheckedAt, r.CreatedBy)
		return err
	})
}

// FindRuntime 查询被测对象在指定环境的运行版本。
func (s *EnvStore) FindRuntime(ctx context.Context, teamID, targetID int64, env string) (*domain.EnvRuntime, error) {
	var r domain.EnvRuntime
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, team_id, target_id, env, running_version, checked_at
			FROM env_runtime WHERE team_id = $1 AND target_id = $2 AND env = $3`,
			teamID, targetID, env)
		return row.Scan(&r.ID, &r.TeamID, &r.TargetID, &r.Env, &r.RunningVersion, &r.CheckedAt)
	})
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// SaveCheck 保存版本校验记录。
func (s *EnvStore) SaveCheck(ctx context.Context, c *domain.EnvCheck) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO env_check(id, team_id, run_id, target_id, target_version, env_version, result, checked_at, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			c.ID, c.TeamID, c.RunID, c.TargetID, c.TargetVersion, c.EnvVersion,
			string(c.Result), c.CheckedAt, 0)
		return err
	})
}

// RecentChecks 查询被测对象最近校验记录。
func (s *EnvStore) RecentChecks(ctx context.Context, teamID, targetID int64, limit int) ([]*domain.EnvCheck, error) {
	var out []*domain.EnvCheck
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, team_id, run_id, target_id, target_version, env_version, result, checked_at
			FROM env_check WHERE team_id = $1 AND target_id = $2 ORDER BY checked_at DESC LIMIT $3`,
			teamID, targetID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.EnvCheck
			var result string
			if err := rows.Scan(&c.ID, &c.TeamID, &c.RunID, &c.TargetID, &c.TargetVersion, &c.EnvVersion, &result, &c.CheckedAt); err != nil {
				return err
			}
			c.Result = domain.EnvCheckResult(result)
			out = append(out, &c)
		}
		return rows.Err()
	})
	return out, err
}
