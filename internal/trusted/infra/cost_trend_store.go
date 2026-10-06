package infra

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
)

type CostTrendStore struct{ db *db.DB }

func NewCostTrendStore(database *db.DB) *CostTrendStore { return &CostTrendStore{db: database} }

func (s *CostTrendStore) UpsertFact(ctx context.Context, fact domain.CostFact) (bool, error) {
	inserted := false
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO ts_cost_fact
			(source_cost_id, team_id, target_id, run_id, case_id, biz_category, biz_point, model,
			 tokens_in, tokens_out, amount, occurred_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			ON CONFLICT (source_cost_id) DO NOTHING`,
			fact.SourceCostID, fact.TeamID, fact.TargetID, fact.RunID, fact.CaseID, string(fact.Category),
			fact.BizPoint, fact.Model, fact.TokensIn, fact.TokensOut, fact.Amount, fact.OccurredAt)
		if err != nil {
			return err
		}
		inserted = tag.RowsAffected() == 1
		return nil
	})
	return inserted, err
}

func (s *CostTrendStore) DeleteTeamFacts(ctx context.Context, teamID int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM ts_cost_fact WHERE team_id=$1`, teamID)
		return err
	})
}

func (s *CostTrendStore) Trend(ctx context.Context, filter domain.CostTrendFilter) ([]domain.CostTrendPoint, error) {
	interval, err := trendInterval(filter.Bucket)
	if err != nil {
		return nil, err
	}
	var out []domain.CostTrendPoint
	err = s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		query := `SELECT date_trunc($1, occurred_at), biz_category, SUM(amount), SUM(tokens_in), SUM(tokens_out), COUNT(*)
			FROM ts_cost_fact WHERE team_id=$2 AND occurred_at >= $3 AND occurred_at < $4`
		args := []any{interval, filter.TeamID, filter.From, filter.To}
		conditions := []struct {
			enabled bool
			column  string
			value   any
		}{
			{filter.TargetID != 0, "target_id", filter.TargetID},
			{filter.CaseID != 0, "case_id", filter.CaseID},
			{filter.Model != "", "model", filter.Model},
			{filter.Category != "", "biz_category", string(filter.Category)},
		}
		for _, condition := range conditions {
			if condition.enabled {
				args = append(args, condition.value)
				query += fmt.Sprintf(" AND %s=$%d", condition.column, len(args))
			}
		}
		query += ` GROUP BY 1,2 ORDER BY 1,2`
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var point domain.CostTrendPoint
			var category string
			if err := rows.Scan(&point.BucketStart, &category, &point.Amount, &point.TokensIn, &point.TokensOut, &point.CallCount); err != nil {
				return err
			}
			point.Category = domain.CostCategory(category)
			out = append(out, point)
		}
		return rows.Err()
	})
	return out, err
}

func trendInterval(bucket domain.TrendBucket) (string, error) {
	if _, err := bucket.Duration(); err != nil {
		return "", err
	}
	return strings.ToLower(string(bucket)), nil
}

var _ domain.CostTrendRepository = (*CostTrendStore)(nil)
