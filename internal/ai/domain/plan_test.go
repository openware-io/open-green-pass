package domain

import (
	"strings"
	"testing"
	"time"
)

func TestCanonicalGenerationPlanHashIsOrderIndependent(t *testing.T) {
	items := []GenerationPlanItem{
		{ServiceID: 51, RepoID: 31, SourcePath: "b", RefType: "branch", RefName: "main", CommitSHA: "b1", Valid: true},
		{ServiceID: 50, RepoID: 30, SourcePath: "a", RefType: "tag", RefName: "v1", CommitSHA: "a1", Valid: true},
	}
	first := CanonicalGenerationPlanHash(20, "project", 0, items)
	reversed := []GenerationPlanItem{items[1], items[0]}
	second := CanonicalGenerationPlanHash(20, "project", 0, reversed)
	if first != second {
		t.Fatalf("hash depends on order: %s != %s", first, second)
	}
	if !strings.HasPrefix(first, "sha256:") || len(first) != len("sha256:")+64 {
		t.Fatalf("unexpected hash format: %s", first)
	}
	changed := []GenerationPlanItem{items[0], items[1]}
	changed[1].CommitSHA = "b2"
	if CanonicalGenerationPlanHash(20, "project", 0, changed) == first {
		t.Fatal("hash does not cover the commit sha")
	}
	if CanonicalGenerationPlanHash(21, "project", 0, items) == first {
		t.Fatal("hash does not cover the project")
	}
	if CanonicalGenerationPlanHash(20, "repo", 0, items) == first {
		t.Fatal("hash does not cover the scope type")
	}
}

func TestGenerationPlanExpiryAndValidItems(t *testing.T) {
	now := time.Now().UTC()
	plan := &GenerationPlan{
		ExpiresAt: now.Add(time.Minute),
		Items: []GenerationPlanItem{
			{ServiceID: 1, Valid: true},
			{ServiceID: 2, Valid: false, Error: "ref_not_found: x"},
		},
	}
	if plan.Expired(now) {
		t.Fatal("fresh plan reported expired")
	}
	expiry := now.Add(2 * time.Minute)
	if !plan.Expired(expiry) {
		t.Fatal("expired plan reported fresh")
	}
	if got := plan.ValidItems(); len(got) != 1 || got[0].ServiceID != 1 {
		t.Fatalf("valid items = %+v", got)
	}
	empty := &GenerationPlan{}
	if !empty.Expired(now) {
		t.Fatal("zero expiry must be treated as expired")
	}
}

func TestGenerationUnitProjectionMapsBatchStates(t *testing.T) {
	cases := []struct {
		batch  GenerationBatchState
		state  GenerationUnitState
		stage  string
		review bool
	}{
		{GenerationEnqueued, GenerationUnitQueued, "enqueued", false},
		{GenerationGenerating, GenerationUnitRunning, "generating", false},
		{GenerationReview, GenerationUnitWaitingReview, "review", true},
		{GenerationCommitting, GenerationUnitRunning, "committing", false},
		{GenerationApproved, GenerationUnitSucceeded, "approved", false},
		{GenerationRejected, GenerationUnitRejected, "rejected", false},
		{GenerationFailed, GenerationUnitFailed, "failed", false},
		{GenerationCancelled, GenerationUnitCancelled, "cancelled", false},
		{GenerationRolledBack, GenerationUnitFailed, "rolled_back", false},
	}
	for _, tc := range cases {
		state, stage, review := GenerationUnitProjection(tc.batch)
		if state != tc.state || stage != tc.stage || review != tc.review {
			t.Fatalf("projection(%s) = (%s,%s,%v), want (%s,%s,%v)", tc.batch, state, stage, review, tc.state, tc.stage, tc.review)
		}
	}
}

func TestFoldGenerationRunState(t *testing.T) {
	unit := func(state GenerationUnitState) *GenerationUnit { return &GenerationUnit{State: state} }
	cases := []struct {
		name      string
		cancelled bool
		units     []*GenerationUnit
		want      GenerationRunState
		completed int
		failed    int
		waiting   int
	}{
		{"empty", false, nil, GenerationRunQueued, 0, 0, 0},
		{"review wins", false, []*GenerationUnit{unit(GenerationUnitSucceeded), unit(GenerationUnitWaitingReview)}, GenerationRunWaitingReview, 1, 0, 1},
		{"all succeeded", false, []*GenerationUnit{unit(GenerationUnitSucceeded), unit(GenerationUnitSucceeded)}, GenerationRunSucceeded, 2, 0, 0},
		{"partial", false, []*GenerationUnit{unit(GenerationUnitSucceeded), unit(GenerationUnitFailed)}, GenerationRunPartialFailed, 1, 1, 0},
		{"all failed", false, []*GenerationUnit{unit(GenerationUnitFailed), unit(GenerationUnitRejected)}, GenerationRunFailed, 0, 2, 0},
		{"running", false, []*GenerationUnit{unit(GenerationUnitRunning), unit(GenerationUnitSucceeded)}, GenerationRunRunning, 1, 0, 0},
		{"cancelled", true, []*GenerationUnit{unit(GenerationUnitCancelled), unit(GenerationUnitSucceeded)}, GenerationRunCancelled, 1, 0, 0},
	}
	for _, tc := range cases {
		state, completed, failed, waiting := FoldGenerationRunState(tc.cancelled, tc.units)
		if state != tc.want || completed != tc.completed || failed != tc.failed || waiting != tc.waiting {
			t.Fatalf("%s: fold = (%s,%d,%d,%d), want (%s,%d,%d,%d)", tc.name, state, completed, failed, waiting, tc.want, tc.completed, tc.failed, tc.waiting)
		}
	}
}

func TestProgressPercentCountsCompletedAndFailedOnly(t *testing.T) {
	cases := []struct {
		total, completed, failed, want int
	}{
		{0, 0, 0, 0},
		{4, 1, 1, 50},
		{4, 0, 0, 0}, // cancelled or pending work is not completed
		{3, 1, 0, 33},
		{2, 2, 0, 100},
	}
	for _, tc := range cases {
		if got := ProgressPercent(tc.total, tc.completed, tc.failed); got != tc.want {
			t.Fatalf("progress(%d,%d,%d) = %d, want %d", tc.total, tc.completed, tc.failed, got, tc.want)
		}
	}
}

func TestGenerationRunEventValidation(t *testing.T) {
	event := &GenerationRunEvent{TeamID: 1, RunID: 2, Type: GenerationRunEventRunCreated, Payload: map[string]any{"state": "queued"}}
	if err := event.Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	unknown := &GenerationRunEvent{TeamID: 1, RunID: 2, Type: "run.unknown"}
	if err := unknown.Validate(); err == nil {
		t.Fatal("unknown event type accepted")
	}
	oversized := &GenerationRunEvent{TeamID: 1, RunID: 2, Type: GenerationRunEventRunProgress, Payload: map[string]any{"blob": strings.Repeat("x", MaxGenerationRunEventPayloadBytes)}}
	if err := oversized.Validate(); err == nil {
		t.Fatal("oversized payload accepted")
	}
	missing := &GenerationRunEvent{Type: GenerationRunEventRunCreated}
	if err := missing.Validate(); err == nil {
		t.Fatal("event without tenant/run accepted")
	}
}

func TestRunAndUnitStateEventTypes(t *testing.T) {
	if got := RunStateEventType(GenerationRunWaitingReview); got != GenerationRunEventRunWaitingReview {
		t.Fatalf("run waiting event = %s", got)
	}
	if got := RunStateEventType(GenerationRunSucceeded); got != GenerationRunEventRunCompleted {
		t.Fatalf("run completed event = %s", got)
	}
	if got := UnitStateEventType(GenerationUnitSucceeded); got != GenerationRunEventUnitApproved {
		t.Fatalf("unit approved event = %s", got)
	}
	if got := UnitStateEventType(GenerationUnitWaitingReview); got != GenerationRunEventUnitReviewRequired {
		t.Fatalf("unit review event = %s", got)
	}
}
