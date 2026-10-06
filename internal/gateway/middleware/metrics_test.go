package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openware-io/open-green-pass/internal/platform/observability"
)

func TestMetricsUsesRoutePatternAndStatusClass(t *testing.T) {
	r := observability.NewRecorder()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /targets/{targetId}", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", http.StatusNotFound) })
	h := Metrics(r)(mux)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/targets/365598005031604224", nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status=%d", resp.Code)
	}
	metricsResp := httptest.NewRecorder()
	r.Handler().ServeHTTP(metricsResp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := metricsResp.Body.String()
	if !strings.Contains(body, `route="/targets/{targetId}"`) || !strings.Contains(body, `status="4xx"`) || strings.Contains(body, "365598005031604224") {
		t.Fatalf("unexpected metrics: %s", body)
	}
}
