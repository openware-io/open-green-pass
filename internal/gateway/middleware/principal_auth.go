package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/pkg/protocol"
)

// publicAuthPaths skip credential validation because they are liveness,
// metrics, or the login handshake itself. Metrics contain only the bounded
// platform labels enforced by the recorder; scrape authorization is a
// deployment/NetworkPolicy concern, like liveness/readiness, so Prometheus does
// not need an end-user token.
var publicAuthPaths = map[string]struct{}{
	"/healthz":              {},
	"/readyz":               {},
	"/version":              {},
	"/metrics":              {},
	"/auth/login":           {},
	"/auth/wechat/status":   {},
	"/auth/wechat/start":    {},
	"/auth/wechat/callback": {},
}

// PrincipalAuth validates a bearer credential through an injected provider.
// Header-based identity remains available only in the explicit dev fallback.
func PrincipalAuth(provider iam.AuthenticationProvider, allowHeaderFallback bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			servePrincipalAuth(w, r, next, provider, allowHeaderFallback)
		})
	}
}

func servePrincipalAuth(w http.ResponseWriter, r *http.Request, next http.Handler, provider iam.AuthenticationProvider, allowHeaderFallback bool) {
	if _, public := publicAuthPaths[r.URL.Path]; public {
		next.ServeHTTP(w, r)
		return
	}
	credential := requestCredential(r)
	if provider != nil && credential != "" {
		serveProviderCredential(w, r, next, provider, credential)
		return
	}
	if !allowHeaderFallback {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// Header identity is deliberately restricted to the explicit dev/test
	// fallback. It is converted to the same Principal shape as a verified
	// provider result, so RBAC cannot be silently bypassed in local e2e tests.
	// Production composition always passes allowHeaderFallback=false.
	if serveDevHeaderIdentity(w, r, next) {
		return
	}
	next.ServeHTTP(w, r)
}

func requestCredential(r *http.Request) string {
	credential := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if credential != "" {
		return credential
	}
	if cookie, err := r.Cookie("gp_session"); err == nil {
		return cookie.Value
	}
	return ""
}

func serveProviderCredential(w http.ResponseWriter, r *http.Request, next http.Handler, provider iam.AuthenticationProvider, credential string) {
	result := provider.Authenticate(r.Context(), credential)
	if err := result.Validate(time.Now().UTC()); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	principal := *result.Principal
	next.ServeHTTP(w, r.WithContext(iam.WithPrincipal(r.Context(), principal)))
}

// serveDevHeaderIdentity handles the dev-header fallback. It returns false when
// no header identity was supplied and the request must continue unauthenticated.
func serveDevHeaderIdentity(w http.ResponseWriter, r *http.Request, next http.Handler) bool {
	teamID := strings.TrimSpace(r.Header.Get(protocol.HeaderTenantID))
	userID := strings.TrimSpace(r.Header.Get(protocol.HeaderUserID))
	if teamID == "" && userID == "" {
		return false
	}
	if _, err := strconv.ParseInt(teamID, 10, 64); err != nil || teamID == "" || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return true
	}
	principal := iam.Principal{Subject: userID, TenantID: teamID, Issuer: "gp-dev-header"}
	next.ServeHTTP(w, r.WithContext(iam.WithPrincipal(r.Context(), principal)))
	return true
}
