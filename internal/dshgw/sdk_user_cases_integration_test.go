//go:build integration

package dshgw

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestSDKRunnerUserCases(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	cells := []map[string]any{
		{"id": 2903566203719680, "method": "GET", "path": "/api/v1/admin/users", "dimension": "happy"},
		{"id": 2903566203949056, "method": "GET", "path": "/api/v1/admin/users", "dimension": "authn"},
		{"id": 2903566203981824, "method": "GET", "path": "/api/v1/admin/users", "dimension": "authz"},
		{"id": 2903566204014592, "method": "GET", "path": "/api/v1/admin/users", "dimension": "pagination"},
		{"id": 2903566204047360, "method": "GET", "path": "/api/v1/admin/users", "dimension": "empty"},
		{"id": 2903566204080128, "method": "GET", "path": "/api/v1/admin/users", "dimension": "invalid_filter"},
		{"id": 2903566204080129, "method": "GET", "path": "/api/v1/admin/users", "dimension": "sort"},
		{"id": 2903566204112896, "method": "GET", "path": "/api/v1/admin/users/{id}", "dimension": "happy"},
		{"id": 2903566204145664, "method": "GET", "path": "/api/v1/admin/users/{id}", "dimension": "not_found"},
		{"id": 2903566204178432, "method": "GET", "path": "/api/v1/admin/users/{id}", "dimension": "authz"},
		{"id": 2903566204211200, "method": "GET", "path": "/api/v1/admin/users/{id}", "dimension": "invalid_id"},
		{"id": 2903566204243968, "method": "GET", "path": "/api/v1/admin/users/{id}", "dimension": "tenant_isolation"},
		{"id": 2903566204276736, "method": "PUT", "path": "/api/v1/admin/users/{id}/status", "dimension": "happy"},
		{"id": 2903566204309504, "method": "PUT", "path": "/api/v1/admin/users/{id}/status", "dimension": "not_found"},
		{"id": 2903566204309505, "method": "PUT", "path": "/api/v1/admin/users/{id}/status", "dimension": "partial_invalid"},
		{"id": 2903566204342272, "method": "PUT", "path": "/api/v1/admin/users/{id}/status", "dimension": "stale_version"},
		{"id": 2903566204342273, "method": "PUT", "path": "/api/v1/admin/users/{id}/status", "dimension": "authz"},
		{"id": 2903566204375040, "method": "PUT", "path": "/api/v1/admin/users/{id}/status", "dimension": "state_transition"},
		{"id": 2903566204375041, "method": "DELETE", "path": "/api/v1/admin/users/{id}", "dimension": "happy"},
		{"id": 2903566204407808, "method": "DELETE", "path": "/api/v1/admin/users/{id}", "dimension": "not_found"},
		{"id": 2903566204440576, "method": "DELETE", "path": "/api/v1/admin/users/{id}", "dimension": "repeat_idempotent"},
		{"id": 2903566204473344, "method": "DELETE", "path": "/api/v1/admin/users/{id}", "dimension": "authz"},
		{"id": 2903566204506112, "method": "DELETE", "path": "/api/v1/admin/users/{id}", "dimension": "referential_conflict"},
		{"id": 2903566204538880, "method": "DELETE", "path": "/api/v1/admin/users/{id}", "dimension": "confirm_mismatch"},
	}
	cellsJSON, _ := json.Marshal(cells)
	prompt := `You are a test case generator. Return ONLY a JSON array, no markdown, no prose, no tool calls.
Create exactly 12 cases covering every cell exactly once. Each element:
{"code":"user-gen-001","title":"...","kind":"api","script":{"method":"GET","path":"...","expected_status":200},"coverage_cell_ids":[<ids>]}
Cells: ` + string(cellsJSON)
	runner := SDKRunner{
		Binary: envOr("GP_DSH_BINARY", "dsh.cmd"), Profile: "sdk", Provider: "deepseek-official",
		Model: "deepseek-flash", APIKey: os.Getenv("DEEPSEEK_API_KEY"), MaxTokens: 32768,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	result, err := runner.Run(ctx, Job{ID: "gp-user-cases-1", Prompt: prompt})
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("GP_DSH_OUTPUT"); out != "" {
		if err := os.WriteFile(out, []byte(result.Output), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if len(result.Output) < 20 {
		t.Fatalf("unexpected output: %s", result.Output)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
