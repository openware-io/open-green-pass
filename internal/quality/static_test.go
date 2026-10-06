package quality

import "testing"

func TestAnalyzeDetectsKnownWeakeningWithoutMutationScore(t *testing.T) {
	r := Analyze(map[string]string{"sample_test.go": "func TestX(t *testing.T) { t.Skip(\"later\") }\n// TODO: strengthen"}, RuleVersion{"static", "2026-10-06"}, nil)
	if len(r.Findings) != 2 {
		t.Fatalf("findings=%d, want 2", len(r.Findings))
	}
	if r.EvidenceHash == "" || r.Findings[0].EvidenceHash == "" {
		t.Fatal("evidence hashes must be present")
	}
	if r.MutationScore != nil {
		t.Fatal("static analysis must not claim mutation score")
	}
	input := GateInputs(r)
	quality := input["quality"].(map[string]any)
	if quality["static_findings"] != 2 || quality["mutation_score"] != nil {
		t.Fatalf("unexpected gate input: %#v", quality)
	}
}

func TestHashEvidenceIsStable(t *testing.T) {
	a, b := HashEvidence("x.go", 3, "x"), HashEvidence("x.go", 3, "x")
	if a != b {
		t.Fatal("evidence hash is not deterministic")
	}
	if a == HashEvidence("x.go", 4, "x") {
		t.Fatal("line must affect evidence hash")
	}
}
