// Package dshgw is the GP-owned DSH gateway core: bounded queue, one job per
// worker, cancellation, and a pluggable runner. It intentionally owns no GP
// business data and never touches the gp database.
package dshgw

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// State is the job lifecycle state.
type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateTimeout   State = "timeout"
	StateCancelled State = "cancelled"
)

// Job is one bounded DSH invocation request.
type Job struct {
	ID             string        `json:"job_id"`
	Prompt         string        `json:"prompt"`
	Profile        string        `json:"profile,omitempty"`
	Timeout        time.Duration `json:"-"`
	State          State         `json:"state"`
	Output         string        `json:"output,omitempty"`
	TokensIn       int64         `json:"tokens_in"`
	TokensOut      int64         `json:"tokens_out"`
	Error          string        `json:"error,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	StartedAt      *time.Time    `json:"started_at,omitempty"`
	FinishedAt     *time.Time    `json:"finished_at,omitempty"`
	IdempotencyKey string        `json:"idempotency_key,omitempty"`
}

// Event is an append-only job event (kept in memory for the skeleton).
type Event struct {
	Seq     int       `json:"seq"`
	Type    string    `json:"type"`
	State   State     `json:"state"`
	At      time.Time `json:"at"`
	Message string    `json:"message,omitempty"`
}

// Result is returned by a Runner.
type Result struct {
	Output    string
	TokensIn  int64
	TokensOut int64
}

// Runner executes one job. A production implementation wraps the DSH SDK
// child process; the skeleton uses StubRunner.
type Runner interface {
	Run(ctx context.Context, job Job) (Result, error)
}

// Manager owns the queue and workers.
type Manager struct {
	mu      sync.RWMutex
	jobs    map[string]*Job
	events  map[string][]Event
	cancels map[string]context.CancelFunc
	byKey   map[string]string
	queue   chan *Job
	runner  Runner
	wg      sync.WaitGroup
	closed  bool
}

// NewManager starts the worker pool.
func NewManager(runner Runner, workers, queueSize int) *Manager {
	if workers < 1 {
		workers = 1
	}
	if queueSize < 1 {
		queueSize = 64
	}
	m := &Manager{
		jobs: map[string]*Job{}, events: map[string][]Event{}, cancels: map[string]context.CancelFunc{},
		byKey: map[string]string{}, queue: make(chan *Job, queueSize), runner: runner,
	}
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go m.worker()
	}
	return m
}

// Submit enqueues a job. Duplicate idempotency keys return the existing job.
func (m *Manager) Submit(job Job) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}
	if job.ID == "" {
		return nil, errors.New("job id required")
	}
	if job.IdempotencyKey != "" {
		if id, ok := m.byKey[job.IdempotencyKey]; ok {
			return cloneJob(m.jobs[id]), nil
		}
	}
	if _, exists := m.jobs[job.ID]; exists {
		return nil, fmt.Errorf("job %s already exists", job.ID)
	}
	now := time.Now().UTC()
	job.State = StateQueued
	job.CreatedAt = now
	if job.Timeout <= 0 {
		job.Timeout = 15 * time.Minute
	}
	copyJob := job
	m.jobs[job.ID] = &copyJob
	if job.IdempotencyKey != "" {
		m.byKey[job.IdempotencyKey] = job.ID
	}
	m.appendEventLocked(job.ID, Event{Type: "state", State: StateQueued, At: now, Message: "queued"})
	select {
	case m.queue <- &copyJob:
		return cloneJob(&copyJob), nil
	default:
		delete(m.jobs, job.ID)
		if job.IdempotencyKey != "" {
			delete(m.byKey, job.IdempotencyKey)
		}
		return nil, ErrQueueFull
	}
}

// Get returns a snapshot.
func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, false
	}
	return cloneJob(job), true
}

// Events returns a copy of job events.
func (m *Manager) Events(id string) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Event, len(m.events[id]))
	copy(out, m.events[id])
	return out
}

// Cancel requests cancellation; the worker observes ctx.Done and marks the job.
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return false
	}
	if job.State == StateSucceeded || job.State == StateFailed || job.State == StateCancelled || job.State == StateTimeout {
		return false
	}
	job.State = StateCancelled
	now := time.Now().UTC()
	job.FinishedAt = &now
	if cancel, ok := m.cancels[id]; ok {
		cancel()
	}
	m.appendEventLocked(id, Event{Type: "state", State: StateCancelled, At: now, Message: "cancelled by request"})
	return true
}

// Close stops workers and marks queued jobs cancelled.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	close(m.queue)
	for _, cancel := range m.cancels {
		cancel()
	}
	for id, job := range m.jobs {
		if job.State == StateQueued || job.State == StateRunning {
			job.State = StateCancelled
			now := time.Now().UTC()
			job.FinishedAt = &now
			m.appendEventLocked(id, Event{Type: "state", State: StateCancelled, At: now, Message: "gateway shutdown"})
		}
	}
	m.mu.Unlock()
	m.wg.Wait()
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for job := range m.queue {
		m.run(job)
	}
}

func (m *Manager) run(job *Job) {
	m.mu.Lock()
	current, ok := m.jobs[job.ID]
	if !ok || current.State != StateQueued {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), job.Timeout)
	m.cancels[job.ID] = cancel
	now := time.Now().UTC()
	job.State = StateRunning
	job.StartedAt = &now
	m.appendEventLocked(job.ID, Event{Type: "state", State: StateRunning, At: now, Message: "running"})
	m.mu.Unlock()
	defer cancel()

	type outcome struct {
		result Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := m.runner.Run(ctx, *job)
		done <- outcome{result: result, err: err}
	}()

	var out outcome
	select {
	case out = <-done:
	case <-ctx.Done():
		out.err = ctx.Err()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if current, ok := m.jobs[job.ID]; ok && current.State == StateCancelled {
		return
	}
	finished := time.Now().UTC()
	job.FinishedAt = &finished
	delete(m.cancels, job.ID)
	switch {
	case out.err == nil:
		job.State = StateSucceeded
		job.Output = out.result.Output
		job.TokensIn = out.result.TokensIn
		job.TokensOut = out.result.TokensOut
		m.appendEventLocked(job.ID, Event{Type: "state", State: StateSucceeded, At: finished, Message: "succeeded"})
	case errors.Is(out.err, context.DeadlineExceeded):
		job.State = StateTimeout
		job.Error = "timeout"
		m.appendEventLocked(job.ID, Event{Type: "state", State: StateTimeout, At: finished, Message: "timeout"})
	case errors.Is(out.err, context.Canceled):
		job.State = StateCancelled
		m.appendEventLocked(job.ID, Event{Type: "state", State: StateCancelled, At: finished, Message: "cancelled"})
	default:
		job.State = StateFailed
		job.Error = out.err.Error()
		m.appendEventLocked(job.ID, Event{Type: "state", State: StateFailed, At: finished, Message: out.err.Error()})
	}
}

func (m *Manager) appendEventLocked(id string, event Event) {
	event.Seq = len(m.events[id]) + 1
	m.events[id] = append(m.events[id], event)
}

func cloneJob(in *Job) *Job {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

var (
	// ErrQueueFull is returned when the bounded queue is full.
	ErrQueueFull = errors.New("dsh queue full")
	// ErrClosed is returned after shutdown.
	ErrClosed = errors.New("dsh gateway closed")
)
