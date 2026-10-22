package dshgw

import (
	"context"
	"errors"
	"testing"
	"time"
)

type blockingRunner struct{}

func (blockingRunner) Run(ctx context.Context, _ Job) (Result, error) {
	<-ctx.Done()
	return Result{}, ctx.Err()
}

func waitState(t *testing.T, m *Manager, id string, want State) *Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := m.Get(id)
		if ok && job.State == want {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, _ := m.Get(id)
	t.Fatalf("job %s state=%v want %v", id, job.State, want)
	return nil
}

func TestSubmitRunsAndSucceeds(t *testing.T) {
	m := NewManager(StubRunner{Delay: 10 * time.Millisecond}, 1, 2)
	defer m.Close()
	if _, err := m.Submit(Job{ID: "j1", Prompt: "hello"}); err != nil {
		t.Fatal(err)
	}
	job := waitState(t, m, "j1", StateSucceeded)
	if job.Output != "stub:j1" {
		t.Fatalf("output=%q", job.Output)
	}
	if len(m.Events("j1")) != 3 {
		t.Fatalf("events=%d", len(m.Events("j1")))
	}
}

func TestIdempotencyAndQueueFull(t *testing.T) {
	m := NewManager(blockingRunner{}, 1, 1)
	defer m.Close()
	if _, err := m.Submit(Job{ID: "j1", Prompt: "a", IdempotencyKey: "k1"}); err != nil {
		t.Fatal(err)
	}
	// first job running or queued; second fills queue; third rejected.
	_, err1 := m.Submit(Job{ID: "j2", Prompt: "b"})
	if err1 != nil && !errors.Is(err1, ErrQueueFull) {
		t.Fatalf("unexpected: %v", err1)
	}
	if _, err := m.Submit(Job{ID: "j3", Prompt: "c"}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("want queue full, got %v", err)
	}
	first, err := m.Submit(Job{ID: "j1", Prompt: "a", IdempotencyKey: "k1"})
	if err != nil || first.ID != "j1" {
		t.Fatalf("idempotent submit: %v %+v", err, first)
	}
}

func TestCancelRunningJob(t *testing.T) {
	m := NewManager(blockingRunner{}, 1, 1)
	defer m.Close()
	if _, err := m.Submit(Job{ID: "j1", Prompt: "a", IdempotencyKey: "k1"}); err != nil {
		t.Fatal(err)
	}
	waitState(t, m, "j1", StateRunning)
	if !m.Cancel("j1") {
		t.Fatal("cancel returned false")
	}
	job := waitState(t, m, "j1", StateCancelled)
	if job.FinishedAt == nil {
		t.Fatal("cancelled job missing finished_at")
	}
}

func TestTimeout(t *testing.T) {
	m := NewManager(blockingRunner{}, 1, 1)
	defer m.Close()
	if _, err := m.Submit(Job{ID: "j1", Prompt: "a", Timeout: 20 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	waitState(t, m, "j1", StateTimeout)
}
