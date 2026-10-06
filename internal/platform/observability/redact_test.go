package observability

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactFields(t *testing.T) {
	got := RedactFields(map[string]any{
		"request_id":    "req-1",
		"Authorization": "Bearer secret",
		"prompt":        "private input",
		"phone_number":  "13800000000",
		"status":        200,
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

func TestStructuredLoggerRedactsSensitiveAttributes(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{ReplaceAttr: redactAttr}))
	logger.Info("request", "request_id", "req-1", "api_token", "raw-token", "Authorization", "Bearer raw")
	logged := output.String()
	if strings.Contains(logged, "raw-token") || strings.Contains(logged, "Bearer raw") {
		t.Fatalf("sensitive value leaked: %s", logged)
	}
	if !strings.Contains(logged, "req-1") || strings.Count(logged, "[REDACTED]") != 2 {
		t.Fatalf("unexpected redacted output: %s", logged)
	}
}
