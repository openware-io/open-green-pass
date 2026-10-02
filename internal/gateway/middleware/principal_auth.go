package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/iam"
)

// PrincipalAuth validates a bearer credential through an injected provider.
// Header-based identity remains available only in the explicit dev fallback.
func PrincipalAuth(provider iam.AuthenticationProvider, allowHeaderFallback bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			credential := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
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
			next.ServeHTTP(w, r)
		})
	}
}
