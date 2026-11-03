package domain

import "context"

// GenerationCost is one measured DSH generation charge. RequestID is unique
// per batch/seed so a retried activity cannot double-charge.
type GenerationCost struct {
	RequestID       string
	TeamID          int64
	TargetID        int64
	BatchID         int64
	Model           string
	TokensIn        int64
	TokensOut       int64
	CacheReadTokens int64
	UnitPrice       float64
}

// GenerationCostPort is the generation-to-trusted measurement boundary.
type GenerationCostPort interface {
	RecordGenerationCost(context.Context, GenerationCost) error
}

// GenerationCostReadPort is the AI-side read boundary for generation cost
// aggregation. The trusted domain owns cost_line_item, so AI run reads ask for
// per-batch totals instead of joining a trusted table (one batched query, no
// N+1). Implementations must include the legacy request-id association for rows
// written before batch ids existed.
type GenerationCostReadPort interface {
	BatchCosts(ctx context.Context, teamID int64, batchIDs []int64) (map[int64]float64, error)
}
