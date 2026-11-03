package infra

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

func seedMemoryBatchPage(t *testing.T, repo *MemoryGenerationBatchRepository, count int) {
	t.Helper()
	for i := int64(1); i <= int64(count); i++ {
		batch, err := domain.NewGenerationBatch(i, 10, 20, 30, 40, "main", "", fmt.Sprintf("page-%d", i), time.Unix(100, 0).UTC())
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Create(context.Background(), batch); err != nil {
			t.Fatal(err)
		}
	}
}

func memoryBatchPage(t *testing.T, repo *MemoryGenerationBatchRepository, targetID, beforeID int64, limit int) *domain.GenerationBatchPage {
	t.Helper()
	page, err := repo.ListByTargetPage(context.Background(), 10, targetID, beforeID, limit)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func assertMemoryBatchPage(t *testing.T, page *domain.GenerationBatchPage, wantIDs []int64, wantNext *int64) {
	t.Helper()
	if len(page.Batches) != len(wantIDs) {
		t.Fatalf("batch count = %d, want %d (%#v)", len(page.Batches), len(wantIDs), page)
	}
	for i, want := range wantIDs {
		if page.Batches[i].ID != want {
			t.Fatalf("batch[%d] = %d, want %d", i, page.Batches[i].ID, want)
		}
	}
	if wantNext == nil {
		if page.NextCursor != nil {
			t.Fatalf("next_cursor = %v, want nil", *page.NextCursor)
		}
		return
	}
	if page.NextCursor == nil || *page.NextCursor != *wantNext {
		t.Fatalf("next_cursor = %v, want %d", page.NextCursor, *wantNext)
	}
}

func int64Pointer(value int64) *int64 { return &value }

func TestMemoryGenerationBatchRepositoryPagesByIDDesc(t *testing.T) {
	repo := NewMemoryGenerationBatchRepository()
	seedMemoryBatchPage(t, repo, 5)
	other, err := domain.NewGenerationBatch(6, 10, 21, 30, 40, "main", "", "page-other", time.Unix(100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(context.Background(), other); err != nil {
		t.Fatal(err)
	}

	first := memoryBatchPage(t, repo, 20, 0, 2)
	assertMemoryBatchPage(t, first, []int64{5, 4}, int64Pointer(4))
	second := memoryBatchPage(t, repo, 20, *first.NextCursor, 2)
	assertMemoryBatchPage(t, second, []int64{3, 2}, int64Pointer(2))
	last := memoryBatchPage(t, repo, 20, *second.NextCursor, 2)
	assertMemoryBatchPage(t, last, []int64{1}, nil)
	// Target isolation: target 21 only has batch 6.
	otherPage := memoryBatchPage(t, repo, 21, 0, 2)
	assertMemoryBatchPage(t, otherPage, []int64{6}, nil)
}
