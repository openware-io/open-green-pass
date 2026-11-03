// Package infra DSH 生成 Job 仓储（gen_job/gen_job_event，RLS）。
package infra

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	governance "github.com/openware-io/open-green-pass/internal/governance/infra"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// GenerationJobStore 生成 Job 仓储。
type GenerationJobStore struct {
	db  *governance.DB
	gen *id.Generator
}

// NewGenerationJobStore 创建仓储。
func NewGenerationJobStore(db *governance.DB, gen *id.Generator) *GenerationJobStore {
	return &GenerationJobStore{db: db, gen: gen}
}

// Create 写 job + 首个事件；幂等键命中返回 created=false。
func (s *GenerationJobStore) Create(ctx context.Context, job *domain.GenerationJob, event *domain.GenerationJobEvent) (bool, error) {
	created := false
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO gen_job(
				id, team_id, project_id, service_id, module, state, external_job_id, prompt,
				timeout_ms, output, error, idempotency_key, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (team_id, idempotency_key) DO NOTHING`,
			job.ID, job.TeamID, job.ProjectID, job.ServiceID, job.Module, job.State,
			job.ExternalJobID, job.Prompt, job.TimeoutMS, job.Output, job.Error,
			job.IdempotencyKey, job.CreatedBy)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		created = true
		if event == nil {
			return nil
		}
		event.TeamID = job.TeamID
		event.JobID = job.ID
		_, err = tx.Exec(ctx, `
			INSERT INTO gen_job_event(id, team_id, job_id, seq, type, state, message)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			event.ID, event.TeamID, event.JobID, event.Seq, event.Type, event.State, event.Message)
		return err
	})
	return created, err
}

// Find 查询 job。
func (s *GenerationJobStore) Find(ctx context.Context, teamID, id int64) (*domain.GenerationJob, error) {
	var job domain.GenerationJob
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id, team_id, project_id, service_id, module, state, external_job_id, prompt,
				timeout_ms, output, error, idempotency_key, created_by, created_at, started_at,
				finished_at, updated_at
			FROM gen_job WHERE team_id=$1 AND id=$2`, teamID, id).Scan(
			&job.ID, &job.TeamID, &job.ProjectID, &job.ServiceID, &job.Module, &job.State,
			&job.ExternalJobID, &job.Prompt, &job.TimeoutMS, &job.Output, &job.Error,
			&job.IdempotencyKey, &job.CreatedBy, &job.CreatedAt, &job.StartedAt,
			&job.FinishedAt, &job.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// FindByKey 查询幂等键。
func (s *GenerationJobStore) FindByKey(ctx context.Context, teamID int64, key string) (*domain.GenerationJob, error) {
	var job domain.GenerationJob
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id, team_id, project_id, service_id, module, state, external_job_id, prompt,
				timeout_ms, output, error, idempotency_key, created_by, created_at, started_at,
				finished_at, updated_at
			FROM gen_job WHERE team_id=$1 AND idempotency_key=$2`, teamID, key).Scan(
			&job.ID, &job.TeamID, &job.ProjectID, &job.ServiceID, &job.Module, &job.State,
			&job.ExternalJobID, &job.Prompt, &job.TimeoutMS, &job.Output, &job.Error,
			&job.IdempotencyKey, &job.CreatedBy, &job.CreatedAt, &job.StartedAt,
			&job.FinishedAt, &job.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// Update 更新 job + 事件。
func (s *GenerationJobStore) Update(ctx context.Context, job *domain.GenerationJob, event *domain.GenerationJobEvent) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE gen_job SET state=$1, external_job_id=$2, output=$3, error=$4,
				started_at=$5, finished_at=$6, updated_at=$7
			WHERE team_id=$8 AND id=$9`,
			job.State, job.ExternalJobID, job.Output, job.Error, job.StartedAt, job.FinishedAt,
			time.Now().UTC(), job.TeamID, job.ID)
		if err != nil {
			return err
		}
		if event == nil {
			return nil
		}
		event.TeamID = job.TeamID
		event.JobID = job.ID
		_, err = tx.Exec(ctx, `
			INSERT INTO gen_job_event(id, team_id, job_id, seq, type, state, message)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			event.ID, event.TeamID, event.JobID, event.Seq, event.Type, event.State, event.Message)
		return err
	})
}

// ReadOutput returns the terminal state/output for the governance import port.
func (s *GenerationJobStore) ReadOutput(ctx context.Context, teamID, jobID int64) (string, string, error) {
	job, err := s.Find(ctx, teamID, jobID)
	if err != nil {
		return "", "", generationJobNotFoundError(err)
	}
	return string(job.State), job.Output, nil
}

// ReadJobProject returns the job's project for authorization before an import.
// It is tenant-scoped, so a guessed job id cannot resolve another tenant.
func (s *GenerationJobStore) ReadJobProject(ctx context.Context, teamID, jobID int64) (int64, error) {
	job, err := s.Find(ctx, teamID, jobID)
	if err != nil {
		return 0, generationJobNotFoundError(err)
	}
	return job.ProjectID, nil
}

// generationJobNotFoundError maps the repository's no-rows error to the shared
// 404 kind so the HTTP adapters return not_found instead of 500/400.
func generationJobNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return gperr.NotFound("generation job not found")
	}
	return err
}

// Events 查询事件。
func (s *GenerationJobStore) Events(ctx context.Context, teamID, jobID int64) ([]domain.GenerationJobEvent, error) {
	var out []domain.GenerationJobEvent
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, team_id, job_id, seq, type, state, message, created_at
			FROM gen_job_event WHERE team_id=$1 AND job_id=$2 ORDER BY seq`, teamID, jobID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e domain.GenerationJobEvent
			if err := rows.Scan(&e.ID, &e.TeamID, &e.JobID, &e.Seq, &e.Type, &e.State, &e.Message, &e.CreatedAt); err != nil {
				return err
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}
