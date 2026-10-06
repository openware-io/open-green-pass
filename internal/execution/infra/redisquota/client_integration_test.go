package redisquota

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/execution/infra/quota"
)

func TestRedisQuotaAcrossIndependentClients(t *testing.T) {
	address := os.Getenv("GP_REDIS_TEST_ADDR")
	if address == "" {
		t.Skip("GP_REDIS_TEST_ADDR is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	options := ClientOptions{Address: address, Password: os.Getenv("GP_REDIS_TEST_PASSWORD")}
	firstClient, err := NewClient(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer firstClient.Close()
	secondClient, err := NewClient(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer secondClient.Close()

	resourceType := "integration-" + time.Now().UTC().Format("20060102150405.000000000")
	limits := map[string]quota.Limits{resourceType: {Global: 1, Team: 1, Target: 1, Owner: 1}}
	firstQuota, err := New(firstClient, limits)
	if err != nil {
		t.Fatal(err)
	}
	secondQuota, err := New(secondClient, limits)
	if err != nil {
		t.Fatal(err)
	}
	request := domain.ResourceRequest{TeamID: 9001, TargetID: 9002, OwnerID: 9003, Type: resourceType, Units: 1}
	defer firstQuota.Release(context.Background(), request)

	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, candidate := range []*RedisQuota{firstQuota, secondQuota} {
		workers.Add(1)
		go func(candidate *RedisQuota) {
			defer workers.Done()
			<-start
			results <- candidate.Acquire(ctx, request)
		}(candidate)
	}
	close(start)
	workers.Wait()
	close(results)

	var acquired, rejected int
	for result := range results {
		switch {
		case result == nil:
			acquired++
		case errors.Is(result, quota.ErrQuotaExceeded):
			rejected++
		default:
			t.Fatalf("unexpected acquire error: %v", result)
		}
	}
	if acquired != 1 || rejected != 1 {
		t.Fatalf("acquired=%d rejected=%d, want one of each", acquired, rejected)
	}
}
