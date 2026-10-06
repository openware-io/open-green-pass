package domain

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidTrendBucket = errors.New("invalid cost trend bucket")

type TrendBucket string

const (
	TrendBucketHour TrendBucket = "hour"
	TrendBucketDay  TrendBucket = "day"
)

func (b TrendBucket) Duration() (time.Duration, error) {
	switch b {
	case TrendBucketHour:
		return time.Hour, nil
	case TrendBucketDay:
		return 24 * time.Hour, nil
	default:
		return 0, ErrInvalidTrendBucket
	}
}

// CostFact is a rebuildable projection row sourced only from cost_line_item.
type CostFact struct {
	SourceCostID int64
	TeamID       int64
	TargetID     int64
	RunID        int64
	CaseID       int64
	Category     CostCategory
	BizPoint     string
	Model        string
	TokensIn     int64
	TokensOut    int64
	Amount       float64
	OccurredAt   time.Time
}

type CostTrendFilter struct {
	TeamID   int64
	From     time.Time
	To       time.Time
	Bucket   TrendBucket
	TargetID int64
	CaseID   int64
	Model    string
	Category CostCategory
}

type CostTrendPoint struct {
	BucketStart time.Time
	Category    CostCategory
	Amount      float64
	TokensIn    int64
	TokensOut   int64
	CallCount   int64
}

// CostTrendRepository stores disposable facts. Upsert must be idempotent by source cost ID.
type CostTrendRepository interface {
	UpsertFact(ctx context.Context, fact CostFact) (bool, error)
	DeleteTeamFacts(ctx context.Context, teamID int64) error
	Trend(ctx context.Context, filter CostTrendFilter) ([]CostTrendPoint, error)
}
