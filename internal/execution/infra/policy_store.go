// 执行域截图策略仓储实现（svc_policy，RLS 注入租户）。
package infra

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// PolicyStore 截图策略仓储实现。
type PolicyStore struct {
	db  *db.DB
	gen *id.Generator
}

// NewPolicyStore 创建截图策略仓储。
func NewPolicyStore(db *db.DB, gen *id.Generator) *PolicyStore {
	return &PolicyStore{db: db, gen: gen}
}

// Set 下发策略（UPSERT）。
func (s *PolicyStore) Set(ctx context.Context, p *domain.ScreenshotPolicy) (*domain.ScreenshotPolicy, error) {
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO svc_policy(id, team_id, target_id, scenario_id, screenshot_enabled, updated_by)
			VALUES ($1, current_setting('gp.team_id', true)::bigint, $2, $3, $4, $5)
			ON CONFLICT (team_id, target_id, scenario_id)
			DO UPDATE SET screenshot_enabled = EXCLUDED.screenshot_enabled, updated_at = now()`,
			s.gen.Next(), p.TargetID, p.ScenarioID, p.ScreenshotEnabled, 1)
		return err
	})
	return p, err
}

// Get 读取展开策略（无行默认开启截图）。
func (s *PolicyStore) Get(ctx context.Context, teamID, targetID, scenarioID int64) (*domain.ScreenshotPolicy, error) {
	p := &domain.ScreenshotPolicy{TargetID: targetID, ScenarioID: scenarioID, ScreenshotEnabled: true}
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var enabled bool
		err := tx.QueryRow(ctx, `
			SELECT screenshot_enabled FROM svc_policy
			WHERE team_id=$1 AND target_id=$2 AND scenario_id=$3`,
			teamID, targetID, scenarioID).Scan(&enabled)
		if err == nil {
			p.ScreenshotEnabled = enabled
			return nil
		}
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	})
	return p, err
}

var _ domain.PolicyRepository = (*PolicyStore)(nil)
var _ domain.PolicyPort = (*PolicyStore)(nil)
var _ domain.PolicyWriter = (*PolicyStore)(nil)
