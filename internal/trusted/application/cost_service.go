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

// RecordRequest 成本计量请求（P1 mock meter；真实 AI 网关 MeterPort 后续接入）。
type RecordRequest struct {
	RequestID      string           `json:"request_id"`
	BizPoint       string           `json:"biz_point"` // case_generate / case_execute / gate_judge / analysis
	TargetID       int64            `json:"target_id"`
	RunID          int64            `json:"run_id"`
	CaseID         int64            `json:"case_id"`
	Model          string           `json:"model"`
	TokensIn       int64            `json:"tokens_in"`
	TokensOut      int64            `json:"tokens_out"`
	UnitPrice      float64          `json:"unit_price"` // 每千 token 单价
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
	if req.UnitPrice > 0 {
		amount = float64(req.TokensIn+req.TokensOut) / 1000 * req.UnitPrice
	}
	item := &domain.CostLineItem{
		ID: s.gen.Next(), RequestID: req.RequestID,
		IdempotencyKey: req.RequestID + ":" + req.BizPoint,
		TeamID: teamID, TargetID: req.TargetID, RunID: req.RunID, CaseID: req.CaseID,
		Category: category, BizPoint: req.BizPoint, Model: req.Model,
		TokensIn: req.TokensIn, TokensOut: req.TokensOut,
		UnitPrice: req.UnitPrice, Amount: amount, OccurredAt: time.Now().UTC(),
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
