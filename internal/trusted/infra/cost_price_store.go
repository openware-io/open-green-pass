package infra

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
)

// CostPriceStore is the tenant-scoped, effective-dated tariff store.
type CostPriceStore struct{ db *db.DB }

func NewCostPriceStore(database *db.DB) *CostPriceStore {
	return &CostPriceStore{db: database}
}

var _ domain.CostPriceRepository = (*CostPriceStore)(nil)

func (s *CostPriceStore) Create(ctx context.Context, price *domain.CostPrice) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO cost_price(id,team_id,model_key,input_per_1k,output_per_1k,
				cache_read_per_1k,currency,effective_from,created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			price.ID, price.TeamID, price.Model, price.InputUnitPrice, price.OutputUnitPrice,
			price.CacheReadUnitPrice, price.Currency, price.EffectiveFrom, price.CreatedBy)
		return err
	})
}

func (s *CostPriceStore) List(ctx context.Context, teamID int64) ([]*domain.CostPrice, error) {
	var out []*domain.CostPrice
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id,team_id,model_key,input_per_1k,output_per_1k,cache_read_per_1k,
				currency,effective_from,created_by,created_at
			FROM cost_price WHERE team_id=$1 ORDER BY model_key,effective_from DESC`, teamID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p domain.CostPrice
			if err := rows.Scan(&p.ID, &p.TeamID, &p.Model, &p.InputUnitPrice, &p.OutputUnitPrice,
				&p.CacheReadUnitPrice, &p.Currency, &p.EffectiveFrom, &p.CreatedBy, &p.CreatedAt); err != nil {
				return err
			}
			out = append(out, &p)
		}
		return rows.Err()
	})
	return out, err
}

func (s *CostPriceStore) Effective(ctx context.Context, teamID int64, model string, at time.Time) (*domain.CostPrice, error) {
	var p domain.CostPrice
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id,team_id,model_key,input_per_1k,output_per_1k,cache_read_per_1k,
				currency,effective_from,created_by,created_at
			FROM cost_price
			WHERE team_id=$1 AND model_key=$2 AND effective_from <= $3
			  AND status='active' AND (effective_to IS NULL OR effective_to > $3)
			ORDER BY effective_from DESC LIMIT 1`, teamID, model, at).Scan(
			&p.ID, &p.TeamID, &p.Model, &p.InputUnitPrice, &p.OutputUnitPrice,
			&p.CacheReadUnitPrice, &p.Currency, &p.EffectiveFrom, &p.CreatedBy, &p.CreatedAt)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrCostPriceNotFound
	}
	return &p, err
}
