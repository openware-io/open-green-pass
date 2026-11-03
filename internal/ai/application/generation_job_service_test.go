package application

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// missingGenerationJobRepo models the repository's "no rows" contract so the
// service-level 404 mapping is covered without a database.
type missingGenerationJobRepo struct{}

func (missingGenerationJobRepo) Create(context.Context, *domain.GenerationJob, *domain.GenerationJobEvent) (bool, error) {
	return false, nil
}

func (missingGenerationJobRepo) Find(context.Context, int64, int64) (*domain.GenerationJob, error) {
	return nil, pgx.ErrNoRows
}

func (missingGenerationJobRepo) FindByKey(context.Context, int64, string) (*domain.GenerationJob, error) {
	return nil, pgx.ErrNoRows
}

func (missingGenerationJobRepo) Update(context.Context, *domain.GenerationJob, *domain.GenerationJobEvent) error {
	return nil
}

func (missingGenerationJobRepo) Events(context.Context, int64, int64) ([]domain.GenerationJobEvent, error) {
	return nil, nil
}

func TestGenerationJobGetCancelMapMissingJobToNotFound(t *testing.T) {
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewGenerationJobService(missingGenerationJobRepo{}, nil, gen)
	ctx := rls.WithTenant(context.Background(), 10)
	if _, err := svc.Get(ctx, 999); gperr.KindOf(err) != gperr.KindNotFound {
		t.Fatalf("Get missing job error kind = %v, want not_found (err=%v)", gperr.KindOf(err), err)
	}
	if err := svc.Cancel(ctx, 999); gperr.KindOf(err) != gperr.KindNotFound {
		t.Fatalf("Cancel missing job error kind = %v, want not_found (err=%v)", gperr.KindOf(err), err)
	}
}
