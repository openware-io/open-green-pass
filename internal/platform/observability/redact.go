package observability

import "strings"

// RedactFields removes credentials and model content from structured log fields.
// Callers should still avoid passing sensitive values to the logger when possible.
func RedactFields(fields map[string]any) map[string]any {
	out := make(map[string]any, len(fields))
	for key, value := range fields {
		if sensitiveLogKey(key) {
			out[key] = "[REDACTED]"
			continue
		}
		out[key] = value
	}
	return out
}

func sensitiveLogKey(key string) bool {
	k := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), ".", "_"))
	for _, token := range []string{"authorization", "token", "secret", "password", "api_key", "apikey", "prompt", "response", "phone", "mobile"} {
		if strings.Contains(k, token) {
			return true
		}
	}
	return false
}
