package domain

import "testing"

func TestRunCancelTransitions(t *testing.T) {
	states := []RunState{RunQueued, RunVersionCheck, RunScheduled, RunRunning, RunPaused, RunCollect, RunGate, RunReport}
	for _, state := range states {
		run := &Run{State: state}
		if err := run.Cancel(); err != nil {
			t.Fatalf("cancel from %s: %v", state, err)
		}
		if run.State != RunCancelled {
			t.Fatalf("state after cancel from %s = %s, want cancelled", state, run.State)
		}
	}
	for _, terminal := range []RunState{RunDone, RunFailed, RunCancelled} {
		run := &Run{State: terminal}
		if err := run.Cancel(); err != ErrRunFinished {
			t.Fatalf("cancel from terminal %s err = %v, want ErrRunFinished", terminal, err)
		}
	}
	run := &Run{State: RunCancelled}
	for _, next := range []RunState{RunRunning, RunCollect, RunGate, RunReport, RunDone, RunFailed} {
		if err := run.transition(next); err != ErrRunFinished {
			t.Fatalf("transition from cancelled to %s err = %v, want ErrRunFinished", next, err)
		}
	}
}
