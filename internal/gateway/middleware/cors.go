package middleware

import (
	"net/http"
	"strings"
)

// CORS applies an explicit origin allowlist. It intentionally never returns
// wildcard origins because tenant and user headers are part of the API contract.
func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	origins := parseOrigins(allowedOrigins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && origins[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-GP-Team-ID, X-GP-User-ID, X-GP-Request-ID")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			}
			if r.Method == http.MethodOptions {
				if origin == "" || !origins[origin] {
					http.Error(w, "cors origin not allowed", http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseOrigins(raw string) map[string]bool {
	origins := make(map[string]bool)
	for _, origin := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			origins[trimmed] = true
		}
	}
	return origins
}
