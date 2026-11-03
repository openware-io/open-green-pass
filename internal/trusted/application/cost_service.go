// 可信域成本应用服务：成本计量写入（P1 mock meter，幂等）+ 总览 / 明细 / 历史对比。
package application

import (
	"context"
	"time"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// CostService 成本应用服务（口径单点：唯一写入口，幂等落账）。
type CostService struct {
	repo  domain.CostRepository
	audit *AuditService
	gen   *id.Generator
}

// NewCostService 创建成本服务。
func NewCostService(repo domain.CostRepository, audit *AuditService, gen *id.Generator) *CostService {
	return &CostService{repo: repo, audit: audit, gen: gen}
}

// BatchCostReader is the provider-owned read contract for generation batch cost
// aggregation. It is the explicit boundary AI adapters consume; the trusted
// domain never imports AI, and the concrete implementation below owns tenant
// scoping and exact-batch matching.
type BatchCostReader interface {
	BatchCosts(ctx context.Context, teamID int64, batchIDs []int64) (map[int64]float64, error)
}

var _ BatchCostReader = (*CostService)(nil)

// RecordRequest 成本计量请求（P1 mock meter；真实 AI 网关 MeterPort 后续接入）。
type RecordRequest struct {
	RequestID string `json:"request_id"`
	BizPoint  string `json:"biz_point"` // case_generate / case_execute / gate_judge / analysis
	TargetID  int64  `json:"target_id"`
	RunID     int64  `json:"run_id"`
	// BatchID attributes case_generate charges to one generation batch; zero for
	// non-generation charges and for legacy callers.
	BatchID            int64   `json:"batch_id"`
	CaseID             int64   `json:"case_id"`
	Model              string  `json:"model"`
	TokensIn           int64   `json:"tokens_in"`
	TokensOut          int64   `json:"tokens_out"`
	CacheReadTokens    int64   `json:"cache_read_tokens"`
	UnitPrice          float64 `json:"unit_price"` // legacy blended price per 1K
	InputUnitPrice     float64 `json:"input_unit_price"`
	OutputUnitPrice    float64 `json:"output_unit_price"`
	CacheReadUnitPrice float64 `json:"cache_read_unit_price"`
	PriceID            *int64  `json:"price_id,omitempty"`
	PriceMissing       bool    `json:"price_missing"`
}

// Record 计量一条成本行（幂等：同 request_id+biz_point 不重复落账）。
func (s *CostService) Record(ctx context.Context, req RecordRequest) (bool, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return false, ErrTenantRequired
	}
	if req.BizPoint == "" {
		return false, nil
	}
	category := domain.CostExecute
	switch req.BizPoint {
	case "case_generate":
		category = domain.CostGenerate
	case "case_execute":
		category = domain.CostExecute
	case "gate_judge", "analysis":
		category = domain.CostExecute
	}
	amount := 0.0
	switch {
	case req.InputUnitPrice > 0 || req.OutputUnitPrice > 0 || req.CacheReadUnitPrice > 0:
		amount = float64(req.TokensIn)/1000*req.InputUnitPrice +
			float64(req.TokensOut)/1000*req.OutputUnitPrice +
			float64(req.CacheReadTokens)/1000*req.CacheReadUnitPrice
	case req.UnitPrice > 0:
		amount = float64(req.TokensIn+req.TokensOut) / 1000 * req.UnitPrice
	}
	item := &domain.CostLineItem{
		ID: s.gen.Next(), RequestID: req.RequestID,
		IdempotencyKey: req.RequestID + ":" + req.BizPoint,
		TeamID:         teamID, TargetID: req.TargetID, RunID: req.RunID, BatchID: req.BatchID, CaseID: req.CaseID,
		Category: category, BizPoint: req.BizPoint, Model: req.Model,
		TokensIn: req.TokensIn, TokensOut: req.TokensOut, CacheReadTokens: req.CacheReadTokens,
		UnitPrice: req.UnitPrice, InputUnitPrice: req.InputUnitPrice, OutputUnitPrice: req.OutputUnitPrice,
		CacheReadUnitPrice: req.CacheReadUnitPrice, PriceID: req.PriceID, PriceMissing: req.PriceMissing,
		Amount: amount, OccurredAt: time.Now().UTC(),
	}
	inserted, err := s.repo.Insert(ctx, item)
	if err != nil {
		return false, err
	}
	// 成本写入入审计链（口径单点可审计）
	if inserted {
		_, _ = s.audit.Append(ctx, 0, "cost.insert", "case", req.CaseID,
			map[string]any{"biz_point": req.BizPoint, "amount": amount, "idempotency_key": item.IdempotencyKey})
	}
	return inserted, nil
}

// Overview 成本总览（=Σ明细，按大类/细类）。
func (s *CostService) Overview(ctx context.Context) (*domain.CostOverview, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.Overview(ctx, teamID)
}

// Items 成本明细（按大类/细类筛选）。
func (s *CostService) Items(ctx context.Context, category, point string, limit int) ([]*domain.CostLineItem, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.Items(ctx, teamID, category, point, limit)
}

// CompareHistory 单个用例成本历史对比（显示存量持平/递减）。
func (s *CostService) CompareHistory(ctx context.Context, caseID int64) ([]*domain.CostLineItem, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.CompareHistory(ctx, teamID, caseID)
}

// BatchCosts is the stable trusted read API for generation run cost detail.
// It returns case_generate cost per batch id and only counts exact batch_id
// matches; historical batch_id IS NULL rows were removed by COST-CLEANUP, so
// there is intentionally no request_id LIKE fallback. The tenant scope is
// injected here so consumers cannot accidentally read another tenant's ledger.
func (s *CostService) BatchCosts(ctx context.Context, teamID int64, batchIDs []int64) (map[int64]float64, error) {
	if teamID <= 0 {
		return nil, ErrTenantRequired
	}
	if len(batchIDs) == 0 {
		return map[int64]float64{}, nil
	}
	return s.repo.BatchCosts(rls.WithTenant(ctx, teamID), teamID, batchIDs)
}
