// DSH 生成 Job 领域模型：queued/running/succeeded/failed/timeout/cancelled。
package domain

import (
	"context"
	"time"
)

// GenerationJobState DSH job 状态。
type GenerationJobState string

const (
	GenerationJobQueued    GenerationJobState = "queued"
	GenerationJobRunning   GenerationJobState = "running"
	GenerationJobSucceeded GenerationJobState = "succeeded"
	GenerationJobFailed    GenerationJobState = "failed"
	GenerationJobTimeout   GenerationJobState = "timeout"
	GenerationJobCancelled GenerationJobState = "cancelled"
)

// GenerationJob DSH 生成 Job（gen_job）。
type GenerationJob struct {
	ID             int64
	TeamID         int64
	ProjectID      int64
	ServiceID      *int64
	Module         string
	State          GenerationJobState
	ExternalJobID  string
	Prompt         string
	TimeoutMS      int64
	Output         string
	Error          string
	IdempotencyKey string
	CreatedBy      int64
	CreatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	UpdatedAt      time.Time
}

// GenerationJobEvent job 事件（gen_job_event）。
type GenerationJobEvent struct {
	ID        int64
	TeamID    int64
	JobID     int64
	Seq       int
	Type      string
	State     string
	Message   string
	CreatedAt time.Time
}

// GenerationJobRepository 生成 Job 仓储端口。
type GenerationJobRepository interface {
	Create(ctx context.Context, job *GenerationJob, event *GenerationJobEvent) (bool, error)
	Find(ctx context.Context, teamID, id int64) (*GenerationJob, error)
	FindByKey(ctx context.Context, teamID int64, key string) (*GenerationJob, error)
	Update(ctx context.Context, job *GenerationJob, event *GenerationJobEvent) error
	Events(ctx context.Context, teamID, jobID int64) ([]GenerationJobEvent, error)
}

// Terminal 判断终态。
func (s GenerationJobState) Terminal() bool {
	return s == GenerationJobSucceeded || s == GenerationJobFailed || s == GenerationJobTimeout || s == GenerationJobCancelled
}

// DSHJob is the gateway view of one job.
type DSHJob struct {
	ID        string
	State     string
	Output    string
	TokensIn  int64
	TokensOut int64
	Model     string
	Error     string
}

// DSHJobClient is the internal gateway boundary.
type DSHJobClient interface {
	Submit(ctx context.Context, jobID, prompt string, timeoutMS int64, idempotencyKey string) (DSHJob, error)
	Get(ctx context.Context, jobID string) (DSHJob, error)
	Cancel(ctx context.Context, jobID string) error
}
