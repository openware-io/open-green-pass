package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/ai/infra"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

type batchListRow struct {
	ID       int64                       `json:"id"`
	TeamID   int64                       `json:"team_id"`
	TargetID int64                       `json:"target_id"`
	State    domain.GenerationBatchState `json:"state"`
}

type batchListPage struct {
	Batches    []batchListRow `json:"batches"`
	NextCursor *string        `json:"next_cursor"`
}

func seedBatchListRepository(t *testing.T) *infra.MemoryGenerationBatchRepository {
	t.Helper()
	repo := infra.NewMemoryGenerationBatchRepository()
	for i := int64(1); i <= 53; i++ {
		batch, err := domain.NewGenerationBatch(i, 10, 20, 30, 40, "main", "", fmt.Sprint(i), time.Unix(100, 0).UTC())
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			batch.CreatedAt = batch.CreatedAt.Add(time.Second)
			batch.State = domain.GenerationApproved
		}
		if i == 52 {
			batch.TargetID = 21
		}
		if i == 53 {
			batch.TeamID = 11
		}
		if err := repo.Create(context.Background(), batch); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func getBatchList(t *testing.T, mux *http.ServeMux, query string, teamID int64) batchListPage {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/generation/batches?"+query, nil)
	req = req.WithContext(rls.WithTenant(req.Context(), teamID))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var page batchListPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func TestGenerationBatchListScopesOrdersAndLimitsResults(t *testing.T) {
	repo := seedBatchListRepository(t)
	mux := http.NewServeMux()
	RegisterGeneration(mux, application.NewGenerationBatchService(repo), nil, nil)
	page := getBatchList(t, mux, "target_id=20", 10)
	if len(page.Batches) != 20 || page.Batches[0].ID != 51 || page.Batches[19].ID != 32 {
		t.Fatalf("unexpected default page: %#v", page)
	}
	if page.NextCursor == nil || *page.NextCursor != "32" {
		t.Fatalf("default next_cursor = %v, want 32", page.NextCursor)
	}
	for _, batch := range page.Batches {
		if batch.TeamID != 10 || batch.TargetID != 20 {
			t.Fatalf("cross-tenant/project batch: %#v", batch)
		}
	}
}

func TestGenerationBatchListCursorPagination(t *testing.T) {
	repo := seedBatchListRepository(t)
	mux := http.NewServeMux()
	RegisterGeneration(mux, application.NewGenerationBatchService(repo), nil, nil)
	first := getBatchList(t, mux, "target_id=20&limit=2", 10)
	if len(first.Batches) != 2 || first.Batches[0].ID != 51 || first.Batches[1].ID != 50 {
		t.Fatalf("first page = %#v", first)
	}
	if first.NextCursor == nil || *first.NextCursor != "50" {
		t.Fatalf("first next_cursor = %v", first.NextCursor)
	}
	second := getBatchList(t, mux, "target_id=20&limit=2&cursor="+*first.NextCursor, 10)
	if len(second.Batches) != 2 || second.Batches[0].ID != 49 || second.Batches[1].ID != 48 {
		t.Fatalf("second page = %#v", second)
	}
	if second.NextCursor == nil || *second.NextCursor != "48" {
		t.Fatalf("second next_cursor = %v", second.NextCursor)
	}
	// A project with a single batch has no older page: next_cursor is null.
	only := getBatchList(t, mux, "target_id=21&limit=2", 10)
	if len(only.Batches) != 1 || only.Batches[0].ID != 52 || only.NextCursor != nil {
		t.Fatalf("single-page result = %#v", only)
	}
}

func TestGenerationBatchListEmptyProjectReturnsObject(t *testing.T) {
	repo := seedBatchListRepository(t)
	mux := http.NewServeMux()
	RegisterGeneration(mux, application.NewGenerationBatchService(repo), nil, nil)
	page := getBatchList(t, mux, "target_id=999", 10)
	if page.Batches == nil || len(page.Batches) != 0 || page.NextCursor != nil {
		t.Fatalf("empty project page = %#v", page)
	}
}

func TestGenerationBatchListRejectsInvalidScopeAndLimit(t *testing.T) {
	mux := http.NewServeMux()
	// A nil service makes any accidental attempt to query an invalid request fail.
	RegisterGeneration(mux, nil, nil, nil)
	for _, tc := range []struct {
		query string
		team  int64
	}{
		{"target_id=1", 0},
		{"target_id=1", -1},
		{"", 10},
		{"target_id=0", 10},
		{"target_id=-1", 10},
		{"target_id=abc", 10},
		{"target_id=9223372036854775808", 10},
		{"target_id=1&limit=0", 10},
		{"target_id=1&limit=-1", 10},
		{"target_id=1&limit=101", 10},
		{"target_id=1&limit=abc", 10},
		{"target_id=1&limit=", 10},
		{"target_id=1&cursor=abc", 10},
		{"target_id=1&cursor=0", 10},
		{"target_id=1&cursor=-5", 10},
	} {
		t.Run(fmt.Sprintf("%d/%s", tc.team, tc.query), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/generation/batches?"+tc.query, nil)
			if tc.team != 0 {
				req = req.WithContext(rls.WithTenant(req.Context(), tc.team))
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}
