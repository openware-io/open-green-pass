package httpcase

import (
	"strings"
	"testing"
)

func TestBuildCommandUsesRegisteredOriginAndAssertsResponse(t *testing.T) {
	spec := map[string]any{
		"request": map[string]any{
			"method":  "POST",
			"path":    "/api/v1/items?mode=test",
			"headers": map[string]any{"Content-Type": "application/json"},
			"body":    map[string]any{"name": "test"},
		},
		"assertions": []any{
			map[string]any{"type": "status", "expected_status": float64(201)},
			map[string]any{"type": "body_contains", "value": "created"},
		},
	}
	command, err := BuildCommand(spec, "https://qa.example.test")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(command, " ")
	for _, want := range []string{"https://qa.example.test/api/v1/items?mode=test", "Content-Type: application/json", "HTTP status expected 201", "body assertion failed"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("command %q does not include %q", joined, want)
		}
	}
	if strings.Contains(joined, "example.attacker") || !strings.Contains(joined, "grep -F") {
		t.Fatalf("unexpected command destination or assertion: %q", joined)
	}
}

func TestBuildCommandRejectsUntrustedDestinationAndShellData(t *testing.T) {
	base := map[string]any{
		"request":    map[string]any{"method": "GET", "path": "/health", "headers": map[string]any{}, "body": nil},
		"assertions": []any{map[string]any{"type": "status", "expected_status": float64(200)}},
	}
	for _, target := range []string{"https://user:pass@example.test", "https://example.test/path", "file:///etc/passwd"} {
		if _, err := BuildCommand(base, target); err == nil {
			t.Fatalf("expected invalid registered origin %q", target)
		}
	}
	badPath := map[string]any{
		"request":    map[string]any{"method": "GET", "path": "/safe; echo pwned", "headers": map[string]any{}, "body": nil},
		"assertions": []any{map[string]any{"type": "status", "expected_status": float64(200)}},
	}
	if _, err := BuildCommand(badPath, "https://example.test"); err == nil {
		t.Fatal("expected shell punctuation in path to be rejected")
	}
}
