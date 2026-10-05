// Package gateway 组装 HTTP 接入层：中间件链 + 路由。
// 业务路由由各 domain 的 api 层注册（P0 仅健康检查）。
package gateway

import (
	"log/slog"
	"net/http"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/gateway/middleware"
	"github.com/openware-io/open-green-pass/internal/iam"
)

// NewRouter 组装中间件链与基础路由。
// 中间件顺序（由外到内）：AccessLog -> RequestID -> Tenant -> Auth -> 业务路由。
// registrars 由各业务域 api 层提供（cmd/server 组装依赖后注入）。
func NewRouter(log *slog.Logger, registrars ...func(*http.ServeMux)) http.Handler {
	return NewRouterWithAuth(log, nil, true, registrars...)
}

// NewRouterWithAuth allows cmd-level wiring of a verified identity provider.
// The legacy constructor remains a dev/test compatibility path.
func NewRouterWithAuth(log *slog.Logger, provider iam.AuthenticationProvider, allowHeaderFallback bool, registrars ...func(*http.ServeMux)) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	for _, reg := range registrars {
		reg(mux)
	}

	var h http.Handler = mux
	// PrincipalAuth owns both verified provider credentials and the explicitly
	// enabled development-header fallback. Keeping this single path ensures
	// downstream RBAC never observes a header-only request without a Principal.
	h = middleware.PrincipalAuth(provider, allowHeaderFallback)(h)
	h = middleware.Tenant(h)
	h = middleware.RequestID(h)
	h = middleware.AccessLog(log)(h)
	return h
}
