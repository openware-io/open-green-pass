package gateway

import (
	"context"
	"net/http"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/platform/buildinfo"
	"github.com/openware-io/open-green-pass/internal/platform/observability"
)

// SystemChecker is a dependency health probe. It must not expose connection
// strings or credentials in its returned error.
type SystemChecker func(context.Context) error

type SystemStatus struct {
	Ready   SystemChecker
	Metrics *observability.Recorder
}

func RegisterSystem(mux *http.ServeMux, status SystemStatus) {
	if status.Metrics != nil {
		mux.Handle("GET /metrics", status.Metrics.Handler())
	}
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if status.Ready == nil {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ready", "checks": map[string]string{}})
			return
		}
		if err := status.Ready(r.Context()); err != nil {
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "checks": map[string]string{"database": "failed"}})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ready", "checks": map[string]string{"database": "ok"}})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, buildinfo.Current())
	})
}
