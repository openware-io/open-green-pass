package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadyzReportsDependencyFailure(t *testing.T) {
	h := NewRouter(nil, func(mux *http.ServeMux) {
		RegisterSystem(mux, SystemStatus{Ready: func(context.Context) error { return errors.New("database unavailable") }})
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "database unavailable") || !strings.Contains(rec.Body.String(), `"status":"not_ready"`) || !strings.Contains(rec.Body.String(), `"database":"failed"`) {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestSystemVersionAndReadyz(t *testing.T) {
	h := NewRouter(nil, func(mux *http.ServeMux) {
		RegisterSystem(mux, SystemStatus{Ready: func(context.Context) error { return nil }})
	})
	for _, path := range []string{"/readyz", "/version"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d", path, rec.Code)
		}
	}
}
