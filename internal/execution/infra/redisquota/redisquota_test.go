package redisquota

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/execution/infra/quota"
)

func TestRedisQuotaAcquireUsesAtomicScriptAndFourKeys(t *testing.T) {
	var gotScript string
	var gotKeys []string
	var gotArgs []string
	q, err := New(EvaluatorFunc(func(_ context.Context, script string, keys []string, args ...string) (any, error) {
		gotScript, gotKeys, gotArgs = script, keys, args
		return int64(1), nil
	}), map[string]quota.Limits{"browser": {Global: 5, Team: 3, Target: 2, Owner: 1}})
	if err != nil {
		t.Fatal(err)
	}
	err = q.Acquire(context.Background(), domain.ResourceRequest{TeamID: 1, TargetID: 2, OwnerID: 3, Type: "browser", Units: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotScript, "for i,key in ipairs(KEYS)") || !strings.Contains(gotScript, "INCRBY") {
		t.Fatal("acquire script is not an all-level atomic check-and-increment")
	}
	if len(gotKeys) != 4 || len(gotArgs) != 5 {
		t.Fatalf("keys=%d args=%d, want 4 keys and 5 args", len(gotKeys), len(gotArgs))
	}
	if gotArgs[0] != "1" || gotArgs[1] != "5" || gotArgs[4] != "1" {
		t.Fatalf("unexpected args %v", gotArgs)
	}
	for _, key := range gotKeys {
		if !strings.Contains(key, "{") || !strings.Contains(key, "}") {
			t.Fatalf("key lacks cluster hash tag: %q", key)
		}
	}
}

func TestRedisQuotaMapsExceededAndInvalidResults(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result any
		want   error
	}{
		{name: "exceeded", result: int64(0), want: quota.ErrQuotaExceeded},
		{name: "invalid", result: int64(-1), want: quota.ErrInvalidRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, err := New(EvaluatorFunc(func(context.Context, string, []string, ...string) (any, error) { return tc.result, nil }), map[string]quota.Limits{"browser": {}})
			if err != nil {
				t.Fatal(err)
			}
			err = q.Acquire(context.Background(), domain.ResourceRequest{TeamID: 1, TargetID: 2, OwnerID: 3, Type: "browser", Units: 1})
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v, want %v", err, tc.want)
			}
		})
	}
}

func TestRedisQuotaReleaseUsesSingleUnitsArgument(t *testing.T) {
	var gotScript string
	var gotArgs []string
	q, err := New(EvaluatorFunc(func(_ context.Context, script string, _ []string, args ...string) (any, error) {
		gotScript, gotArgs = script, args
		return "1", nil
	}), map[string]quota.Limits{"browser": {}})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Release(context.Background(), domain.ResourceRequest{TeamID: 1, TargetID: 2, OwnerID: 3, Type: "browser", Units: 2}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotScript, "DEL") || len(gotArgs) != 1 || gotArgs[0] != "2" {
		t.Fatalf("unexpected release invocation: args=%v", gotArgs)
	}
}

func TestRedisQuotaRejectsInvalidRequestBeforeEvaluator(t *testing.T) {
	called := false
	q, err := New(EvaluatorFunc(func(context.Context, string, []string, ...string) (any, error) {
		called = true
		return int64(1), nil
	}), map[string]quota.Limits{"browser": {}})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Acquire(context.Background(), domain.ResourceRequest{Type: "browser"}); !errors.Is(err, quota.ErrInvalidRequest) {
		t.Fatalf("error=%v", err)
	}
	if called {
		t.Fatal("evaluator called for invalid request")
	}
}
