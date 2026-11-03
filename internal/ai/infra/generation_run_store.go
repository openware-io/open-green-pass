package infra

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/platform/db"
)

// GenerationRunStore persists generation runs and their service units.
//
// Reads are pure: the repository never updates tokens/cost or unit states while
// loading a run. The worker reconciler owns state convergence, which removes the
// old GET side effect that rewrote rows on every poll.
type GenerationRunStore struct {
	db   *db.DB
	cost domain.GenerationCostReadPort
}

// NewGenerationRunStore creates the store. The trusted cost read port is
// injected separately when the cross-domain adapter is available; without it
// cost_amount stays zero (only embedded/test compositions rely on that).
func NewGenerationRunStore(database *db.DB) *GenerationRunStore {
	return &GenerationRunStore{db: database}
}

// SetCostReadPort wires the trusted-domain batch cost reader. It is a port, not
// a direct table read: AI infra must not JOIN cost_line_item.
func (s *GenerationRunStore) SetCostReadPort(port domain.GenerationCostReadPort) {
	s.cost = port
}

var _ domain.GenerationRunRepository = (*GenerationRunStore)(nil)

const runSelectColumns = `
	r.id,r.team_id,r.project_id,r.scope_type,r.scope_id,r.state,r.total_units,
	r.completed_units,r.failed_units,r.waiting_review_units,r.plan_id,COALESCE(r.plan_hash,''),
	r.idempotency_key,r.created_by,r.started_at,r.finished_at,r.cancelled_at,r.cancelled_by,
	r.created_at,r.updated_at,
	COALESCE((SELECT SUM(b.tokens_in) FROM gen_run_unit u JOIN gen_batch b ON b.id=u.batch_id
		WHERE u.team_id=r.team_id AND u.run_id=r.id),0),
	COALESCE((SELECT SUM(b.tokens_out) FROM gen_run_unit u JOIN gen_batch b ON b.id=u.batch_id
		WHERE u.team_id=r.team_id AND u.run_id=r.id),0)`

func (s *GenerationRunStore) Create(ctx context.Context, run *domain.GenerationRun) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO gen_run(id,team_id,project_id,scope_type,scope_id,state,total_units,completed_units,failed_units,waiting_review_units,plan_id,plan_hash,idempotency_key,created_by,started_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,0,0,0,$8,$9,$10,$11,$12)`,
			run.ID, run.TeamID, run.ProjectID, run.ScopeType, run.ScopeID, string(run.State), run.TotalUnits,
			run.PlanID, nullString(run.PlanHash), run.IdempotencyKey, run.CreatedBy, run.StartedAt); err != nil {
			return err
		}
		for _, unit := range run.Units {
			if _, err := tx.Exec(ctx, `
				INSERT INTO gen_run_unit(id,team_id,run_id,service_id,repo_id,source_path,ref_type,ref_name,commit_sha,batch_id,state,batch_state,stage)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULL,$10,$11,$12)`,
				unit.ID, run.TeamID, run.ID, unit.ServiceID, unit.RepoID, nullString(unit.SourcePath),
				nullString(unit.RefType), nullString(unit.RefName), nullString(unit.CommitSHA), string(unit.State), "", ""); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GenerationRunStore) Find(ctx context.Context, teamID, runID int64) (*domain.GenerationRun, error) {
	var out *domain.GenerationRun
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var err error
		out, err = findGenerationRun(ctx, tx, teamID, runID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := s.applyRunCosts(ctx, teamID, []*domain.GenerationRun{out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *GenerationRunStore) FindByIdempotencyKey(ctx context.Context, teamID int64, key string) (*domain.GenerationRun, error) {
	var out *domain.GenerationRun
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var runID int64
		if err := tx.QueryRow(ctx, `SELECT id FROM gen_run WHERE team_id=$1 AND idempotency_key=$2`, teamID, key).Scan(&runID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrGenerationRunNotFound
			}
			return err
		}
		var err error
		out, err = findGenerationRun(ctx, tx, teamID, runID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := s.applyRunCosts(ctx, teamID, []*domain.GenerationRun{out}); err != nil {
		return nil, err
	}
	return out, nil
}

func findGenerationRun(ctx context.Context, tx pgx.Tx, teamID, runID int64) (*domain.GenerationRun, error) {
	var run domain.GenerationRun
	var state string
	var planID *int64
	if err := tx.QueryRow(ctx, `SELECT `+runSelectColumns+`
		FROM gen_run r WHERE r.team_id=$1 AND r.id=$2`, teamID, runID).Scan(
		&run.ID, &run.TeamID, &run.ProjectID, &run.ScopeType, &run.ScopeID, &state, &run.TotalUnits,
		&run.CompletedUnits, &run.FailedUnits, &run.WaitingReviewUnits, &planID, &run.PlanHash,
		&run.IdempotencyKey, &run.CreatedBy, &run.StartedAt, &run.FinishedAt, &run.CancelledAt, &run.CancelledBy,
		&run.CreatedAt, &run.UpdatedAt, &run.TokensIn, &run.TokensOut); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrGenerationRunNotFound
		}
		return nil, err
	}
	run.State = domain.GenerationRunState(state)
	run.PlanID = planID
	units, err := loadGenerationUnits(ctx, tx, teamID, []int64{runID})
	if err != nil {
		return nil, err
	}
	run.Units = units[runID]
	return &run, nil
}

// applyRunCosts resolves generation cost through the trusted-domain read port.
// It batches every distinct batch id of the page into one call (no N+1) and
// never queries cost_line_item directly from AI infra.
func (s *GenerationRunStore) applyRunCosts(ctx context.Context, teamID int64, runs []*domain.GenerationRun) error {
	if s.cost == nil || len(runs) == 0 {
		return nil
	}
	batchIDs := make([]int64, 0)
	seen := make(map[int64]struct{}, 8)
	runBatches := make(map[int64][]int64, len(runs))
	for _, run := range runs {
		if run == nil {
			continue
		}
		ids := make([]int64, 0, len(run.Units))
		for _, unit := range run.Units {
			if unit == nil || unit.BatchID == nil {
				continue
			}
			batchID := *unit.BatchID
			ids = append(ids, batchID)
			if _, exists := seen[batchID]; exists {
				continue
			}
			seen[batchID] = struct{}{}
			batchIDs = append(batchIDs, batchID)
		}
		runBatches[run.ID] = ids
	}
	if len(batchIDs) == 0 {
		return nil
	}
	costs, err := s.cost.BatchCosts(ctx, teamID, batchIDs)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run == nil {
			continue
		}
		total := 0.0
		for _, batchID := range runBatches[run.ID] {
			total += costs[batchID]
		}
		run.CostAmount = total
	}
	return nil
}

const unitSelectColumns = `
	u.id,u.team_id,u.run_id,u.service_id,COALESCE(s.name,''),u.repo_id,COALESCE(rr.url,''),
	COALESCE(u.source_path,''),COALESCE(u.ref_type,''),COALESCE(u.ref_name,''),COALESCE(u.commit_sha,''),
	u.batch_id,COALESCE(u.batch_state,''),COALESCE(u.stage,''),u.review_required,u.state,
	COALESCE(u.error_summary,''),u.started_at,u.finished_at,u.last_synced_at,u.created_at,u.updated_at`

func loadGenerationUnits(ctx context.Context, tx pgx.Tx, teamID int64, runIDs []int64) (map[int64][]*domain.GenerationUnit, error) {
	out := make(map[int64][]*domain.GenerationUnit, len(runIDs))
	rows, err := tx.Query(ctx, `SELECT `+unitSelectColumns+`
		FROM gen_run_unit u
		LEFT JOIN tgt_service s ON s.id=u.service_id AND s.team_id=u.team_id
		LEFT JOIN repo_repo rr ON rr.id=u.repo_id AND rr.team_id=u.team_id
		WHERE u.team_id=$1 AND u.run_id = ANY($2) ORDER BY u.id`, teamID, runIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var unit domain.GenerationUnit
		var unitState string
		if err := rows.Scan(&unit.ID, &unit.TeamID, &unit.RunID, &unit.ServiceID, &unit.ServiceName,
			&unit.RepoID, &unit.RepoURL, &unit.SourcePath, &unit.RefType, &unit.RefName, &unit.CommitSHA,
			&unit.BatchID, &unit.BatchState, &unit.Stage, &unit.ReviewRequired, &unitState,
			&unit.Error, &unit.StartedAt, &unit.FinishedAt, &unit.LastSyncedAt, &unit.CreatedAt, &unit.UpdatedAt); err != nil {
			return nil, err
		}
		unit.State = domain.GenerationUnitState(unitState)
		out[unit.RunID] = append(out[unit.RunID], &unit)
	}
	return out, rows.Err()
}

// ResolveGenerationRunScope expands project/repo/service into eligible services.
func (s *GenerationRunStore) ResolveGenerationRunScope(ctx context.Context, teamID, projectID int64, scopeType string, scopeID int64) ([]domain.GenerationRunScope, error) {
	var out []domain.GenerationRunScope
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT s.id,s.repo_id,COALESCE(s.source_path,''),COALESCE(r.default_branch,'')
			FROM tgt_service s JOIN repo_repo r ON r.id=s.repo_id AND r.team_id=s.team_id
			WHERE s.team_id=$1 AND s.project_id=$2 AND s.status='active'
			  AND s.generation_status='confirmed' AND s.profile_status='confirmed'
			  AND (($3='project') OR ($3='repo' AND s.repo_id=$4) OR ($3='service' AND s.id=$4))
			ORDER BY s.id`, teamID, projectID, scopeType, scopeID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item domain.GenerationRunScope
			if err := rows.Scan(&item.ServiceID, &item.RepoID, &item.SourcePath, &item.DefaultBranch); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func (s *GenerationRunStore) ListByProject(ctx context.Context, teamID, projectID int64, limit int) ([]*domain.GenerationRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var out []*domain.GenerationRun
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+runSelectColumns+`
			FROM gen_run r WHERE r.team_id=$1 AND r.project_id=$2
			ORDER BY r.created_at DESC,r.id DESC LIMIT $3`, teamID, projectID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		ids := make([]int64, 0)
		for rows.Next() {
			var run domain.GenerationRun
			var state string
			if err := rows.Scan(&run.ID, &run.TeamID, &run.ProjectID, &run.ScopeType, &run.ScopeID, &state, &run.TotalUnits,
				&run.CompletedUnits, &run.FailedUnits, &run.WaitingReviewUnits, &run.PlanID, &run.PlanHash,
				&run.IdempotencyKey, &run.CreatedBy, &run.StartedAt, &run.FinishedAt, &run.CancelledAt, &run.CancelledBy,
				&run.CreatedAt, &run.UpdatedAt, &run.TokensIn, &run.TokensOut); err != nil {
				return err
			}
			run.State = domain.GenerationRunState(state)
			out = append(out, &run)
			ids = append(ids, run.ID)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		// pgx requires the result set to be closed before another query runs
		// on the same transaction/connection (otherwise: "conn busy").
		rows.Close()
		if len(ids) == 0 {
			return nil
		}
		units, err := loadGenerationUnits(ctx, tx, teamID, ids)
		if err != nil {
			return err
		}
		for _, run := range out {
			run.Units = units[run.ID]
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.applyRunCosts(ctx, teamID, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListProjectRunsPage pages runs by id DESC: a stable cursor avoids the
// duplicate/skip behavior of created_at pagination. It fetches limit+1 rows to
// report NextCursor only when an older page really exists.
func (s *GenerationRunStore) ListProjectRunsPage(ctx context.Context, teamID, projectID, beforeID int64, state string, limit int) (*domain.GenerationRunPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := &domain.GenerationRunPage{Runs: []*domain.GenerationRun{}}
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+runSelectColumns+`
			FROM gen_run r
			WHERE r.team_id=$1 AND r.project_id=$2
			  AND ($3::bigint = 0 OR r.id < $3::bigint)
			  AND ($4::text = '' OR r.state = $4::text)
			ORDER BY r.id DESC
			LIMIT $5`, teamID, projectID, beforeID, state, limit+1)
		if err != nil {
			return err
		}
		defer rows.Close()
		ids := make([]int64, 0, limit)
		hasMore := false
		for rows.Next() {
			if len(page.Runs) == limit {
				hasMore = true
				break
			}
			var run domain.GenerationRun
			var runState string
			if err := rows.Scan(&run.ID, &run.TeamID, &run.ProjectID, &run.ScopeType, &run.ScopeID, &runState, &run.TotalUnits,
				&run.CompletedUnits, &run.FailedUnits, &run.WaitingReviewUnits, &run.PlanID, &run.PlanHash,
				&run.IdempotencyKey, &run.CreatedBy, &run.StartedAt, &run.FinishedAt, &run.CancelledAt, &run.CancelledBy,
				&run.CreatedAt, &run.UpdatedAt, &run.TokensIn, &run.TokensOut); err != nil {
				return err
			}
			run.State = domain.GenerationRunState(runState)
			page.Runs = append(page.Runs, &run)
			ids = append(ids, run.ID)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		// pgx requires the result set to be closed before another query runs
		// on the same transaction/connection (otherwise: "conn busy").
		rows.Close()
		if len(ids) == 0 {
			return nil
		}
		units, err := loadGenerationUnits(ctx, tx, teamID, ids)
		if err != nil {
			return err
		}
		for _, run := range page.Runs {
			run.Units = units[run.ID]
		}
		if hasMore {
			cursor := page.Runs[len(page.Runs)-1].ID
			page.NextCursor = &cursor
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.applyRunCosts(ctx, teamID, page.Runs); err != nil {
		return nil, err
	}
	return page, nil
}

func (s *GenerationRunStore) LinkBatch(ctx context.Context, teamID, runID, unitID, batchID int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE gen_run_unit SET batch_id=$1,state='running',batch_state=$2,stage=$2,
			started_at=COALESCE(started_at,now()),last_synced_at=now(),updated_at=now()
			WHERE team_id=$3 AND run_id=$4 AND id=$5`, batchID, string(domain.GenerationEnqueued), teamID, runID, unitID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrGenerationRunNotFound
		}
		return nil
	})
}

func (s *GenerationRunStore) SetUnitState(ctx context.Context, teamID, runID, unitID int64, state domain.GenerationUnitState, errorSummary string) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE gen_run_unit SET state=$1,error_summary=$2,updated_at=now() WHERE team_id=$3 AND run_id=$4 AND id=$5`, string(state), errorSummary, teamID, runID, unitID)
		return err
	})
}

func (s *GenerationRunStore) UpdateState(ctx context.Context, teamID, runID int64, state domain.GenerationRunState, completed, failed int) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE gen_run SET state=$1,completed_units=$2,failed_units=$3,updated_at=now() WHERE team_id=$4 AND id=$5`, string(state), completed, failed, teamID, runID)
		return err
	})
}

func (s *GenerationRunStore) UpdateUnit(ctx context.Context, teamID, runID, unitID int64, update domain.GenerationRunUnitUpdate) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE gen_run_unit SET state=$1,batch_state=$2,stage=$3,review_required=$4,
			error_summary=$5,started_at=$6,finished_at=$7,last_synced_at=$8,updated_at=now()
			WHERE team_id=$9 AND run_id=$10 AND id=$11`,
			string(update.State), nullString(update.BatchState), nullString(update.Stage), update.ReviewRequired,
			nullString(update.Error), update.StartedAt, update.FinishedAt, update.SyncedAt, teamID, runID, unitID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrGenerationRunNotFound
		}
		return nil
	})
}

func (s *GenerationRunStore) UpdateProgress(ctx context.Context, teamID, runID int64, progress domain.GenerationRunProgress) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE gen_run SET state=$1,completed_units=$2,failed_units=$3,waiting_review_units=$4,
			started_at=COALESCE($5,started_at),finished_at=COALESCE($6,finished_at),
			cancelled_at=COALESCE($7,cancelled_at),cancelled_by=COALESCE($8,cancelled_by),updated_at=now()
			WHERE team_id=$9 AND id=$10`,
			string(progress.State), progress.Completed, progress.Failed, progress.WaitingReview,
			progress.StartedAt, progress.FinishedAt, progress.CancelledAt, progress.CancelledBy, teamID, runID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrGenerationRunNotFound
		}
		return nil
	})
}

// ListReconcileCandidates reads non-terminal runs through a bounded
// SECURITY DEFINER function: the worker has no request tenant and gen_run has
// RLS enabled, so a plain cross-tenant SELECT would return nothing.
func (s *GenerationRunStore) ListReconcileCandidates(ctx context.Context, limit int) ([]domain.GenerationReconcileCandidate, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Pool().Query(ctx, `SELECT run_id, team_id FROM gp.generation_reconcile_candidates($1::integer)`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GenerationReconcileCandidate, 0, limit)
	for rows.Next() {
		var item domain.GenerationReconcileCandidate
		if err := rows.Scan(&item.RunID, &item.TeamID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
