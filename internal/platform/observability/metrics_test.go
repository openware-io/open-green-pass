package observability

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecorderRejectsUnboundedLabelsAndRendersSnapshot(t *testing.T) {
	r := NewRecorder()
	r.IncrLabels("gp_http_requests_total", map[string]string{"route": "/targets/abc123", "method": "GET"}, 1)
	r.IncrLabels("gp_http_requests_total", map[string]string{"route": "/targets/{targetId}", "method": "GET", "status": "2xx"}, 2)
	r.ObserveLabels("gp_http_request_duration_seconds", map[string]string{"route": "/targets/{targetId}", "method": "GET", "status": "2xx"}, .25)
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, _ := io.ReadAll(w.Result().Body)
	s := string(body)
	if strings.Contains(s, "abc123") || !strings.Contains(s, `gp_http_requests_total{method="GET",route="/targets/{targetId}",status="2xx"} 2`) {
		t.Fatalf("unexpected exposition: %s", s)
	}
	if !strings.Contains(s, `gp_http_request_duration_seconds_count{method="GET",route="/targets/{targetId}",status="2xx"} 1`) {
		t.Fatalf("histogram count missing: %s", s)
	}
}
