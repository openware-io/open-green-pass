// Package infra 治理域基础设施：测试用例版本化仓储实现（cas_case/cas_version，RLS 租户经 DB.WithTenant 注入）。
package infra

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// CaseStore 用例版本化仓储实现。
type CaseStore struct {
	db  *DB
	gen *id.Generator
}

// NewCaseStore 创建用例仓储。
func NewCaseStore(db *DB, gen *id.Generator) *CaseStore {
	return &CaseStore{db: db, gen: gen}
}

// SaveCase 保存用例主体（含 current_version/status）。
func (s *CaseStore) SaveCase(ctx context.Context, c *domain.Case) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO cas_case(id, team_id, target_id, code, title, kind, current_version, status, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO UPDATE SET
				target_id = EXCLUDED.target_id, code = EXCLUDED.code, title = EXCLUDED.title,
				kind = EXCLUDED.kind, current_version = EXCLUDED.current_version,
				status = EXCLUDED.status, updated_at = now()`,
			c.ID, c.TeamID, c.TargetID, c.Code, c.Title, c.Kind, c.CurrentVersion, c.Status, c.CreatedBy)
		return err
	})
}

// SaveVersion 保存用例版本（只增不改）。
func (s *CaseStore) SaveVersion(ctx context.Context, v *domain.CaseVersion) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var script []byte
		if v.ScriptJSON != nil {
			script = *v.ScriptJSON
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO cas_version(id, team_id, case_id, version, change_type, source_repo_id, source_branch, script_json, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			v.ID, v.TeamID, v.CaseID, v.Version, string(v.ChangeType),
			v.SourceRepoID, v.SourceBranch, script, v.CreatedBy)
		return err
	})
}

// FindCase 查询单个用例。
func (s *CaseStore) FindCase(ctx context.Context, teamID, id int64) (*domain.Case, error) {
	var c domain.Case
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, team_id, target_id, code, title, kind, current_version, status
			FROM cas_case WHERE id = $1 AND team_id = $2`, id, teamID)
		return scanCase(row, &c)
	})
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// FindVersion 查询指定版本（回退目标校验）。
func (s *CaseStore) FindVersion(ctx context.Context, teamID, caseID int64, version int) (*domain.CaseVersion, error) {
	var v domain.CaseVersion
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, case_id, version, change_type, source_repo_id, source_branch, script_json, approved_by, approved_at, created_by, created_at
			FROM cas_version WHERE case_id = $1 AND version = $2 AND team_id = $3`,
			caseID, version, teamID)
		return scanVersion(row, &v)
	})
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ListCases 列出用例（筛选）。
func (s *CaseStore) ListCases(ctx context.Context, teamID int64, f domain.CaseFilter) ([]*domain.Case, error) {
	where := "team_id = $1"
	args := []interface{}{teamID}
	if f.TargetID != nil {
		args = append(args, *f.TargetID)
		where += " AND target_id = $" + itoa(len(args))
	}
	if f.Kind != "" {
		args = append(args, f.Kind)
		where += " AND kind = $" + itoa(len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where += " AND status = $" + itoa(len(args))
	}
	if f.Code != "" {
		args = append(args, "%"+f.Code+"%")
		where += " AND code ILIKE $" + itoa(len(args))
	}
	var out []*domain.Case
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, team_id, target_id, code, title, kind, current_version, status
			FROM cas_case WHERE `+where+` ORDER BY target_id, code`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.Case
			if err := scanCase(rows, &c); err != nil {
				return err
			}
			out = append(out, &c)
		}
		return rows.Err()
	})
	return out, err
}

// History 查询用例全部版本（只增不改，可对比；按版本号升序）。
func (s *CaseStore) History(ctx context.Context, teamID, caseID int64) ([]*domain.CaseVersion, error) {
	var out []*domain.CaseVersion
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, case_id, version, change_type, source_repo_id, source_branch, script_json, approved_by, approved_at, created_by, created_at
			FROM cas_version WHERE case_id = $1 AND team_id = $2 ORDER BY version`, caseID, teamID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v domain.CaseVersion
			if err := scanVersion(rows, &v); err != nil {
				return err
			}
			out = append(out, &v)
		}
		return rows.Err()
	})
	return out, err
}

// scanCase 扫描用例主体行（row 或 rows 兼容）。
func scanCase(row pgx.Row, c *domain.Case) error {
	var targetID, currentVersion int64
	if err := row.Scan(&c.ID, &c.TeamID, &targetID, &c.Code, &c.Title, &c.Kind, &currentVersion, &c.Status); err != nil {
		return err
	}
	c.TargetID = targetID
	c.CurrentVersion = int(currentVersion)
	return nil
}

// scanVersion 扫描用例版本行。
func scanVersion(row pgx.Row, v *domain.CaseVersion) error {
	var version int64
	var changeType string
	var sourceRepoID *int64
	var sourceBranch *string
	var script []byte
	var approvedBy *int64
	var approvedAt *time.Time
	var createdBy int64
	var createdAt time.Time
	if err := row.Scan(&v.ID, &v.CaseID, &version, &changeType, &sourceRepoID, &sourceBranch, &script,
		&approvedBy, &approvedAt, &createdBy, &createdAt); err != nil {
		return err
	}
	v.Version = int(version)
	v.ChangeType = domain.ChangeType(changeType)
	v.SourceRepoID = sourceRepoID
	v.SourceBranch = sourceBranch
	if len(script) > 0 {
		rm := json.RawMessage(script)
		v.ScriptJSON = &rm
	}
	v.ApprovedBy = approvedBy
	v.ApprovedAt = approvedAt
	v.CreatedBy = createdBy
	v.CreatedAt = createdAt
	return nil
}
