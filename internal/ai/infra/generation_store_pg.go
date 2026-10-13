package infra

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

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

type GenerationSeed struct {
	ID             int64          `json:"id"`
	BatchID        int64          `json:"batch_id"`
	SourceLocator  string         `json:"source_locator"`
	NormalizedSpec map[string]any `json:"normalized_spec"`
	QualityState   string         `json:"quality_state"`
	ApprovalState  string         `json:"approval_state"`
}

func (s *GenerationBatchStore) CreateSeed(ctx context.Context, teamID int64, seedID int64, batchID int64, locator string, spec map[string]any) error {
	payload, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	digest := md5.Sum(append([]byte(locator), payload...))
	dedupeHash := hex.EncodeToString(digest[:])
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO gen_seed (id,team_id,batch_id,source_locator,normalized_spec,dedupe_hash) VALUES ($1,$2,$3,$4,$5::jsonb,$6) ON CONFLICT (batch_id,dedupe_hash) DO NOTHING`, seedID, teamID, batchID, locator, payload, dedupeHash)
		return err
	})
}

func (s *GenerationBatchStore) MarkSeedsQuality(ctx context.Context, teamID, batchID int64, state string) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE gen_seed SET quality_state=$3 WHERE team_id=$1 AND batch_id=$2`, teamID, batchID, state)
		return err
	})
}

func (s *GenerationBatchStore) CommitSeeds(ctx context.Context, teamID, batchID int64, genNext func() int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var targetID, repoID int64
		var branch string
		if err := tx.QueryRow(ctx, `SELECT target_id,repo_id,branch FROM gen_batch WHERE team_id=$1 AND id=$2`, teamID, batchID).Scan(&targetID, &repoID, &branch); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id,normalized_spec FROM gen_seed WHERE team_id=$1 AND batch_id=$2 AND quality_state='passed'`, teamID, batchID)
		if err != nil {
			return err
		}
		type seedRow struct {
			id  int64
			raw []byte
		}
		seeds := make([]seedRow, 0)
		for rows.Next() {
			var seedID int64
			var raw []byte
			if err := rows.Scan(&seedID, &raw); err != nil {
				rows.Close()
				return err
			}
			seeds = append(seeds, seedRow{id: seedID, raw: raw})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, seed := range seeds {
			seedID, raw := seed.id, seed.raw
			var spec map[string]any
			if err := json.Unmarshal(raw, &spec); err != nil {
				return err
			}
			code := fmt.Sprintf("GEN-%d-%d", batchID, seedID)
			title, _ := spec["title"].(string)
			if title == "" {
				title = fmt.Sprintf("生成候选种子 %d", seedID)
			}
			caseID, versionID := genNext(), genNext()
			if _, err := tx.Exec(ctx, `INSERT INTO cas_case (id,team_id,target_id,code,title,kind,created_by) VALUES ($1,$2,$3,$4,$5,'api',1) ON CONFLICT (team_id,target_id,code) DO NOTHING`, caseID, teamID, targetID, code, title); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO cas_version (id,team_id,case_id,version,change_type,source_repo_id,source_branch,script_json,approved_at) SELECT $1,$2,id,1,'added',$3,$4,$5,now() FROM cas_case WHERE team_id=$2 AND target_id=$6 AND code=$7 ON CONFLICT DO NOTHING`, versionID, teamID, repoID, branch, raw, targetID, code); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE gen_seed SET approval_state='approved' WHERE team_id=$1 AND batch_id=$2 AND id=$3`, teamID, batchID, seedID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GenerationBatchStore) ListSeeds(ctx context.Context, teamID, batchID int64) ([]GenerationSeed, error) {
	out := make([]GenerationSeed, 0)
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,batch_id,source_locator,normalized_spec,quality_state,approval_state FROM gen_seed WHERE team_id=$1 AND batch_id=$2 ORDER BY id`, teamID, batchID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var seed GenerationSeed
			var normalized []byte
			if err := rows.Scan(&seed.ID, &seed.BatchID, &seed.SourceLocator, &normalized, &seed.QualityState, &seed.ApprovalState); err != nil {
				return err
			}
			if len(normalized) > 0 {
				if err := json.Unmarshal(normalized, &seed.NormalizedSpec); err != nil {
					return err
				}
			}
			out = append(out, seed)
		}
		return rows.Err()
	})
	return out, err
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
	if err := row.Scan(&b.ID, &b.TeamID, &b.TargetID, &b.RepoID, &b.Branch, &b.RequestedVersion, &b.HeadSHA, &state, &b.StateVersion, &b.WorkflowID, &b.IdempotencyKey, &b.ParentBatchID, &b.CreatedBy, &b.FailureSummary, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return err
	}
	b.State = domain.GenerationBatchState(state)
	return nil
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
