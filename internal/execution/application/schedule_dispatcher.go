package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

// WorkflowStarter starts one durable execution workflow. Implementations must
// treat an already-started workflow ID as success so redelivery remains safe.
type WorkflowStarter interface {
	Start(context.Context, int64, int64) error
}

type scheduledRun struct {
	RunID  int64 `json:"run_id"`
	TeamID int64 `json:"team_id"`
}

// EnqueueRun persists a newly created run in the shared weighted-fair queue.
func EnqueueRun(ctx context.Context, queue domain.ScheduleQueuePort, run *domain.Run) error {
	if queue == nil || run == nil || run.ID <= 0 || run.TeamID <= 0 {
		return domain.ErrInvalidScheduleJob
	}
	payload, err := json.Marshal(scheduledRun{RunID: run.ID, TeamID: run.TeamID})
	if err != nil {
		return fmt.Errorf("encode scheduled run: %w", err)
	}
	return queue.Enqueue(ctx, domain.ScheduleJob{
		ID: strconv.FormatInt(run.ID, 10), TeamID: run.TeamID, Weight: 1,
		Payload: payload, CreatedAt: time.Now().UTC(),
	})
}

// ScheduleDispatcher owns the promoter and consumer lifecycle for one worker.
// Canceling ctx stops both loops; failed deliveries remain pending for recovery.
type ScheduleDispatcher struct {
	queue       domain.ScheduleQueuePort
	starter     WorkflowStarter
	consumer    string
	batch       int
	block       time.Duration
	recoverIdle time.Duration
	log         *slog.Logger
}

// NewScheduleDispatcher validates dependencies for the queue-to-workflow bridge.
func NewScheduleDispatcher(queue domain.ScheduleQueuePort, starter WorkflowStarter, consumer string, log *slog.Logger) (*ScheduleDispatcher, error) {
	if queue == nil || starter == nil || consumer == "" {
		return nil, errors.New("schedule queue, workflow starter and consumer are required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ScheduleDispatcher{queue: queue, starter: starter, consumer: consumer, batch: 16, block: time.Second, recoverIdle: 30 * time.Second, log: log}, nil
}

// Run promotes WFQ entries and consumes the Redis Stream until ctx is canceled.
func (d *ScheduleDispatcher) Run(ctx context.Context) error {
	promote := time.NewTicker(250 * time.Millisecond)
	recoverTicker := time.NewTicker(d.recoverIdle)
	defer promote.Stop()
	defer recoverTicker.Stop()
	if err := d.consume(ctx, true); err != nil {
		d.log.Warn("recover scheduled runs failed", "consumer", d.consumer, "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-promote.C:
			if _, err := d.queue.Promote(ctx, d.batch); err != nil && ctx.Err() == nil {
				d.log.Warn("promote scheduled runs failed", "consumer", d.consumer, "err", err)
			}
		case <-recoverTicker.C:
			if err := d.consume(ctx, true); err != nil && ctx.Err() == nil {
				d.log.Warn("recover scheduled runs failed", "consumer", d.consumer, "err", err)
			}
		default:
			if err := d.consume(ctx, false); err != nil && ctx.Err() == nil {
				d.log.Warn("consume scheduled runs failed", "consumer", d.consumer, "err", err)
			}
		}
	}
}

func (d *ScheduleDispatcher) consume(ctx context.Context, recoverPending bool) error {
	var jobs []domain.ClaimedScheduleJob
	var err error
	if recoverPending {
		jobs, err = d.queue.Recover(ctx, d.consumer, d.batch, d.recoverIdle)
	} else {
		jobs, err = d.queue.Claim(ctx, d.consumer, d.batch, d.block)
	}
	if err != nil {
		return err
	}
	for _, job := range jobs {
		var payload scheduledRun
		if err := json.Unmarshal(job.Payload, &payload); err != nil || payload.RunID <= 0 || payload.TeamID <= 0 || payload.TeamID != job.TeamID {
			d.log.Error("invalid scheduled run payload", "job_id", job.ID, "delivery_id", job.DeliveryID, "err", err)
			if ackErr := d.queue.Ack(ctx, job); ackErr != nil {
				return fmt.Errorf("ack invalid scheduled run %s: %w", job.ID, ackErr)
			}
			continue
		}
		if err := d.starter.Start(ctx, payload.RunID, payload.TeamID); err != nil {
			d.log.Warn("start scheduled workflow failed", "run_id", payload.RunID, "tenant_id", payload.TeamID, "err", err)
			continue
		}
		if err := d.queue.Ack(ctx, job); err != nil {
			return fmt.Errorf("ack scheduled run %d: %w", payload.RunID, err)
		}
	}
	return nil
}
