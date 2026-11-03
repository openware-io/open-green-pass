package application

import (
	"context"
	"strings"
	"testing"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type captureCostRepository struct {
	items []*domain.CostLineItem

	batchCalls  int
	batchTeamID int64
	batchIDs    []int64
	batchTenant int64
	batchResult map[int64]float64
	batchErr    error
}

func (r *captureCostRepository) Insert(_ context.Context, item *domain.CostLineItem) (bool, error) {
	r.items = append(r.items, item)
	return true, nil
}

func (r *captureCostRepository) Overview(context.Context, int64) (*domain.CostOverview, error) {
	return &domain.CostOverview{}, nil
}

func (r *captureCostRepository) Items(context.Context, int64, string, string, int) ([]*domain.CostLineItem, error) {
	return nil, nil
}

func (r *captureCostRepository) CompareHistory(context.Context, int64, int64) ([]*domain.CostLineItem, error) {
	return nil, nil
}

func (r *captureCostRepository) BatchCosts(ctx context.Context, teamID int64, batchIDs []int64) (map[int64]float64, error) {
	r.batchCalls++
	r.batchTeamID = teamID
	r.batchIDs = append([]int64(nil), batchIDs...)
	if tenant, ok := rls.TenantFrom(ctx); ok {
		r.batchTenant = tenant
	}
	if r.batchErr != nil {
		return nil, r.batchErr
	}
	return r.batchResult, nil
}

type noopAuditRepository struct{}

func (noopAuditRepository) Append(context.Context, *domain.AuditEvent) error { return nil }

func (noopAuditRepository) LatestHash(context.Context, int64) (string, error) {
	return strings.Repeat("0", 64), nil
}

func newCostServiceForTest(t *testing.T) (*CostService, *captureCostRepository) {
	t.Helper()
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo := &captureCostRepository{}
	return NewCostService(repo, NewAuditService(noopAuditRepository{}, gen), gen), repo
}

func TestCostRecordPersistsGenerationBatchID(t *testing.T) {
	svc, repo := newCostServiceForTest(t)
	ctx := rls.WithTenant(context.Background(), 10)
	inserted, err := svc.Record(ctx, RecordRequest{
		RequestID: "gen-40-1", BizPoint: "case_generate", TargetID: 20, RunID: 30, BatchID: 40,
		TokensIn: 1000, TokensOut: 1000, InputUnitPrice: 0.01, OutputUnitPrice: 0.02,
	})
	if err != nil || !inserted {
		t.Fatalf("record = %v, %v", inserted, err)
	}
	if len(repo.items) != 1 || repo.items[0].BatchID != 40 {
		t.Fatalf("generation cost batch_id = %+v", repo.items)
	}
	// 1000/1000 tokens at 0.01/0.02 per 1K = 0.01 + 0.02.
	if repo.items[0].Amount < 0.029 || repo.items[0].Amount > 0.031 {
		t.Fatalf("amount = %v", repo.items[0].Amount)
	}

	// Non-generation charges keep batch_id zero so the column stays nullable for
	// execution/gate rows.
	if _, err := svc.Record(ctx, RecordRequest{RequestID: "exec-1", BizPoint: "case_execute", TargetID: 20}); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 2 || repo.items[1].BatchID != 0 {
		t.Fatalf("non-generation batch_id = %+v", repo.items)
	}
}

func TestCostServiceBatchCostsInjectsTenantScope(t *testing.T) {
	svc, repo := newCostServiceForTest(t)
	repo.batchResult = map[int64]float64{11: 0.05}
	result, err := svc.BatchCosts(context.Background(), 77, []int64{11, 12})
	if err != nil {
		t.Fatal(err)
	}
	if result[11] != 0.05 {
		t.Fatalf("batch cost = %+v", result)
	}
	if repo.batchCalls != 1 || repo.batchTeamID != 77 || repo.batchTenant != 77 {
		t.Fatalf("repo call = calls:%d team:%d tenant:%d", repo.batchCalls, repo.batchTeamID, repo.batchTenant)
	}
	if len(repo.batchIDs) != 2 || repo.batchIDs[0] != 11 || repo.batchIDs[1] != 12 {
		t.Fatalf("batch ids = %v", repo.batchIDs)
	}
	// Empty input is a valid empty result and must not hit the repository.
	if empty, err := svc.BatchCosts(context.Background(), 77, nil); err != nil || len(empty) != 0 {
		t.Fatalf("empty batch costs = %+v, %v", empty, err)
	}
	if repo.batchCalls != 1 {
		t.Fatalf("empty input called repository %d times", repo.batchCalls)
	}
	if _, err := svc.BatchCosts(context.Background(), 0, []int64{11}); err == nil {
		t.Fatal("invalid team must be rejected")
	}
}
