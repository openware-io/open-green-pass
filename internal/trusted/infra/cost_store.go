// 可信域成本仓储实现（cost_line_item，append-only + 幂等键 + RLS 注入租户）。
package infra

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// CostStore 成本仓储实现。
type CostStore struct {
	db  *db.DB
	gen *id.Generator
}

// NewCostStore 创建成本仓储。
func NewCostStore(db *db.DB, gen *id.Generator) *CostStore {
	return &CostStore{db: db, gen: gen}
}

// Insert 写入成本行；幂等键冲突不重复落账。
func (s *CostStore) Insert(ctx context.Context, c *domain.CostLineItem) (bool, error) {
	inserted := false
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO cost_line_item(id, request_id, idempotency_key, team_id, target_id, run_id, case_id,
				biz_category, biz_point, model, tokens_in, tokens_out, unit_price, amount, occurred_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			ON CONFLICT (idempotency_key) DO NOTHING`,
			c.ID, c.RequestID, c.IdempotencyKey, c.TeamID, c.TargetID, c.RunID, c.CaseID,
			string(c.Category), c.BizPoint, c.Model, c.TokensIn, c.TokensOut, c.UnitPrice, c.Amount, c.OccurredAt)
		if err != nil {
			return err
		}
		inserted = tag.RowsAffected() > 0
		return nil
	})
	return inserted, err
}

// Overview 成本总览（=Σ明细，按大类/细类）。
func (s *CostStore) Overview(ctx context.Context, teamID int64) (*domain.CostOverview, error) {
	ov := &domain.CostOverview{
		ByCategory: map[domain.CostCategory]float64{},
		ByPoint:    map[string]float64{},
	}
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount),0) FROM cost_line_item WHERE team_id=$1`, teamID).Scan(&ov.TotalAmount); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT biz_category, COALESCE(SUM(amount),0) FROM cost_line_item WHERE team_id=$1 GROUP BY biz_category`, teamID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var cat string
			var amt float64
			if err := rows.Scan(&cat, &amt); err != nil {
				return err
			}
			ov.ByCategory[domain.CostCategory(cat)] = amt
		}
		if err := rows.Err(); err != nil {
			return err
		}
		prows, err := tx.Query(ctx, `
			SELECT biz_point, COALESCE(SUM(amount),0) FROM cost_line_item WHERE team_id=$1 GROUP BY biz_point`, teamID)
		if err != nil {
			return err
		}
		defer prows.Close()
		for prows.Next() {
			var p string
			var amt float64
			if err := prows.Scan(&p, &amt); err != nil {
				return err
			}
			ov.ByPoint[p] = amt
		}
		return prows.Err()
	})
	if err != nil {
		return nil, err
	}
	return ov, nil
}

// Items 成本明细（按大类/细类筛选）。
func (s *CostStore) Items(ctx context.Context, teamID int64, category, point string, limit int) ([]*domain.CostLineItem, error) {
	var out []*domain.CostLineItem
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		q := `SELECT id, request_id, team_id, target_id, run_id, case_id, biz_category, biz_point,
			 model, tokens_in, tokens_out, unit_price, amount, occurred_at
			FROM cost_line_item WHERE team_id=$1`
		args := []any{teamID}
		argi := 2
		if category != "" {
			q += ` AND biz_category=$` + itoa64(argi)
			args = append(args, category)
			argi++
		}
		if point != "" {
			q += ` AND biz_point=$` + itoa64(argi)
			args = append(args, point)
			argi++
		}
		q += ` ORDER BY occurred_at DESC LIMIT $` + itoa64(argi)
		args = append(args, limit)
		rows, err := tx.Query(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.CostLineItem
			var cat string
			if err := rows.Scan(&c.ID, &c.RequestID, &c.TeamID, &c.TargetID, &c.RunID, &c.CaseID,
				&cat, &c.BizPoint, &c.Model, &c.TokensIn, &c.TokensOut, &c.UnitPrice, &c.Amount, &c.OccurredAt); err != nil {
				return err
			}
			c.Category = domain.CostCategory(cat)
			out = append(out, &c)
		}
		return rows.Err()
	})
	return out, err
}

// CompareHistory 单个用例成本历史（供存量持平/递减对比）。
func (s *CostStore) CompareHistory(ctx context.Context, teamID, caseID int64) ([]*domain.CostLineItem, error) {
	var out []*domain.CostLineItem
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, request_id, team_id, target_id, run_id, case_id, biz_category, biz_point,
			 model, tokens_in, tokens_out, unit_price, amount, occurred_at
			FROM cost_line_item WHERE team_id=$1 AND case_id=$2 ORDER BY occurred_at`,
			teamID, caseID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.CostLineItem
			var cat string
			if err := rows.Scan(&c.ID, &c.RequestID, &c.TeamID, &c.TargetID, &c.RunID, &c.CaseID,
				&cat, &c.BizPoint, &c.Model, &c.TokensIn, &c.TokensOut, &c.UnitPrice, &c.Amount, &c.OccurredAt); err != nil {
				return err
			}
			c.Category = domain.CostCategory(cat)
			out = append(out, &c)
		}
		return rows.Err()
	})
	return out, err
}

func itoa64(n int) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

var _ domain.CostRepository = (*CostStore)(nil)
