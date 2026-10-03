// Package redisquota contains the Redis-backed implementation boundary for
// execution quotas. It only depends on a small evaluator interface; wiring a
// concrete Redis client belongs to the composition root and is deliberately
// not claimed by this package.
package redisquota

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/execution/infra/quota"
)

// Evaluator is the provider-independent subset needed from a Redis client.
// Implementations should execute the supplied script with EVAL (or an
// equivalent atomic command) and return its integer result. The interface is
// intentionally not go-redis specific so production wiring can choose a
// client, pool, or test double without leaking it into the execution domain.
type Evaluator interface {
	Eval(context.Context, string, []string, ...string) (any, error)
}

// EvaluatorFunc adapts a function to Evaluator, useful for composition roots
// and deterministic unit tests.
type EvaluatorFunc func(context.Context, string, []string, ...string) (any, error)

func (f EvaluatorFunc) Eval(ctx context.Context, script string, keys []string, args ...string) (any, error) {
	return f(ctx, script, keys, args...)
}

// RedisQuota applies the same four-dimensional contract as quota.MemoryQuota
// using a single Redis Lua invocation per operation.
type RedisQuota struct {
	eval   Evaluator
	limits map[string]quota.Limits
}

func New(eval Evaluator, limits map[string]quota.Limits) (*RedisQuota, error) {
	if eval == nil {
		return nil, errors.New("redis quota evaluator is required")
	}
	copyLimits := make(map[string]quota.Limits, len(limits))
	for resource, lim := range limits {
		if resource == "" {
			return nil, errors.New("redis quota resource type is required")
		}
		copyLimits[resource] = lim
	}
	return &RedisQuota{eval: eval, limits: copyLimits}, nil
}

func (q *RedisQuota) Acquire(ctx context.Context, req domain.ResourceRequest) error {
	if err := validateRequest(ctx, req); err != nil {
		return err
	}
	lim, ok := q.limits[req.Type]
	if !ok {
		return fmt.Errorf("%w: resource=%s", quota.ErrQuotaExceeded, req.Type)
	}
	result, err := q.eval.Eval(ctx, acquireScript, keys(req), strconv.Itoa(req.Units), strconv.Itoa(lim.Global), strconv.Itoa(lim.Team), strconv.Itoa(lim.Target), strconv.Itoa(lim.Owner))
	if err != nil {
		return err
	}
	return operationError(result)
}

func (q *RedisQuota) Release(ctx context.Context, req domain.ResourceRequest) error {
	if err := validateRequest(ctx, req); err != nil {
		return err
	}
	result, err := q.eval.Eval(ctx, releaseScript, keys(req), strconv.Itoa(req.Units))
	if err != nil {
		return err
	}
	return operationError(result)
}

func validateRequest(ctx context.Context, req domain.ResourceRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if req.Type == "" || req.Units <= 0 || req.TeamID <= 0 || req.TargetID <= 0 || req.OwnerID <= 0 {
		return quota.ErrInvalidRequest
	}
	return nil
}

func operationError(value any) error {
	result, ok := integerResult(value)
	if !ok || result < 0 {
		if result < 0 {
			return quota.ErrInvalidRequest
		}
		return errors.New("redis quota evaluator returned a non-integer result")
	}
	if result == 0 {
		return quota.ErrQuotaExceeded
	}
	if result != 1 {
		return fmt.Errorf("redis quota evaluator returned unexpected result %d", result)
	}
	return nil
}

func integerResult(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case uint64:
		return int64(v), true
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		return n, err == nil
	case []byte:
		n, err := strconv.ParseInt(string(v), 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func keys(req domain.ResourceRequest) []string {
	// A hash tag derived from the resource type keeps all four keys in one
	// Redis Cluster slot while avoiding user-controlled braces in key names.
	sum := sha256.Sum256([]byte(req.Type))
	tag := hex.EncodeToString(sum[:])
	prefix := "gp:quota:{" + tag + "}"
	return []string{
		prefix + ":global",
		prefix + ":team:" + strconv.FormatInt(req.TeamID, 10),
		prefix + ":target:" + strconv.FormatInt(req.TeamID, 10) + ":" + strconv.FormatInt(req.TargetID, 10),
		prefix + ":owner:" + strconv.FormatInt(req.TeamID, 10) + ":" + strconv.FormatInt(req.OwnerID, 10),
	}
}

var _ domain.QuotaPort = (*RedisQuota)(nil)
