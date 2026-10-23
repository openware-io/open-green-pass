package infra

import (
	"context"
	"errors"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	trusteddomain "github.com/openware-io/open-green-pass/internal/trusted/domain"
)

// GenerationCostAdapter resolves the effective model tariff and records exact
// split token pricing. A missing tariff records tokens with price_missing=true
// instead of inventing a price.
type GenerationCostAdapter struct {
	cost   *trustedapp.CostService
	prices *trustedapp.CostPriceService
}

func NewGenerationCostAdapter(cost *trustedapp.CostService, prices *trustedapp.CostPriceService) *GenerationCostAdapter {
	return &GenerationCostAdapter{cost: cost, prices: prices}
}

func (a *GenerationCostAdapter) RecordGenerationCost(ctx context.Context, usage domain.GenerationCost) error {
	req := trustedapp.RecordRequest{
		RequestID:       usage.RequestID,
		BizPoint:        "case_generate",
		TargetID:        usage.TargetID,
		Model:           usage.Model,
		TokensIn:        usage.TokensIn,
		TokensOut:       usage.TokensOut,
		CacheReadTokens: usage.CacheReadTokens,
	}
	if a.prices != nil {
		price, err := a.prices.Effective(ctx, usage.Model)
		switch {
		case err == nil && price != nil:
			req.InputUnitPrice = price.InputUnitPrice
			req.OutputUnitPrice = price.OutputUnitPrice
			req.CacheReadUnitPrice = price.CacheReadUnitPrice
			req.PriceID = &price.ID
		case errors.Is(err, trusteddomain.ErrCostPriceNotFound):
			req.PriceMissing = true
		case err != nil:
			return err
		}
	} else {
		req.PriceMissing = true
	}
	_, err := a.cost.Record(ctx, req)
	return err
}
