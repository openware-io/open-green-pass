package application

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type capturingCostRepo struct{ item *domain.CostLineItem }

func (r *capturingCostRepo) Insert(_ context.Context, item *domain.CostLineItem) (bool, error) {
	r.item = item
	return true, nil
}
func (r *capturingCostRepo) Overview(context.Context, int64) (*domain.CostOverview, error) {
	return &domain.CostOverview{}, nil
}
func (r *capturingCostRepo) Items(context.Context, int64, string, string, int) ([]*domain.CostLineItem, error) {
	return nil, nil
}
func (r *capturingCostRepo) CompareHistory(context.Context, int64, int64) ([]*domain.CostLineItem, error) {
	return nil, nil
}

func TestCostServiceUsesSplitTokenPricing(t *testing.T) {
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo := &capturingCostRepo{}
	svc := NewCostService(repo, NewAuditService(&atomicAuditRepo{}, gen), gen)
	ctx := rls.WithTenant(context.Background(), 7)
	priceID := int64(99)
	inserted, err := svc.Record(ctx, RecordRequest{
		RequestID: "gen-1", BizPoint: "case_generate", Model: "dsh/deepseek-flash",
		TokensIn: 1000, TokensOut: 1000, CacheReadTokens: 500,
		InputUnitPrice: 0.002, OutputUnitPrice: 0.008, CacheReadUnitPrice: 0.001,
		PriceID: &priceID,
	})
	if err != nil || !inserted {
		t.Fatalf("record split cost: inserted=%v err=%v", inserted, err)
	}
	if repo.item == nil {
		t.Fatal("cost row was not captured")
	}
	if repo.item.Amount != 0.0105 {
		t.Fatalf("amount=%v, want 0.0105", repo.item.Amount)
	}
	if repo.item.InputUnitPrice != 0.002 || repo.item.OutputUnitPrice != 0.008 || repo.item.CacheReadUnitPrice != 0.001 {
		t.Fatalf("unit prices not preserved: %+v", repo.item)
	}
	if repo.item.PriceMissing {
		t.Fatal("price_missing must be false when a price is supplied")
	}
}

func TestCostServiceMarksMissingPrice(t *testing.T) {
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo := &capturingCostRepo{}
	svc := NewCostService(repo, NewAuditService(&atomicAuditRepo{}, gen), gen)
	ctx := rls.WithTenant(context.Background(), 7)
	if _, err := svc.Record(ctx, RecordRequest{
		RequestID: "gen-2", BizPoint: "case_generate", TokensIn: 10, TokensOut: 20, PriceMissing: true,
	}); err != nil {
		t.Fatal(err)
	}
	if !repo.item.PriceMissing || repo.item.Amount != 0 {
		t.Fatalf("missing price not represented: %+v", repo.item)
	}
}
