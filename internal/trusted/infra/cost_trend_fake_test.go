package infra

import (
	"context"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/trusted/domain"
)

func TestFakeCostTrendStoreIsIdempotentAndTenantIsolated(t *testing.T) {
	store := NewFakeCostTrendStore()
	base := time.Date(2026, 10, 6, 10, 15, 0, 0, time.UTC)
	facts := []domain.CostFact{
		{SourceCostID: 1, TeamID: 10, Category: domain.CostCategory("generate"), Model: "m1", Amount: 1.25, TokensIn: 10, TokensOut: 20, OccurredAt: base},
		{SourceCostID: 2, TeamID: 10, Category: domain.CostCategory("generate"), Model: "m1", Amount: 2.75, TokensIn: 30, TokensOut: 40, OccurredAt: base.Add(20 * time.Minute)},
		{SourceCostID: 3, TeamID: 20, Category: domain.CostCategory("generate"), Model: "m1", Amount: 99, TokensIn: 99, TokensOut: 99, OccurredAt: base},
	}
	for _, fact := range facts {
		inserted, err := store.UpsertFact(context.Background(), fact)
		if err != nil || !inserted {
			t.Fatalf("upsert fact: inserted=%v err=%v", inserted, err)
		}
	}
	inserted, err := store.UpsertFact(context.Background(), facts[0])
	if err != nil || inserted {
		t.Fatalf("duplicate event must be ignored: inserted=%v err=%v", inserted, err)
	}

	points, err := store.Trend(context.Background(), domain.CostTrendFilter{
		TeamID: 10, From: base.Add(-time.Hour), To: base.Add(time.Hour), Bucket: domain.TrendBucketHour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Amount != 4 || points[0].TokensIn != 40 || points[0].TokensOut != 60 || points[0].CallCount != 2 {
		t.Fatalf("unexpected team projection: %#v", points)
	}
	if points[0].Amount == 103 {
		t.Fatal("cross-team amount leaked into projection")
	}
}

func TestFakeCostTrendStoreCanRebuildTeamProjection(t *testing.T) {
	store := NewFakeCostTrendStore()
	fact := domain.CostFact{SourceCostID: 8, TeamID: 10, Category: domain.CostCategory("execute"), Amount: 3, OccurredAt: time.Now().UTC()}
	_, _ = store.UpsertFact(context.Background(), fact)
	if err := store.DeleteTeamFacts(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	inserted, err := store.UpsertFact(context.Background(), fact)
	if err != nil || !inserted {
		t.Fatalf("rebuild must accept source after reset: inserted=%v err=%v", inserted, err)
	}
}
