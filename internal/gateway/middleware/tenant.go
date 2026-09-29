package middleware

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/protocol"
)

// Tenant 从 x-gp-team-id 解析 team_id 注入 context（供 RLS 租户过滤）。
// 缺省/非法时放行不注入；后续可按路由要求改为强制（401/403）。
func Tenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get(protocol.HeaderTenantID); v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				next.ServeHTTP(w, r.WithContext(rls.WithTenant(r.Context(), id)))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
