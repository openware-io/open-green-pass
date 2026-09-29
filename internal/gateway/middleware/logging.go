package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/openware-io/open-green-pass/pkg/protocol"
)

type statusWriter struct {
	http.ResponseWriter
	status int
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
