// Package application DSH 生成 Job 编排：GP 持久化 + gateway 提交/查询/取消。
package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// CreateGenerationJobInput is the HTTP/application input.
type CreateGenerationJobInput struct {
	ProjectID      int64
	ServiceID      *int64
	Module         string
	Prompt         string
	TimeoutMS      int64
	IdempotencyKey string
}

// GenerationJobService owns job persistence and gateway calls.
type GenerationJobService struct {
	repo   domain.GenerationJobRepository
	client domain.DSHJobClient
	gen    *id.Generator
}

// NewGenerationJobService creates the service.
func NewGenerationJobService(repo domain.GenerationJobRepository, client domain.DSHJobClient, gen *id.Generator) *GenerationJobService {
	return &GenerationJobService{repo: repo, client: client, gen: gen}
}

// Create persists and submits one job. Duplicate idempotency keys return the
// existing job and do not submit a second gateway job.
func (s *GenerationJobService) Create(ctx context.Context, in CreateGenerationJobInput) (*domain.GenerationJob, bool, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, false, gperr.Unauthorized("tenant required")
	}
	actor, _ := rls.UserFrom(ctx)
	if in.ProjectID <= 0 {
		return nil, false, gperr.Validation("project_id required")
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return nil, false, gperr.Validation("prompt required")
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return nil, false, gperr.Validation("idempotency_key required")
	}
	if in.TimeoutMS <= 0 {
		in.TimeoutMS = 900_000
	}
	now := time.Now().UTC()
	job := &domain.GenerationJob{
		ID: s.gen.Next(), TeamID: teamID, ProjectID: in.ProjectID, ServiceID: in.ServiceID,
		Module: in.Module, State: domain.GenerationJobQueued, Prompt: in.Prompt,
		TimeoutMS: in.TimeoutMS, IdempotencyKey: key, CreatedBy: actor, CreatedAt: now, UpdatedAt: now,
	}
	created, err := s.repo.Create(ctx, job, &domain.GenerationJobEvent{
		ID: s.gen.Next(), Seq: 1, Type: "state", State: string(domain.GenerationJobQueued), Message: "queued",
	})
	if err != nil {
		return nil, false, err
	}
	if !created {
		existing, findErr := s.repo.FindByKey(ctx, teamID, key)
		if findErr != nil {
			return nil, false, findErr
		}
		return existing, false, nil
	}
	if s.client == nil {
		s.fail(ctx, job, "gateway not configured")
		return job, true, gperr.Validation("DSH gateway not configured")
	}
	remote, err := s.client.Submit(ctx, externalJobID(job), job.Prompt, job.TimeoutMS, job.IdempotencyKey)
	if err != nil {
		s.fail(ctx, job, err.Error())
		return job, true, err
	}
	job.ExternalJobID = remote.ID
	job.State = domain.GenerationJobState(remote.State)
	if job.State == "" {
		job.State = domain.GenerationJobQueued
	}
	_ = s.repo.Update(ctx, job, &domain.GenerationJobEvent{
		ID: s.gen.Next(), Seq: 2, Type: "state", State: string(job.State), Message: "submitted",
	})
	return job, true, nil
}

// Get refreshes a non-terminal job from the gateway.
func (s *GenerationJobService) Get(ctx context.Context, id int64) (*domain.GenerationJob, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	job, err := s.repo.Find(ctx, teamID, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, gperr.NotFound("generation job not found")
		}
		return nil, err
	}
	if s.client != nil && !job.State.Terminal() && job.ExternalJobID != "" {
		remote, getErr := s.client.Get(ctx, job.ExternalJobID)
		if getErr == nil {
			next := domain.GenerationJobState(remote.State)
			if next != "" && next != job.State {
				job.State = next
				job.Output = remote.Output
				job.Error = remote.Error
				if next.Terminal() {
					now := time.Now().UTC()
					job.FinishedAt = &now
				}
				seq := int(time.Now().UnixNano() % 1_000_000_000)
				_ = s.repo.Update(ctx, job, &domain.GenerationJobEvent{
					ID: s.gen.Next(), Seq: seq, Type: "state", State: string(next), Message: "polled",
				})
			}
		}
	}
	return job, nil
}

// Cancel requests cancellation.
func (s *GenerationJobService) Cancel(ctx context.Context, id int64) error {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return gperr.Unauthorized("tenant required")
	}
	job, err := s.repo.Find(ctx, teamID, id)
	if err != nil {
		return err
	}
	if job.State.Terminal() {
		return gperr.Conflict("job already terminal")
	}
	if s.client != nil && job.ExternalJobID != "" {
		_ = s.client.Cancel(ctx, job.ExternalJobID)
	}
	job.State = domain.GenerationJobCancelled
	now := time.Now().UTC()
	job.FinishedAt = &now
	return s.repo.Update(ctx, job, &domain.GenerationJobEvent{
		ID: s.gen.Next(), Seq: int(now.UnixNano() % 1_000_000_000), Type: "state",
		State: string(domain.GenerationJobCancelled), Message: "cancelled",
	})
}

func (s *GenerationJobService) fail(ctx context.Context, job *domain.GenerationJob, message string) {
	job.State = domain.GenerationJobFailed
	job.Error = message
	now := time.Now().UTC()
	job.FinishedAt = &now
	_ = s.repo.Update(ctx, job, &domain.GenerationJobEvent{
		ID: s.gen.Next(), Seq: 2, Type: "state", State: string(domain.GenerationJobFailed), Message: message,
	})
}

// externalJobID is the stable gateway job id.
func externalJobID(j *domain.GenerationJob) string {
	if j.ExternalJobID != "" {
		return j.ExternalJobID
	}
	return "gp-" + itoa(j.ID)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
