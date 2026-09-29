// Package gateway 组装 HTTP 接入层：中间件链 + 路由。
// 业务路由由各 domain 的 api 层注册（P0 仅健康检查）。
package gateway

import (
	"log/slog"
	"net/http"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/gateway/middleware"
)

// NewRouter 组装中间件链与基础路由。
// 中间件顺序（由外到内）：AccessLog -> RequestID -> Tenant -> Auth -> 业务路由。
func NewRouter(log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	var h http.Handler = mux
	h = middleware.Auth(h)
	h = middleware.Tenant(h)
	h = middleware.RequestID(h)
	h = middleware.AccessLog(log)(h)
	return h
}
