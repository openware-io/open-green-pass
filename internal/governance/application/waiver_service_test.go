package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

type waiverRepoStub struct {
	created []domain.CoverageWaiver
	revoked int64
}

func (s *waiverRepoStub) CreateWaivers(_ context.Context, teamID int64, waivers []domain.CoverageWaiver) ([]domain.CoverageWaiver, error) {
	for i := range waivers {
		waivers[i].TeamID = teamID
		waivers[i].ID = int64(i + 1)
	}
	s.created = append(s.created, waivers...)
	return waivers, nil
}

func (s *waiverRepoStub) ListWaivers(context.Context, int64, int64) ([]domain.CoverageWaiver, error) {
	return s.created, nil
}

func (s *waiverRepoStub) RevokeWaiver(_ context.Context, _ int64, id int64) error {
	s.revoked = id
	return nil
}

func TestWaiverServiceRequiresReasonAndFutureExpiry(t *testing.T) {
	svc := NewWaiverService(&waiverRepoStub{})
	ctx := rls.WithUser(rls.WithTenant(context.Background(), 1001), 42)
	if _, err := svc.Create(ctx, CreateWaiverRequest{CellIDs: []int64{1}, Reason: " "}); err == nil {
		t.Fatal("blank reason must be rejected")
	}
	past := time.Now().Add(-time.Minute)
	if _, err := svc.Create(ctx, CreateWaiverRequest{CellIDs: []int64{1}, Reason: "fixture unavailable", ExpiresAt: &past}); err == nil {
		t.Fatal("past expiry must be rejected")
	}
}

func TestWaiverServiceCreatesAuditableWaivers(t *testing.T) {
	repo := &waiverRepoStub{}
	svc := NewWaiverService(repo)
	ctx := rls.WithUser(rls.WithTenant(context.Background(), 1001), 42)
	future := time.Now().Add(7 * 24 * time.Hour)
	waivers, err := svc.Create(ctx, CreateWaiverRequest{
		CellIDs: []int64{11, 12}, Reason: "media fixture unavailable", OwnerID: 42, ExpiresAt: &future,
		Evidence: json.RawMessage(`{"fixture":"media"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(waivers) != 2 || waivers[0].TeamID != 1001 || waivers[0].OwnerID != 42 {
		t.Fatalf("unexpected waivers: %+v", waivers)
	}
	if waivers[0].ApprovedBy == nil || *waivers[0].ApprovedBy != 42 {
		t.Fatalf("approved_by must be the RLS actor: %+v", waivers[0].ApprovedBy)
	}
}
