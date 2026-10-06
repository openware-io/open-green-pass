package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/platform/metrics"
	"github.com/openware-io/open-green-pass/internal/platform/observability"
	"github.com/openware-io/open-green-pass/pkg/protocol"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

// Flush preserves streaming handlers such as SSE through the metrics and
// access-log middleware wrappers.
func (w *statusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Metrics records bounded HTTP request counters and latency observations.
// It deliberately uses ServeMux's route pattern instead of the raw URL so
// resource IDs never become metric labels. If a handler does not expose a
// pattern, the stable "unmatched" value is used.
func Metrics(rec *observability.Recorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			// Go 1.22 patterns may include the method prefix. Keep the metric
			// route stable and method as its own bounded label.
			if i := strings.IndexByte(route, ' '); i >= 0 {
				route = route[i+1:]
			}
			statusClass := fmt.Sprintf("%dxx", sw.status/100)
			labels := map[string]string{"method": r.Method, "route": route, "status": statusClass}
			rec.IncrLabels(metrics.HTTPRequests, labels, 1)
			rec.ObserveLabels(metrics.HTTPDuration, labels, time.Since(start).Seconds())
		})
	}
}

// WriteHeader 记录状态码。
func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// AccessLog 用 slog 输出访问日志（request_id/method/path/status/ms）。
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			if log != nil {
				log.Info("access",
					"request_id", r.Header.Get(protocol.HeaderRequestID),
					"method", r.Method,
					"path", r.URL.Path,
					"status", sw.status,
					"ms", time.Since(start).Milliseconds(),
				)
			}
		})
	}
}
