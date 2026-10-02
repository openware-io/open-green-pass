package observability

import "testing"

func TestRedactFields(t *testing.T) {
	got := RedactFields(map[string]any{
		"request_id": "req-1",
		"Authorization": "Bearer secret",
		"prompt":       "private input",
		"phone_number": "13800000000",
		"status":       200,
	})
	for _, key := range []string{"Authorization", "prompt", "phone_number"} {
		if got[key] != "[REDACTED]" {
			t.Fatalf("%s was not redacted: %#v", key, got[key])
		}
	}
	if got["request_id"] != "req-1" || got["status"] != 200 {
		t.Fatalf("non-sensitive fields changed: %#v", got)
	}
}
