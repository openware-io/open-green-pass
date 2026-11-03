package infra

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/platform/metrics"
	"github.com/openware-io/open-green-pass/internal/platform/observability"
)

func gcMetricExposition(t *testing.T, recorder *observability.Recorder) string {
	t.Helper()
	rec := httptest.NewRecorder()
	recorder.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestGenerationPlanGCMetricsAreLowCardinality(t *testing.T) {
	recorder := observability.NewRecorder()
	store := NewGenerationPlanStore(nil)
	store.SetMetricsRecorder(recorder)
	store.recordGCMetrics(GenerationPlanGCSweep{
		Tenants: 4, Expired: 3, FailedTenants: 2, Duration: 150 * time.Millisecond,
	}, false)
	body := gcMetricExposition(t, recorder)
	for _, want := range []string{
		metrics.GenerationPlanGCSweeps + " 1",
		metrics.GenerationPlanGCExpired + " 3",
		metrics.GenerationPlanGCFailed + " 2",
		metrics.GenerationPlanGCDuration + "_count 1",
		metrics.GenerationPlanGCDuration + "_sum 0.15",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics exposition missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "team_id") || strings.Contains(body, "tenant") {
		t.Fatalf("GC metrics must not expose tenant labels:\n%s", body)
	}
}

func TestGenerationPlanGCMetricsCountTenantListFailure(t *testing.T) {
	recorder := observability.NewRecorder()
	store := NewGenerationPlanStore(nil)
	store.SetMetricsRecorder(recorder)
	store.recordGCMetrics(GenerationPlanGCSweep{Duration: 5 * time.Millisecond}, true)
	body := gcMetricExposition(t, recorder)
	if !strings.Contains(body, metrics.GenerationPlanGCSweeps+" 1") {
		t.Fatalf("sweep counter missing:\n%s", body)
	}
	if !strings.Contains(body, metrics.GenerationPlanGCFailed+" 1") {
		t.Fatalf("failed counter missing for tenant-list failure:\n%s", body)
	}
}
