package middleware

import (
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/protocol"
)

// Auth 从 x-gp-user-id 注入操作人（审计 user_id）。
// P0 占位：仅透传头解析；后续接 JWT/微信 OAuth 后改为真实凭证校验。
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get(protocol.HeaderUserID); v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				next.ServeHTTP(w, r.WithContext(rls.WithUser(r.Context(), id)))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
