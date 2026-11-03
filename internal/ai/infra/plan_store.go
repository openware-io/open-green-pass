package infra

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/platform/metrics"
	"github.com/openware-io/open-green-pass/internal/platform/observability"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

// GenerationPlanStore persists preflight plans. Plans are write-once snapshots:
// only the lifecycle transition to consumed/expired updates a row.
type GenerationPlanStore struct {
	db      *db.DB
	metrics observability.MetricRecorder
}

// NewGenerationPlanStore creates the store.
func NewGenerationPlanStore(database *db.DB) *GenerationPlanStore {
	return &GenerationPlanStore{db: database}
}

// SetMetricsRecorder injects the process metrics recorder used by the plan GC.
// It is optional (nil disables metric emission) and intentionally carries no
// tenant labels.
func (s *GenerationPlanStore) SetMetricsRecorder(recorder observability.MetricRecorder) {
	s.metrics = recorder
}

var _ domain.GenerationPlanRepository = (*GenerationPlanStore)(nil)

func (s *GenerationPlanStore) Create(ctx context.Context, plan *domain.GenerationPlan) error {
	request, err := json.Marshal(plan.Request)
	if err != nil {
		return err
	}
	summary, err := json.Marshal(plan.Summary)
	if err != nil {
		return err
	}
	items, err := json.Marshal(plan.Items)
	if err != nil {
		return err
	}
	estimate, err := json.Marshal(plan.Estimate)
	if err != nil {
		return err
	}
	risks, err := json.Marshal(plan.Risks)
	if err != nil {
		return err
	}
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO gen_plan(id,team_id,project_id,scope_type,scope_id,plan_hash,state,request,summary,items,estimate,risks,expires_at,created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			plan.ID, plan.TeamID, plan.ProjectID, plan.ScopeType, plan.ScopeID, plan.PlanHash, string(plan.State),
			request, summary, items, estimate, risks, plan.ExpiresAt, plan.CreatedBy)
		return err
	})
}

func (s *GenerationPlanStore) Find(ctx context.Context, teamID, planID int64) (*domain.GenerationPlan, error) {
	var out *domain.GenerationPlan
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var plan domain.GenerationPlan
		var state string
		var request, summary, items, estimate, risks []byte
		if err := tx.QueryRow(ctx, `
			SELECT id,team_id,project_id,scope_type,scope_id,plan_hash,state,request,summary,items,estimate,risks,
				expires_at,confirmed_run_id,created_by,created_at,updated_at
			FROM gen_plan WHERE team_id=$1 AND id=$2`, teamID, planID).Scan(
			&plan.ID, &plan.TeamID, &plan.ProjectID, &plan.ScopeType, &plan.ScopeID, &plan.PlanHash, &state,
			&request, &summary, &items, &estimate, &risks, &plan.ExpiresAt, &plan.ConfirmedRunID,
			&plan.CreatedBy, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrGenerationPlanNotFound
			}
			return err
		}
		plan.State = domain.GenerationPlanState(state)
		if err := decodePlanJSON(&plan, request, summary, items, estimate, risks); err != nil {
			return err
		}
		out = &plan
		return nil
	})
	return out, err
}

func (s *GenerationPlanStore) FindIdempotent(ctx context.Context, teamID int64, planHash string, now time.Time) (*domain.GenerationPlan, error) {
	var out *domain.GenerationPlan
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var planID int64
		if err := tx.QueryRow(ctx, `
			SELECT id FROM gen_plan
			WHERE team_id=$1 AND plan_hash=$2 AND state='draft' AND expires_at > $3
			ORDER BY created_at DESC,id DESC LIMIT 1`, teamID, planHash, now).Scan(&planID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrGenerationPlanNotFound
			}
			return err
		}
		var plan domain.GenerationPlan
		var state string
		var request, summary, items, estimate, risks []byte
		if err := tx.QueryRow(ctx, `
			SELECT id,team_id,project_id,scope_type,scope_id,plan_hash,state,request,summary,items,estimate,risks,
				expires_at,confirmed_run_id,created_by,created_at,updated_at
			FROM gen_plan WHERE team_id=$1 AND id=$2`, teamID, planID).Scan(
			&plan.ID, &plan.TeamID, &plan.ProjectID, &plan.ScopeType, &plan.ScopeID, &plan.PlanHash, &state,
			&request, &summary, &items, &estimate, &risks, &plan.ExpiresAt, &plan.ConfirmedRunID,
			&plan.CreatedBy, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
			return err
		}
		plan.State = domain.GenerationPlanState(state)
		if err := decodePlanJSON(&plan, request, summary, items, estimate, risks); err != nil {
			return err
		}
		out = &plan
		return nil
	})
	return out, err
}

// Consume is the single-use CAS. The expiry predicate is evaluated in the same
// statement as the state transition so two different idempotency keys cannot
// both pass a prior Find and create two runs from one plan.
func (s *GenerationPlanStore) Consume(ctx context.Context, teamID, planID, runID int64, now time.Time) (bool, error) {
	var consumed bool
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE gen_plan SET state='consumed',confirmed_run_id=$3,updated_at=now()
			WHERE team_id=$1 AND id=$2 AND state='draft' AND expires_at > $4`, teamID, planID, runID, now)
		if err != nil {
			return err
		}
		consumed = tag.RowsAffected() > 0
		return nil
	})
	return consumed, err
}

func (s *GenerationPlanStore) ExpireBefore(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var expired int64
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE gen_plan SET state='expired',updated_at=now()
			WHERE state='draft' AND expires_at <= $1
			  AND id IN (SELECT id FROM gen_plan WHERE state='draft' AND expires_at <= $1 ORDER BY expires_at LIMIT $2)`,
			now, limit)
		if err != nil {
			return err
		}
		expired = tag.RowsAffected()
		return nil
	})
	if err == nil && expired > 0 && s.metrics != nil {
		// Lazy Preflight cleanup (per-tenant, no worker sweep) is still visible
		// in the same plan-GC expiry counter.
		s.metrics.Incr(metrics.GenerationPlanGCExpired, expired)
	}
	return expired, err
}

// TenantIDs lists tenant ids for the cross-tenant plan GC. tnt_team is the
// platform tenant registry (not tenant-scoped data), so the pool query is safe.
func (s *GenerationPlanStore) TenantIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.Pool().Query(ctx, `SELECT id FROM tnt_team ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0, 8)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GenerationPlanGCSweep summarizes one cross-tenant expiry sweep. It is
// exported so schedulers/tests can assert visibility without scraping logs.
type GenerationPlanGCSweep struct {
	Tenants       int
	Expired       int64
	FailedTenants int
	Duration      time.Duration
}

// SweepOnce expires at most 200 elapsed draft plans per tenant and returns a
// structured summary. Per-tenant failures are counted and logged with duration
// and cause; a tenant-list failure leaves Tenants=0 and logs stage=list_tenants.
func (s *GenerationPlanStore) SweepOnce(ctx context.Context, log *slog.Logger) GenerationPlanGCSweep {
	if log == nil {
		log = slog.Default()
	}
	start := time.Now()
	stats := GenerationPlanGCSweep{}
	ids, err := s.TenantIDs(ctx)
	if err != nil {
		stats.Duration = time.Since(start)
		s.recordGCMetrics(stats, true)
		if ctx.Err() == nil {
			log.Warn("generation plan gc sweep failed",
				"stage", "list_tenants", "duration_ms", stats.Duration.Milliseconds(), "err", err)
		}
		return stats
	}
	stats.Tenants = len(ids)
	now := time.Now().UTC()
	for _, teamID := range ids {
		tenantStart := time.Now()
		expired, err := s.ExpireBefore(rls.WithTenant(ctx, teamID), now, 200)
		if err != nil {
			if ctx.Err() == nil {
				stats.FailedTenants++
				log.Warn("generation plan gc tenant sweep failed",
					"team_id", teamID, "duration_ms", time.Since(tenantStart).Milliseconds(), "err", err)
			}
			continue
		}
		stats.Expired += expired
		if expired > 0 {
			log.Info("generation plan gc tenant expired",
				"team_id", teamID, "expired", expired,
				"duration_ms", time.Since(tenantStart).Milliseconds())
		}
	}
	stats.Duration = time.Since(start)
	s.recordGCMetrics(stats, false)
	log.Info("generation plan gc sweep",
		"tenants", stats.Tenants, "expired", stats.Expired,
		"failed_tenants", stats.FailedTenants, "duration_ms", stats.Duration.Milliseconds())
	return stats
}

// recordGCMetrics emits the low-cardinality GC contract. Labels are deliberately
// omitted: tenant ids must never become metric series. listFailed folds a
// tenant-registry failure into the failed counter without inventing a tenant.
func (s *GenerationPlanStore) recordGCMetrics(stats GenerationPlanGCSweep, listFailed bool) {
	if s.metrics == nil {
		return
	}
	s.metrics.Incr(metrics.GenerationPlanGCSweeps, 1)
	s.metrics.Incr(metrics.GenerationPlanGCExpired, stats.Expired)
	failed := int64(stats.FailedTenants)
	if listFailed {
		failed++
	}
	s.metrics.Incr(metrics.GenerationPlanGCFailed, failed)
	s.metrics.Observe(metrics.GenerationPlanGCDuration, stats.Duration.Seconds())
}

// RunGC expires draft plans whose TTL elapsed. gen_plan has FORCE RLS, so the
// sweep runs per tenant with an injected tenant scope instead of bypassing RLS.
// Each sweep emits the aggregated counters above: tenants, expired, failures and
// duration are visible in worker logs.
func (s *GenerationPlanStore) RunGC(ctx context.Context, interval time.Duration, log *slog.Logger) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	s.SweepOnce(ctx, log)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.SweepOnce(ctx, log)
		}
	}
}

func decodePlanJSON(plan *domain.GenerationPlan, request, summary, items, estimate, risks []byte) error {
	if len(request) > 0 {
		if err := json.Unmarshal(request, &plan.Request); err != nil {
			return err
		}
	}
	if len(summary) > 0 {
		if err := json.Unmarshal(summary, &plan.Summary); err != nil {
			return err
		}
	}
	if len(items) > 0 {
		if err := json.Unmarshal(items, &plan.Items); err != nil {
			return err
		}
	}
	if len(estimate) > 0 {
		if err := json.Unmarshal(estimate, &plan.Estimate); err != nil {
			return err
		}
	}
	if len(risks) > 0 {
		if err := json.Unmarshal(risks, &plan.Risks); err != nil {
			return err
		}
	}
	return nil
}
