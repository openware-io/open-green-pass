package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// CostPriceService manages effective-dated model tariffs.
type CostPriceService struct {
	repo  domain.CostPriceRepository
	audit *AuditService
	gen   *id.Generator
	now   func() time.Time
}

func NewCostPriceService(repo domain.CostPriceRepository, audit *AuditService, gen *id.Generator) *CostPriceService {
	return &CostPriceService{repo: repo, audit: audit, gen: gen, now: func() time.Time { return time.Now().UTC() }}
}

type CreateCostPriceRequest struct {
	Model              string    `json:"model"`
	InputUnitPrice     float64   `json:"input_unit_price"`
	OutputUnitPrice    float64   `json:"output_unit_price"`
	CacheReadUnitPrice float64   `json:"cache_read_unit_price"`
	EffectiveFrom      time.Time `json:"effective_from"`
	ActorID            int64     `json:"-"`
}

func (s *CostPriceService) Create(ctx context.Context, req CreateCostPriceRequest) (*domain.CostPrice, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	req.Model = strings.TrimSpace(req.Model)
	if req.Model == "" {
		return nil, errors.New("model is required")
	}
	if req.InputUnitPrice < 0 || req.OutputUnitPrice < 0 || req.CacheReadUnitPrice < 0 {
		return nil, errors.New("cost prices must not be negative")
	}
	effective := req.EffectiveFrom.UTC()
	if effective.IsZero() {
		effective = s.now()
	}
	price := &domain.CostPrice{
		ID: s.gen.Next(), TeamID: teamID, Model: req.Model,
		InputUnitPrice: req.InputUnitPrice, OutputUnitPrice: req.OutputUnitPrice,
		CacheReadUnitPrice: req.CacheReadUnitPrice, Currency: "CNY",
		EffectiveFrom: effective, CreatedBy: req.ActorID,
	}
	if err := s.repo.Create(ctx, price); err != nil {
		return nil, err
	}
	if s.audit != nil {
		_, _ = s.audit.Append(ctx, req.ActorID, "cost.price.create", "cost_price", price.ID, map[string]any{"model": price.Model, "input": price.InputUnitPrice, "output": price.OutputUnitPrice, "effective_from": price.EffectiveFrom})
	}
	return price, nil
}

func (s *CostPriceService) List(ctx context.Context) ([]*domain.CostPrice, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.List(ctx, teamID)
}

// Effective resolves the tariff active for a model at the current time.
func (s *CostPriceService) Effective(ctx context.Context, model string) (*domain.CostPrice, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.Effective(ctx, teamID, model, s.now())
}
