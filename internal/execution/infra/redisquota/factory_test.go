package redisquota

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/execution/infra/quota"
)

type evaluatorFactoryFunc func(context.Context, string) (Evaluator, error)

func (f evaluatorFactoryFunc) NewEvaluator(ctx context.Context, address string) (Evaluator, error) {
	return f(ctx, address)
}

func TestNewFromFactoryUsesExplicitEvaluatorBoundary(t *testing.T) {
	called := false
	q, err := NewFromFactory(context.Background(), evaluatorFactoryFunc(func(_ context.Context, address string) (Evaluator, error) {
		called = address == "redis:6379"
		return EvaluatorFunc(func(context.Context, string, []string, ...string) (any, error) { return int64(1), nil }), nil
	}), "redis:6379", map[string]quota.Limits{"browser": {Global: 1}})
	if err != nil || q == nil || !called {
		t.Fatalf("q=%v err=%v called=%v", q, err, called)
	}
}
