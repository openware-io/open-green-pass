// 用例库只读端口实现：execution 按被测对象读取治理域 cas_case（只读，RLS 注入租户）。
package infra

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/platform/db"
)

// CaseReader 实现 domain.CasePort：按被测对象列出活跃用例 / 取当前版本脚本。
type CaseReader struct {
	db *db.DB
}

// NewCaseReader 创建用例库只读端口。
func NewCaseReader(db *db.DB) *CaseReader { return &CaseReader{db: db} }

// ListIDsByTarget 列出被测对象下活跃用例 id。
func (r *CaseReader) ListIDsByTarget(ctx context.Context, teamID, targetID int64) ([]int64, error) {
	var ids []int64
	err := r.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id FROM cas_case WHERE team_id=$1 AND target_id=$2 AND status='active' ORDER BY id`,
			teamID, targetID)
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
	return ids, err
}

// FetchScript 取用例当前版本脚本。
func (r *CaseReader) FetchScript(ctx context.Context, teamID, caseID int64) (int, any, error) {
	var version int
	var script []byte
	err := r.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT c.current_version, v.script_json
			 FROM cas_case c LEFT JOIN cas_version v ON v.case_id=c.id AND v.version=c.current_version
			 WHERE c.team_id=$1 AND c.id=$2 AND c.status='active'`,
			teamID, caseID).Scan(&version, &script)
	})
	if err != nil {
		return 0, nil, err
	}
	var out any
	_ = json.Unmarshal(script, &out)
	return version, out, nil
}
