package infra

import (
	"context"
	"errors"
	"strings"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	trusteddomain "github.com/openware-io/open-green-pass/internal/trusted/domain"
)

// GenerationPriceAdapter resolves the effective model tariff for plan
// estimation through the trusted cost ledger. A missing tariff returns
// (nil, nil) so preflight reports price_missing instead of inventing a price.
type GenerationPriceAdapter struct {
	prices *trustedapp.CostPriceService
}

func NewGenerationPriceAdapter(prices *trustedapp.CostPriceService) *GenerationPriceAdapter {
	return &GenerationPriceAdapter{prices: prices}
}

var _ domain.GenerationPlanPricePort = (*GenerationPriceAdapter)(nil)

func (a *GenerationPriceAdapter) EffectiveGenerationPrice(ctx context.Context, model string) (*domain.GenerationModelPrice, error) {
	if a == nil || a.prices == nil || strings.TrimSpace(model) == "" {
		return nil, nil
	}
	price, err := a.prices.Effective(ctx, model)
	if errors.Is(err, trusteddomain.ErrCostPriceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if price == nil {
		return nil, nil
	}
	return &domain.GenerationModelPrice{
		Model:           price.Model,
		InputUnitPrice:  price.InputUnitPrice,
		OutputUnitPrice: price.OutputUnitPrice,
		Currency:        price.Currency,
	}, nil
}
