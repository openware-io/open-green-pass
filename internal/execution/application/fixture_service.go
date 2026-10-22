// Package application fixture/环境租约应用服务。
package application

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// FixtureService fixture/lease 服务。
type FixtureService struct {
	repo domain.FixtureRepository
	gen  *id.Generator
}

// NewFixtureService 创建服务。
func NewFixtureService(repo domain.FixtureRepository, gen *id.Generator) *FixtureService {
	return &FixtureService{repo: repo, gen: gen}
}

// Prepare 创建/重置 run fixture 命名空间。
func (s *FixtureService) Prepare(ctx context.Context, runID int64, namespace string, manifest map[string]any, ttl time.Duration) (*domain.RunFixture, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	if runID <= 0 {
		return nil, gperr.Validation("run_id required")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = "gp-run-" + strconv.FormatInt(runID, 10)
	}
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	expires := time.Now().UTC().Add(ttl)
	f := &domain.RunFixture{
		ID: s.gen.Next(), TeamID: teamID, RunID: runID, Namespace: namespace,
		Manifest: manifest, State: "ready", ExpiresAt: &expires, CleanupState: "pending",
	}
	if err := s.repo.Save(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// List 列出 run fixtures。
func (s *FixtureService) List(ctx context.Context, runID int64) ([]*domain.RunFixture, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	return s.repo.List(ctx, teamID, runID)
}

// Cleanup 清理 run fixtures。
func (s *FixtureService) Cleanup(ctx context.Context, runID int64, namespace string) error {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return gperr.Unauthorized("tenant required")
	}
	if err := s.repo.Cleanup(ctx, teamID, runID, namespace); err != nil {
		slog.Error("fixture cleanup failed", "run_id", runID, "namespace", namespace, "error", err)
		if errors.Is(err, pgx.ErrNoRows) {
			return gperr.NotFound("fixture not found")
		}
		return err
	}
	return nil
}

// Lease 获取环境租约。
func (s *FixtureService) Lease(ctx context.Context, envID, runID int64, owner string, ttl time.Duration) (*domain.EnvLease, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	if envID <= 0 || runID <= 0 {
		return nil, gperr.Validation("env_id and run_id required")
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	lease := &domain.EnvLease{
		ID: s.gen.Next(), TeamID: teamID, EnvID: envID, RunID: runID,
		Owner: owner, LeaseUntil: time.Now().UTC().Add(ttl),
	}
	if err := s.repo.Lease(ctx, lease); err != nil {
		return nil, err
	}
	return lease, nil
}

// Release 释放环境租约。
func (s *FixtureService) Release(ctx context.Context, envID int64) error {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return gperr.Unauthorized("tenant required")
	}
	if err := s.repo.Release(ctx, teamID, envID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gperr.NotFound("lease not found")
		}
		return err
	}
	return nil
}
