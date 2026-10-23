package infra

import "testing"

func TestNormalizeLegacyFlatCase(t *testing.T) {
	root := map[string]any{"path": "/legacy", "method": "GET", "expected_status": 200}
	request := normalizeLegacyRequest(root)
	if request["path"] != "/legacy" || request["method"] != "GET" {
		t.Fatalf("request=%+v", request)
	}
	assertions, ok := root["assertions"].([]any)
	if !ok || len(assertions) != 1 {
		t.Fatalf("assertions=%#v", root["assertions"])
	}
	assertion := assertions[0].(map[string]any)
	if assertion["type"] != "status" || assertion["expected_status"] != 200 {
		t.Fatalf("assertion=%#v", assertion)
	}
}
