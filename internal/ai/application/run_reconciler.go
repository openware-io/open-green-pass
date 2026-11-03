package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

// GenerationRunReconciler converges run/unit projections from the authoritative
// batch state. It runs in the worker on a fixed tick because the process has no
// request tenant; each candidate is reconciled inside its own tenant context.
type GenerationRunReconciler struct {
	runs     domain.GenerationRunRepository
	service  *GenerationRunService
	interval time.Duration
	limit    int
	log      *slog.Logger
}

func NewGenerationRunReconciler(runs domain.GenerationRunRepository, service *GenerationRunService, interval time.Duration, log *slog.Logger) *GenerationRunReconciler {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if log == nil {
		log = slog.Default()
	}
	return &GenerationRunReconciler{runs: runs, service: service, interval: interval, limit: 100, log: log}
}

// ReconcileOnce processes one bounded batch of non-terminal runs and returns
// how many candidates were read. Per-run failures are logged and skipped so one
// poisoned run cannot stall the others.
func (r *GenerationRunReconciler) ReconcileOnce(ctx context.Context) (int, error) {
	if r == nil || r.runs == nil || r.service == nil {
		return 0, nil
	}
	candidates, err := r.runs.ListReconcileCandidates(ctx, r.limit)
	if err != nil {
		return 0, err
	}
	for _, candidate := range candidates {
		if candidate.TeamID <= 0 || candidate.RunID <= 0 {
			continue
		}
		runCtx := rls.WithTenant(ctx, candidate.TeamID)
		if _, err := r.service.SyncRun(runCtx, candidate.TeamID, candidate.RunID); err != nil {
			r.log.WarnContext(runCtx, "generation run reconcile failed",
				"team_id", candidate.TeamID, "run_id", candidate.RunID, "error", err)
		}
	}
	return len(candidates), nil
}

// Run blocks until ctx is cancelled, reconciling immediately and then on every
// interval tick.
func (r *GenerationRunReconciler) Run(ctx context.Context) {
	if r == nil {
		return
	}
	if r.interval <= 0 {
		r.interval = 5 * time.Second
	}
	r.reconcile(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

func (r *GenerationRunReconciler) reconcile(ctx context.Context) {
	processed, err := r.ReconcileOnce(ctx)
	if err != nil {
		r.log.WarnContext(ctx, "generation run reconcile scan failed", "error", err)
		return
	}
	if processed > 0 {
		r.log.InfoContext(ctx, "generation run reconcile pass", "candidates", processed)
	}
}
