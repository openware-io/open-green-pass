package domain

import "testing"

func TestRunPauseResumeStateTransitions(t *testing.T) {
	r := NewRun(1, 100, 1, 1003, "test", "v1", "main", "manual", []int64{4001})
	if err := r.StartVersionCheck(); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkVersionChecked(true, "v1", nil); err != nil {
		t.Fatal(err)
	}
	if err := r.Schedule(); err != nil {
		t.Fatal(err)
	}
	if err := r.Pause(); err != nil {
		t.Fatal(err)
	}
	if r.State != RunPaused {
		t.Fatalf("state=%s, want paused", r.State)
	}
	if err := r.Resume(); err != nil {
		t.Fatal(err)
	}
	if r.State != RunRunning {
		t.Fatalf("state=%s, want running", r.State)
	}
}
