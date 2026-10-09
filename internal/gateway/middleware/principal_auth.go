package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/pkg/protocol"
)

// PrincipalAuth validates a bearer credential through an injected provider.
// Header-based identity remains available only in the explicit dev fallback.
func PrincipalAuth(provider iam.AuthenticationProvider, allowHeaderFallback bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Metrics contain only the bounded platform labels enforced by the
			// recorder. Scrape authorization is a deployment/NetworkPolicy concern,
			// like liveness/readiness, so Prometheus does not need an end-user token.
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/version" || r.URL.Path == "/metrics" || r.URL.Path == "/auth/login" || r.URL.Path == "/auth/wechat/status" || r.URL.Path == "/auth/wechat/start" || r.URL.Path == "/auth/wechat/callback" {
				next.ServeHTTP(w, r)
				return
			}
			credential := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
			if credential == "" {
				if cookie, err := r.Cookie("gp_session"); err == nil { credential = cookie.Value }
			}
			if provider != nil && credential != "" {
				result := provider.Authenticate(r.Context(), credential)
				if err := result.Validate(time.Now().UTC()); err != nil {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				principal := *result.Principal
				next.ServeHTTP(w, r.WithContext(iam.WithPrincipal(r.Context(), principal)))
				return
			}
			if !allowHeaderFallback {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			// Header identity is deliberately restricted to the explicit dev/test
			// fallback. It is converted to the same Principal shape as a verified
			// provider result, so RBAC cannot be silently bypassed in local e2e
			// tests. Production composition always passes allowHeaderFallback=false.
			teamID := strings.TrimSpace(r.Header.Get(protocol.HeaderTenantID))
			userID := strings.TrimSpace(r.Header.Get(protocol.HeaderUserID))
			if teamID != "" || userID != "" {
				if _, err := strconv.ParseInt(teamID, 10, 64); err != nil || teamID == "" || userID == "" {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				principal := iam.Principal{Subject: userID, TenantID: teamID, Issuer: "gp-dev-header"}
				next.ServeHTTP(w, r.WithContext(iam.WithPrincipal(r.Context(), principal)))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
