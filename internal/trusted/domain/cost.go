// 可信域成本明细核心概念：口径单点（唯一写者=trusted，幂等键防重复，总览=Σ明细，历史对比）。
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrCostPriceNotFound = errors.New("model cost price not found")
	ErrCostPriceExists   = errors.New("model cost price already exists for effective time")
)

// CostLineItem 成本明细行（append-only；UNIQUE(idempotency_key) 防重复落账）。
type CostLineItem struct {
	ID                 int64
	RequestID          string // AI 网关 requestId（贯穿计量）
	IdempotencyKey     string // 幂等键 = request_id + biz_point
	TeamID             int64
	TargetID           int64
	RunID              int64
	CaseID             int64
	Category           CostCategory // generate / execute（PRD 两大口径）
	BizPoint           string       // case_generate / case_execute / gate_judge / analysis
	Model              string
	TokensIn           int64
	TokensOut          int64
	CacheReadTokens    int64
	UnitPrice          float64 // 旧版混合单价，仅兼容历史行
	InputUnitPrice     float64
	OutputUnitPrice    float64
	CacheReadUnitPrice float64
	PriceID            *int64
	PriceMissing       bool
	Amount             float64 // 成本金额（元）
	OccurredAt         time.Time
}

// CostOverview 成本总览（=Σ明细；按大类/细类聚合）。
type CostOverview struct {
	TotalAmount float64
	ByCategory  map[CostCategory]float64
	ByPoint     map[string]float64
}

// CostRepository 成本仓储端口。
type CostRepository interface {
	// Insert 写入成本行；幂等键冲突时不重复落账，返回 false。
	Insert(ctx context.Context, c *CostLineItem) (bool, error)
	Overview(ctx context.Context, teamID int64) (*CostOverview, error)
	Items(ctx context.Context, teamID int64, category, point string, limit int) ([]*CostLineItem, error)
	CompareHistory(ctx context.Context, teamID, caseID int64) ([]*CostLineItem, error)
}

// CostPrice is one effective-dated model tariff. Prices are CNY per 1K tokens.
type CostPrice struct {
	ID                 int64
	TeamID             int64
	Model              string
	InputUnitPrice     float64
	OutputUnitPrice    float64
	CacheReadUnitPrice float64
	Currency           string
	EffectiveFrom      time.Time
	CreatedBy          int64
	CreatedAt          time.Time
}

type CostPriceRepository interface {
	Create(ctx context.Context, price *CostPrice) error
	List(ctx context.Context, teamID int64) ([]*CostPrice, error)
	Effective(ctx context.Context, teamID int64, model string, at time.Time) (*CostPrice, error)
}
