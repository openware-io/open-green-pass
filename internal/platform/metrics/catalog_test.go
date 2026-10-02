package metrics

import "testing"

func TestValidateLabelsRejectsIDsAndUnknownKeys(t *testing.T) {
	for _, labels := range []map[string]string{
		{"team_id": "100"},
		{"run_id": "123"},
		{"route": "/targets/123"},
		{"method": ""},
	} {
		if err := ValidateLabels(labels); err == nil {
			t.Fatalf("labels should be rejected: %#v", labels)
		}
	}
}

func TestValidateLabelsAcceptsBoundedLabels(t *testing.T) {
	if err := ValidateLabels(map[string]string{"method": "GET", "status": "2xx", "component": "server"}); err != nil {
		t.Fatal(err)
	}
}
