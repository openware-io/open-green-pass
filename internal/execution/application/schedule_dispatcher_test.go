package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

type scheduleQueueStub struct {
	enqueued []domain.ScheduleJob
	claimed  []domain.ClaimedScheduleJob
	acked    []domain.ClaimedScheduleJob
}

func (q *scheduleQueueStub) Enqueue(_ context.Context, job domain.ScheduleJob) error {
	q.enqueued = append(q.enqueued, job)
	return nil
}
func (*scheduleQueueStub) Promote(context.Context, int) (int, error) { return 0, nil }
func (q *scheduleQueueStub) Claim(context.Context, string, int, time.Duration) ([]domain.ClaimedScheduleJob, error) {
	return q.claimed, nil
}
func (*scheduleQueueStub) Recover(context.Context, string, int, time.Duration) ([]domain.ClaimedScheduleJob, error) {
	return nil, nil
}
func (q *scheduleQueueStub) Ack(_ context.Context, jobs ...domain.ClaimedScheduleJob) error {
	q.acked = append(q.acked, jobs...)
	return nil
}

type workflowStarterStub struct {
	err           error
	runID, teamID int64
}

func (s *workflowStarterStub) Start(_ context.Context, runID, teamID int64) error {
	s.runID, s.teamID = runID, teamID
	return s.err
}

func TestEnqueueRunEncodesStablePayload(t *testing.T) {
	queue := &scheduleQueueStub{}
	run := &domain.Run{ID: 41, TeamID: 7}
	if err := EnqueueRun(context.Background(), queue, run); err != nil {
		t.Fatalf("enqueue run: %v", err)
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].ID != "41" || queue.enqueued[0].TeamID != 7 || queue.enqueued[0].Weight != 1 {
		t.Fatalf("unexpected job: %#v", queue.enqueued)
	}
	if string(queue.enqueued[0].Payload) != `{"run_id":41,"team_id":7}` {
		t.Fatalf("unexpected payload: %s", queue.enqueued[0].Payload)
	}
}

func TestScheduleDispatcherAcknowledgesStartedWorkflow(t *testing.T) {
	job := domain.ClaimedScheduleJob{ScheduleJob: domain.ScheduleJob{ID: "41", TeamID: 7, Payload: []byte(`{"run_id":41,"team_id":7}`)}, DeliveryID: "1-0"}
	queue := &scheduleQueueStub{claimed: []domain.ClaimedScheduleJob{job}}
	starter := &workflowStarterStub{}
	dispatcher, err := NewScheduleDispatcher(queue, starter, "worker-1", nil)
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}
	if err := dispatcher.consume(context.Background(), false); err != nil {
		t.Fatalf("consume: %v", err)
	}
	if starter.runID != 41 || starter.teamID != 7 || len(queue.acked) != 1 {
		t.Fatalf("start/ack mismatch: starter=%+v acked=%d", starter, len(queue.acked))
	}
}

func TestScheduleDispatcherLeavesFailedWorkflowPending(t *testing.T) {
	job := domain.ClaimedScheduleJob{ScheduleJob: domain.ScheduleJob{ID: "41", TeamID: 7, Payload: []byte(`{"run_id":41,"team_id":7}`)}, DeliveryID: "1-0"}
	queue := &scheduleQueueStub{claimed: []domain.ClaimedScheduleJob{job}}
	starter := &workflowStarterStub{err: errors.New("temporal unavailable")}
	dispatcher, err := NewScheduleDispatcher(queue, starter, "worker-1", nil)
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}
	if err := dispatcher.consume(context.Background(), false); err != nil {
		t.Fatalf("consume: %v", err)
	}
	if len(queue.acked) != 0 {
		t.Fatalf("failed workflow was acknowledged: %#v", queue.acked)
	}
}
