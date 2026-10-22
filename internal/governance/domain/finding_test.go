package domain

import "testing"

func validFinding() *Finding {
	return &Finding{
		Title:    "callback failure",
		Severity: SeverityP0,
		Status:   FindingCandidate,
		Repository: FindingRepositoryRef{
			Name: "open-chat-admin", URL: "https://github.com/openware-io/open-chat-admin",
			Branch: "2.2.1", CommitSHA: "a44876435f69f78723e23c04d2ff1d59e1d05f96",
		},
	}
}

func TestFindingValidateRequiresRepoAndCommit(t *testing.T) {
	f := validFinding()
	f.Repository.CommitSHA = ""
	if err := f.Validate(); err == nil {
		t.Fatal("expected missing commit error")
	}
	f = validFinding()
	f.Severity = "P9"
	if err := f.Validate(); err == nil {
		t.Fatal("expected invalid severity error")
	}
}

func TestFindingTransitionHappyPath(t *testing.T) {
	f := validFinding()
	steps := []struct {
		action FindingAction
		want   FindingStatus
	}{
		{ActionTriage, FindingTriaged},
		{ActionConfirm, FindingConfirmed},
		{ActionPendingRegression, FindingPendingRegression},
		{ActionClose, FindingClosed},
		{ActionReopen, FindingConfirmed},
	}
	for _, step := range steps {
		if _, _, err := f.Transition(step.action, 42, "test"); err != nil {
			t.Fatalf("transition %s: %v", step.action, err)
		}
		if f.Status != step.want {
			t.Fatalf("transition %s: got %s want %s", step.action, f.Status, step.want)
		}
	}
}

func TestFindingTransitionRejectsIllegal(t *testing.T) {
	f := validFinding()
	if _, _, err := f.Transition(ActionClose, 42, ""); err == nil {
		t.Fatal("candidate cannot close")
	}
	f.Status = FindingClosed
	if _, _, err := f.Transition(ActionFalsePositive, 42, ""); err == nil {
		t.Fatal("closed cannot be false positive")
	}
}

func TestFindingDedupeStableAndNormalized(t *testing.T) {
	a := ComputeFindingDedupeHash(" https://github.com/a/b ", "POST /v1/x", "product", "root  cause", "200", "500")
	b := ComputeFindingDedupeHash("https://github.com/a/b", "post /v1/x", "PRODUCT", "root cause", "200", "500")
	if a != b {
		t.Fatalf("dedupe hash should normalize whitespace/case: %s != %s", a, b)
	}
	c := ComputeFindingDedupeHash("https://github.com/a/b", "POST /v1/x", "product", "other", "200", "500")
	if a == c {
		t.Fatal("different root cause must change dedupe hash")
	}
}
