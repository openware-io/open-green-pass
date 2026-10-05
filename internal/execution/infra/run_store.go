// Package infra 执行域基础设施：运行持久化（run_run/run_case_result，RLS 经 platform/db 注入租户）
// + 用例库只读端口 + 执行器（P1 Mock；K8s Job Runner 独立实现）。
package infra

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// RunStore 运行仓储实现。
type RunStore struct {
	db  *db.DB
	gen *id.Generator
}

// NewRunStore 创建运行仓储。
func NewRunStore(db *db.DB, gen *id.Generator) *RunStore {
	return &RunStore{db: db, gen: gen}
}

// Save 保存/更新运行（INSERT ... ON CONFLICT DO UPDATE 状态机推进）。
func (s *RunStore) Save(ctx context.Context, r *domain.Run) error {
	casesJSON, _ := json.Marshal(r.SelectedCases)
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO run_run(id, team_id, scenario_id, target_id, env, target_version, target_branch,
				env_version, env_check_id, run_mode, state, selected_cases, started_at, ended_at, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			ON CONFLICT (id) DO UPDATE SET
				state=EXCLUDED.state, env_version=EXCLUDED.env_version, env_check_id=EXCLUDED.env_check_id,
				started_at=EXCLUDED.started_at, ended_at=EXCLUDED.ended_at, updated_at=now()`,
			r.ID, r.TeamID, r.ScenarioID, r.TargetID, r.Env, r.TargetVersion, r.TargetBranch,
			r.EnvVersion, r.EnvCheckID, r.RunMode, string(r.State), casesJSON, r.StartedAt, r.EndedAt, 0)
		return err
	})
}

// Find 查询运行。
func (s *RunStore) Find(ctx context.Context, teamID, runID int64) (*domain.Run, error) {
	var r domain.Run
	var state, runMode, targetVersion, targetBranch, envVersion string
	var casesJSON []byte
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, team_id, scenario_id, target_id, env, target_version, target_branch,
				env_version, env_check_id, run_mode, state, selected_cases, started_at, ended_at
			FROM run_run WHERE team_id=$1 AND id=$2`, teamID, runID)
		return row.Scan(&r.ID, &r.TeamID, &r.ScenarioID, &r.TargetID, &r.Env, &targetVersion, &targetBranch,
			&envVersion, &r.EnvCheckID, &runMode, &state, &casesJSON, &r.StartedAt, &r.EndedAt)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRunNotFound
		}
		return nil, err
	}
	r.TargetVersion = targetVersion
	r.TargetBranch = targetBranch
	r.EnvVersion = envVersion
	r.RunMode = runMode
	r.State = domain.RunState(state)
	_ = json.Unmarshal(casesJSON, &r.SelectedCases)
	return &r, nil
}

// List returns tenant-scoped runs ordered newest first.
func (s *RunStore) List(ctx context.Context, teamID int64, targetID *int64, state string, limit int) ([]*domain.Run, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	out := make([]*domain.Run, 0)
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, team_id, scenario_id, target_id, env, target_version, target_branch,
				env_version, env_check_id, run_mode, state, selected_cases, started_at, ended_at
			FROM run_run
			WHERE team_id=$1
			  AND ($2::bigint IS NULL OR target_id=$2)
			  AND ($3::text = '' OR state=$3)
			ORDER BY id DESC LIMIT $4`, teamID, targetID, state, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			run := &domain.Run{}
			var stateValue, runMode, targetVersion, targetBranch, envVersion string
			var casesJSON []byte
			if err := rows.Scan(&run.ID, &run.TeamID, &run.ScenarioID, &run.TargetID, &run.Env, &targetVersion, &targetBranch,
				&envVersion, &run.EnvCheckID, &runMode, &stateValue, &casesJSON, &run.StartedAt, &run.EndedAt); err != nil {
				return err
			}
			run.TargetVersion, run.TargetBranch, run.EnvVersion = targetVersion, targetBranch, envVersion
			run.RunMode, run.State = runMode, domain.RunState(stateValue)
			_ = json.Unmarshal(casesJSON, &run.SelectedCases)
			out = append(out, run)
		}
		return rows.Err()
	})
	return out, err
}

// SaveCaseResult 保存用例执行结果。
func (s *RunStore) SaveCaseResult(ctx context.Context, c *domain.CaseResult) error {
	ev, _ := json.Marshal(c.Evidence)
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO run_case_result(id, team_id, run_id, case_id, case_version, status, result_text,
				evidence_ref, ai_tokens_in, ai_tokens_out, cost_amount, attempt_seq, started_at, ended_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			ON CONFLICT (run_id, case_id, attempt_seq) DO UPDATE SET
				status=EXCLUDED.status, result_text=EXCLUDED.result_text, evidence_ref=EXCLUDED.evidence_ref,
				ai_tokens_in=EXCLUDED.ai_tokens_in, ai_tokens_out=EXCLUDED.ai_tokens_out,
				cost_amount=EXCLUDED.cost_amount, ended_at=EXCLUDED.ended_at`,
			c.ID, c.TeamID, c.RunID, c.CaseID, c.CaseVersion, string(c.Status), c.ResultText,
			ev, c.AITokensIn, c.AITokensOut, c.CostAmount, c.AttemptSeq, c.StartedAt, c.EndedAt)
		return err
	})
}

// CaseResults 查询运行内全部用例结果。
func (s *RunStore) CaseResults(ctx context.Context, teamID, runID int64) ([]*domain.CaseResult, error) {
	var out []*domain.CaseResult
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, run_id, case_id, case_version, status, result_text, evidence_ref,
				ai_tokens_in, ai_tokens_out, cost_amount, attempt_seq, started_at, ended_at
			FROM run_case_result WHERE team_id=$1 AND run_id=$2 ORDER BY attempt_seq, case_id`, teamID, runID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.CaseResult
			var status string
			var ev []byte
			var ended *time.Time
			if err := rows.Scan(&c.ID, &c.RunID, &c.CaseID, &c.CaseVersion, &status, &c.ResultText,
				&ev, &c.AITokensIn, &c.AITokensOut, &c.CostAmount, &c.AttemptSeq, &c.StartedAt, &ended); err != nil {
				return err
			}
			c.Status = domain.CaseStatus(status)
			if len(ev) > 0 && string(ev) != "null" {
				_ = json.Unmarshal(ev, &c.Evidence)
			}
			c.TeamID = teamID
			c.EndedAt = ended
			out = append(out, &c)
		}
		return rows.Err()
	})
	return out, err
}

// MaxAttempt 查询同 run 同 case 当前最大 attempt 序号。
func (s *RunStore) MaxAttempt(ctx context.Context, teamID, runID, caseID int64) (int, error) {
	var maxAttempt int
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(attempt_seq),0) FROM run_case_result
			WHERE team_id=$1 AND run_id=$2 AND case_id=$3`, teamID, runID, caseID).Scan(&maxAttempt)
	})
	return maxAttempt, err
}

var _ domain.RunRepository = (*RunStore)(nil)
