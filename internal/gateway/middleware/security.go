// Package middleware 生产安全基线：响应头与请求体上限。
package middleware

import "net/http"

// SecurityHeaders applies conservative API security headers. HSTS is
// intentionally omitted because TLS terminates at the ingress; the edge must
// own HSTS in production.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// The ingress strips the /api prefix before the API server sees the
		// path, so cache policy is applied to every API response.
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// BodyLimit caps request bodies before JSON decoding. Large evidence binaries
// must use object storage, never an API JSON body.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
