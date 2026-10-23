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
