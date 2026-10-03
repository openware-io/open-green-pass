package redisquota

import (
	"context"
	"errors"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/execution/infra/quota"
)

// EvaluatorFactory is the composition-root boundary for a concrete Redis
// client. The application package never imports a Redis SDK directly.
type EvaluatorFactory interface {
	NewEvaluator(ctx context.Context, address string) (Evaluator, error)
}

// NewFromFactory creates the shared quota port without selecting a Redis SDK
// or silently falling back to process-local state.
func NewFromFactory(ctx context.Context, factory EvaluatorFactory, address string, limits map[string]quota.Limits) (*RedisQuota, error) {
	if factory == nil {
		return nil, errors.New("redis quota evaluator factory is required")
	}
	evaluator, err := factory.NewEvaluator(ctx, address)
	if err != nil {
		return nil, err
	}
	return New(evaluator, limits)
}

var _ domain.QuotaPort = (*RedisQuota)(nil)
