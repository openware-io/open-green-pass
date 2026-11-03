package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/ai/infra"
)

func newBatchPageService(t *testing.T) *GenerationBatchService {
	t.Helper()
	repo := infra.NewMemoryGenerationBatchRepository()
	for i := int64(1); i <= 5; i++ {
		batch, err := domain.NewGenerationBatch(i, 10, 20, 30, 40, "main", "", fmt.Sprintf("svc-page-%d", i), time.Unix(100, 0).UTC())
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Create(context.Background(), batch); err != nil {
			t.Fatal(err)
		}
	}
	return NewGenerationBatchService(repo)
}

func TestGenerationBatchServicePagination(t *testing.T) {
	svc := newBatchPageService(t)
	page, err := svc.ListByTargetPage(context.Background(), 10, 20, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Batches) != 2 || page.Batches[0].ID != 5 || page.Batches[1].ID != 4 {
		t.Fatalf("first page = %#v", page)
	}
	if page.NextCursor == nil || *page.NextCursor != 4 {
		t.Fatalf("next_cursor = %v", page.NextCursor)
	}
}

func TestGenerationBatchServiceDefaultLimitAndValidation(t *testing.T) {
	svc := newBatchPageService(t)
	// A zero limit uses the default 20; with fewer than 20 rows there is no
	// older page.
	all, err := svc.ListByTargetPage(context.Background(), 10, 20, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Batches) != 5 || all.NextCursor != nil || all.Batches[4].ID != 1 {
		t.Fatalf("default page = %#v", all)
	}
	if _, err := svc.ListByTargetPage(context.Background(), 10, 20, 0, MaxGenerationBatchLimit+1); err == nil {
		t.Fatal("over-limit page must return an error")
	}
	if _, err := svc.ListByTargetPage(context.Background(), 0, 20, 0, 2); err == nil {
		t.Fatal("missing tenant must return an error")
	}
}
