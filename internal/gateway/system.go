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
	Checks  map[string]SystemChecker
	Metrics *observability.Recorder
}

func RegisterSystem(mux *http.ServeMux, status SystemStatus) {
	if status.Metrics != nil {
		mux.Handle("GET /metrics", status.Metrics.Handler())
	}
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		checks := make(map[string]string, len(status.Checks)+1)
		if status.Ready != nil {
			checks["database"] = checkStatus(r.Context(), status.Ready)
		}
		for name, checker := range status.Checks {
			if name == "" || checker == nil {
				continue
			}
			checks[name] = checkStatus(r.Context(), checker)
		}
		for _, result := range checks {
			if result == "failed" {
				httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "checks": checks})
				return
			}
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ready", "checks": checks})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, buildinfo.Current())
	})
}

func checkStatus(ctx context.Context, checker SystemChecker) string {
	if err := checker(ctx); err != nil {
		return "failed"
	}
	return "ok"
}
