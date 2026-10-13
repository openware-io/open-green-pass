package infra

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	governance "github.com/openware-io/open-green-pass/internal/governance/infra"
)

// GenerationBatchStore persists the generation state machine in gen_batch.
// The application service remains responsible for legal transitions; this
// repository only provides tenant-scoped reads and compare-and-swap writes.
type GenerationBatchStore struct {
	db *governance.DB
}

func NewGenerationBatchStore(db *governance.DB) *GenerationBatchStore {
	return &GenerationBatchStore{db: db}
}

func (s *GenerationBatchStore) Create(ctx context.Context, b *domain.GenerationBatch) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO gen_batch
            (id,team_id,target_id,repo_id,branch,requested_version,state,state_version,workflow_id,idempotency_key,parent_batch_id,created_by)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			b.ID, b.TeamID, b.TargetID, b.RepoID, b.Branch, nullString(b.RequestedVersion), string(b.State), b.StateVersion,
			b.WorkflowIdentity(), b.IdempotencyKey, b.ParentBatchID, b.CreatedBy)
		return err
	})
}

func (s *GenerationBatchStore) Find(ctx context.Context, teamID, id int64) (*domain.GenerationBatch, error) {
	var b domain.GenerationBatch
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return scanBatch(tx.QueryRow(ctx, `SELECT id,team_id,target_id,repo_id,branch,COALESCE(requested_version,''),COALESCE(head_sha,''),state,state_version,COALESCE(workflow_id,''),idempotency_key,parent_batch_id,created_by,COALESCE(failure_summary,''),created_at,updated_at FROM gen_batch WHERE team_id=$1 AND id=$2`, teamID, id), &b)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGenerationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *GenerationBatchStore) FindByIdempotencyKey(ctx context.Context, teamID int64, key string) (*domain.GenerationBatch, error) {
	var b domain.GenerationBatch
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return scanBatch(tx.QueryRow(ctx, `SELECT id,team_id,target_id,repo_id,branch,COALESCE(requested_version,''),COALESCE(head_sha,''),state,state_version,COALESCE(workflow_id,''),idempotency_key,parent_batch_id,created_by,COALESCE(failure_summary,''),created_at,updated_at FROM gen_batch WHERE team_id=$1 AND idempotency_key=$2`, teamID, key), &b)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGenerationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *GenerationBatchStore) CompareAndSwap(ctx context.Context, b *domain.GenerationBatch, expected int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `UPDATE gen_batch SET state=$3,state_version=$4,updated_at=$5,updated_by=$6 WHERE team_id=$1 AND id=$2 AND state_version=$7`, b.TeamID, b.ID, string(b.State), b.StateVersion, b.UpdatedAt, b.CreatedBy, expected)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return domain.ErrGenerationStateConflict
		}
		return nil
	})
}

func scanBatch(row pgx.Row, b *domain.GenerationBatch) error {
	var state string
	return row.Scan(&b.ID, &b.TeamID, &b.TargetID, &b.RepoID, &b.Branch, &b.RequestedVersion, &b.HeadSHA, &state, &b.StateVersion, &b.WorkflowID, &b.IdempotencyKey, &b.ParentBatchID, &b.CreatedBy, &b.FailureSummary, &b.CreatedAt, &b.UpdatedAt)
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
