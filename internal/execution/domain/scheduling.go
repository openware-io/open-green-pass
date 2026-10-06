package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidScheduleJob = errors.New("invalid schedule job")
	ErrScheduleJobExists  = errors.New("schedule job already exists")
)

type ScheduleJob struct {
	ID        string
	TeamID    int64
	Weight    int
	Payload   []byte
	CreatedAt time.Time
}
type ClaimedScheduleJob struct {
	ScheduleJob
	DeliveryID string
}
type ScheduleQueuePort interface {
	Enqueue(context.Context, ScheduleJob) error
	Promote(context.Context, int) (int, error)
	Claim(context.Context, string, int, time.Duration) ([]ClaimedScheduleJob, error)
	Recover(context.Context, string, int, time.Duration) ([]ClaimedScheduleJob, error)
	Ack(context.Context, ...ClaimedScheduleJob) error
}
