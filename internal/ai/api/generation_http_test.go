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

func seedBatchListRepository(t *testing.T) *infra.MemoryGenerationBatchRepository {
	t.Helper()
	repo := infra.NewMemoryGenerationBatchRepository()
	for i := int64(1); i <= 53; i++ {
		batch, err := domain.NewGenerationBatch(i, 10, 20, 30, 40, "main", "", fmt.Sprint(i), time.Unix(100, 0).UTC())
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			// Creation time takes precedence over ID.
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

func getBatchList(t *testing.T, mux *http.ServeMux, query string, teamID int64) []batchListRow {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/generation/batches?"+query, nil)
	req = req.WithContext(rls.WithTenant(req.Context(), teamID))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var result []batchListRow
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestGenerationBatchListScopesOrdersAndLimitsResults(t *testing.T) {
	repo := seedBatchListRepository(t)
	mux := http.NewServeMux()
	RegisterGeneration(mux, application.NewGenerationBatchService(repo), nil, nil)
	result := getBatchList(t, mux, "target_id=20", 10)
	if len(result) != 50 || result[0].ID != 1 || result[1].ID != 51 || result[49].ID != 3 || result[0].State != domain.GenerationApproved {
		t.Fatalf("unexpected default ordered results: %#v", result)
	}
	for _, batch := range result {
		if batch.TeamID != 10 || batch.TargetID != 20 {
			t.Fatalf("cross-tenant/project batch: %#v", batch)
		}
	}
}

func TestGenerationBatchListLimitsAndScopes(t *testing.T) {
	repo := seedBatchListRepository(t)
	mux := http.NewServeMux()
	RegisterGeneration(mux, application.NewGenerationBatchService(repo), nil, nil)
	if result := getBatchList(t, mux, "target_id=20&limit=2", 10); len(result) != 2 || result[0].ID != 1 || result[1].ID != 51 {
		t.Fatalf("unexpected bounded results: %#v", result)
	}
	if result := getBatchList(t, mux, "target_id=20", 11); len(result) != 1 || result[0].ID != 53 {
		t.Fatalf("unexpected other tenant results: %#v", result)
	}
	if result := getBatchList(t, mux, "target_id=21", 10); len(result) != 1 || result[0].ID != 52 {
		t.Fatalf("unexpected other project results: %#v", result)
	}
}

func TestGenerationBatchListEmptyProjectReturnsArray(t *testing.T) {
	repo := seedBatchListRepository(t)
	mux := http.NewServeMux()
	RegisterGeneration(mux, application.NewGenerationBatchService(repo), nil, nil)
	if result := getBatchList(t, mux, "target_id=999", 10); result == nil || len(result) != 0 {
		t.Fatalf("empty project should return a JSON array: %#v", result)
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
