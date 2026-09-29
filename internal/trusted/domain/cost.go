// 可信域成本明细核心概念：口径单点（唯一写者=trusted，幂等键防重复，总览=Σ明细，历史对比）。
package domain

import (
	"context"
	"time"
)

// CostLineItem 成本明细行（append-only；UNIQUE(idempotency_key) 防重复落账）。
type CostLineItem struct {
	ID             int64
	RequestID      string // AI 网关 requestId（贯穿计量）
	IdempotencyKey string // 幂等键 = request_id + biz_point
	TeamID         int64
	TargetID       int64
	RunID          int64
	CaseID         int64
	Category       CostCategory // generate / execute（PRD 两大口径）
	BizPoint       string       // case_generate / case_execute / gate_judge / analysis
	Model          string
	TokensIn       int64
	TokensOut      int64
	UnitPrice      float64 // 单价（每千 token）
	Amount         float64 // 成本金额（元）
	OccurredAt     time.Time
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
