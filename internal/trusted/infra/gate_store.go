// Package infra 可信域基础设施：门禁/审计仓储（RLS 经 platform/db 注入租户）。
// 写权限：gp_trusted_writer 独占（DB 层强制，P1 应用层端口约束为主，角色 grant 见 ENGINEERING-SPEC §8.1）。
package infra

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// GateStore 门禁仓储实现。
type GateStore struct {
	db  *db.DB
	gen *id.Generator
}

// NewGateStore 创建门禁仓储。
func NewGateStore(db *db.DB, gen *id.Generator) *GateStore {
	return &GateStore{db: db, gen: gen}
}

// UpsertRule 装载门禁规则。
func (s *GateStore) UpsertRule(ctx context.Context, r *domain.GateRule) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO gate_rule(id, team_id, target_id, scenario_id, rego, version, enabled)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			r.ID, r.TeamID, r.TargetID, r.ScenarioID, r.Rego, r.Version, r.Enabled)
		return err
	})
}

// ActiveRule 查询启用规则的最新版本。
func (s *GateStore) ActiveRule(ctx context.Context, teamID, targetID, scenarioID int64) (*domain.GateRule, error) {
	var r domain.GateRule
	var rego string
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, team_id, target_id, scenario_id, rego, version, enabled
			FROM gate_rule WHERE team_id=$1 AND target_id=$2 AND scenario_id=$3 AND enabled
			ORDER BY version DESC LIMIT 1`, teamID, targetID, scenarioID)
		return row.Scan(&r.ID, &r.TeamID, &r.TargetID, &r.ScenarioID, &rego, &r.Version, &r.Enabled)
	})
	if err != nil {
		return nil, err
	}
	r.Rego = rego
	return &r, nil
}

// SaveResult 保存门禁判定结果。
func (s *GateStore) SaveResult(ctx context.Context, r *domain.GateResult) error {
	d, _ := json.Marshal(r.Detail)
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO gate_result(id, team_id, run_id, rule_id, result, detail, decided_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			r.ID, r.TeamID, r.RunID, r.RuleID, string(r.Result), d, r.DecidedAt)
		return err
	})
}

// ResultsByRun 查询运行的门禁判定结果。
func (s *GateStore) ResultsByRun(ctx context.Context, teamID, runID int64) ([]*domain.GateResult, error) {
	var out []*domain.GateResult
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, run_id, rule_id, result, detail, decided_at
			FROM gate_result WHERE team_id=$1 AND run_id=$2 ORDER BY decided_at`, teamID, runID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r domain.GateResult
			var result string
			var d []byte
			if err := rows.Scan(&r.ID, &r.RunID, &r.RuleID, &result, &d, &r.DecidedAt); err != nil {
				return err
			}
			r.Result = domain.GateDecision(result)
			r.TeamID = teamID
			_ = json.Unmarshal(d, &r.Detail)
			out = append(out, &r)
		}
		return rows.Err()
	})
	return out, err
}

var _ domain.GateRepository = (*GateStore)(nil)

// AuditStore 审计仓储实现（append-only 哈希链）。
type AuditStore struct {
	db  *db.DB
	gen *id.Generator
}

// NewAuditStore 创建审计仓储。
func NewAuditStore(db *db.DB, gen *id.Generator) *AuditStore {
	return &AuditStore{db: db, gen: gen}
}

// LatestHash 读当前租户最新审计事件 hash。
func (s *AuditStore) LatestHash(ctx context.Context, teamID int64) (string, error) {
	var h string
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT hash FROM aud_event WHERE team_id=$1 ORDER BY ts DESC, id DESC LIMIT 1`,
			teamID).Scan(&h)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	return h, err
}

// Append 追加审计事件。
func (s *AuditStore) Append(ctx context.Context, e *domain.AuditEvent) error {
	p, _ := json.Marshal(e.Payload)
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO aud_event(id, team_id, actor, op, asset, asset_id, payload, prev_hash, hash, ts)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			e.ID, e.TeamID, e.Actor, e.Op, e.Asset, e.AssetID, p, e.PrevHash, e.Hash, e.Ts)
		return err
	})
}

var _ domain.AuditRepository = (*AuditStore)(nil)

// RunStatReader 实现 trusted 域对执行域运行的只读统计（P1 跨域读）。
type RunStatReader struct{ db *db.DB }

// NewRunStatReader 创建运行统计只读端口。
func NewRunStatReader(db *db.DB) *RunStatReader { return &RunStatReader{db: db} }

// RunMeta 读取运行的目标与场景（无行→ErrRunNotFound，供 404）。
func (r *RunStatReader) RunMeta(ctx context.Context, teamID, runID int64) (int64, int64, error) {
	var targetID, scenarioID int64
	err := r.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT target_id, scenario_id FROM run_run WHERE team_id=$1 AND id=$2`,
			teamID, runID).Scan(&targetID, &scenarioID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, domain.ErrRunNotFound
	}
	return targetID, scenarioID, err
}

// RunHead 读取运行头信息（环境/版本/分支/模式/状态），供报告。
func (r *RunStatReader) RunHead(ctx context.Context, teamID, runID int64) (*domain.RunHead, error) {
	var h domain.RunHead
	err := r.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var started, ended *time.Time
		err := tx.QueryRow(ctx, `
			SELECT target_id, scenario_id, COALESCE(env,''), COALESCE(target_version,''), COALESCE(target_branch,''), COALESCE(run_mode,''), state,
			       started_at, ended_at
			FROM run_run WHERE team_id=$1 AND id=$2`, teamID, runID).
			Scan(&h.TargetID, &h.ScenarioID, &h.Env, &h.Version, &h.Branch, &h.RunMode, &h.State, &started, &ended)
		if started != nil {
			h.StartedAt = *started
		}
		if ended != nil {
			h.EndedAt = *ended
		}
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrRunNotFound
	}
	return &h, err
}

func (r *RunStatReader) CaseResults(ctx context.Context, teamID, runID int64) ([]*domain.CaseResult, error) {
	var out []*domain.CaseResult
	err := r.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT r.case_id, c.code, COALESCE(c.kind,''), r.status, r.attempt_seq,
			       r.evidence_ref, r.ai_tokens_in, r.ai_tokens_out, r.cost_amount,
			       r.started_at, r.ended_at
			FROM run_case_result r
			LEFT JOIN cas_case c ON c.id = r.case_id AND c.team_id = r.team_id
			WHERE r.team_id=$1 AND r.run_id=$2 AND r.attempt_seq = (
				SELECT max(attempt_seq) FROM run_case_result x WHERE x.run_id=r.run_id AND x.case_id=r.case_id
			)`, teamID, runID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var cr domain.CaseResult
			var ev []byte
			var cost float64
			var tokensIn, tokensOut int64
			var started, ended *time.Time
			if err := rows.Scan(&cr.CaseID, &cr.CaseCode, &cr.Kind, &cr.Status, &cr.Attempt,
				&ev, &tokensIn, &tokensOut, &cost, &started, &ended); err != nil {
				return err
			}
			cr.TokensIn, cr.TokensOut, cr.Cost = tokensIn, tokensOut, cost
			if ev != nil {
				_ = json.Unmarshal(ev, &cr.Evidence)
			}
			if started != nil {
				cr.StartedAt = *started
			}
			if ended != nil {
				cr.EndedAt = *ended
			}
			out = append(out, &cr)
		}
		return rows.Err()
	})
	return out, err
}

// CaseStats 统计运行内各用例最新 attempt 的总数/通过/失败。
func (r *RunStatReader) CaseStats(ctx context.Context, teamID, runID int64) (int64, int64, int64, error) {
	var total, pass, fail int64
	err := r.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT count(*), count(*) FILTER (WHERE status='pass'), count(*) FILTER (WHERE status='fail')
			FROM (
				SELECT DISTINCT ON (case_id) case_id, status
				FROM run_case_result
				WHERE team_id=$1 AND run_id=$2
				ORDER BY case_id, attempt_seq DESC
			) t`, teamID, runID).Scan(&total, &pass, &fail)
	})
	return total, pass, fail, err
}

var _ domain.RunStatPort = (*RunStatReader)(nil)
