package application

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type bundleRepo struct{ reports []*domain.Report }

func (r *bundleRepo) Save(context.Context, *domain.Report) error { return nil }
func (r *bundleRepo) Find(context.Context, int64, int64) (*domain.Report, error) {
	return nil, domain.ErrReportNotFound
}
func (r *bundleRepo) FindMany(context.Context, int64, []int64) ([]*domain.Report, error) {
	return r.reports, nil
}

func TestBuildBundleRendersSummary(t *testing.T) {
	g, _ := id.New(1, nil)
	repo := &bundleRepo{reports: []*domain.Report{{ID: 1, Scenario: "api", Version: "v1", Status: "pass"}, {ID: 2, Scenario: "web", Version: "v2", Status: "fail"}}}
	s, err := NewReportService(nil, nil, nil, repo, g)
	if err != nil {
		t.Fatal(err)
	}
	ctx := rls.WithTenant(context.Background(), 100)
	b, err := s.BuildBundle(ctx, []int64{1, 2}, "html")
	if err != nil {
		t.Fatal(err)
	}
	if b.Total != 2 || b.Pass != 1 || b.Fail != 1 || b.HTML == "" || b.Markdown == "" {
		t.Fatalf("bundle=%+v", b)
	}
}
