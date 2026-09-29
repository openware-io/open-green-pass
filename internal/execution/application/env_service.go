// Package application 执行域应用服务：测试环境版本校验（登记运行版本 / 比对校验 / 校验历史）。
package application

import (
	"context"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// EnvService 版本校验应用服务实现。
type EnvService struct {
	repo domain.EnvRepository
	gen  *id.Generator
}

// NewEnvService 创建版本校验应用服务。
func NewEnvService(repo domain.EnvRepository, gen *id.Generator) *EnvService {
	return &EnvService{repo: repo, gen: gen}
}

// ErrTenantRequired 请求缺少租户（x-gp-team-id）。
var ErrTenantRequired = errTenantRequired()

func errTenantRequired() error { return &TenantErr{} }

// TenantErr 租户缺失错误。
type TenantErr struct{}

func (e *TenantErr) Error() string { return "tenant required: missing x-gp-team-id header" }

// RegisterRuntime 登记/更新被测对象在指定环境的运行版本（upsert）。
func (s *EnvService) RegisterRuntime(ctx context.Context, targetID int64, env, version string) (*domain.EnvRuntime, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	r := &domain.EnvRuntime{
		ID: s.gen.Next(), TeamID: teamID, TargetID: targetID,
		Env: env, RunningVersion: version, CheckedAt: time.Now().UTC(),
	}
	if err := s.repo.UpsertRuntime(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// CheckVersion 比对目标版本 vs 环境运行版本，写 env_check 并返回结果。
// mismatch 返回阻断信号；unknown（无运行版本）不阻断但留痕。
func (s *EnvService) CheckVersion(ctx context.Context, targetID int64, env, targetVersion string) (*domain.EnvCheck, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	rt, err := s.repo.FindRuntime(ctx, teamID, targetID, env)
	if err != nil {
		// 无运行版本 → unknown
		c := &domain.EnvCheck{
			ID: s.gen.Next(), TeamID: teamID, TargetID: targetID,
			TargetVersion: targetVersion, EnvVersion: "", Result: domain.EnvUnknown,
			CheckedAt: time.Now().UTC(),
		}
		_ = s.repo.SaveCheck(ctx, c)
		return c, nil
	}
	res := domain.EnvMatch
	if rt.RunningVersion != targetVersion {
		res = domain.EnvMismatch
	}
	c := &domain.EnvCheck{
		ID: s.gen.Next(), TeamID: teamID, TargetID: targetID,
		TargetVersion: targetVersion, EnvVersion: rt.RunningVersion,
		Result: res, CheckedAt: time.Now().UTC(),
	}
	if err := s.repo.SaveCheck(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// RecentChecks 查询被测对象最近校验记录。
func (s *EnvService) RecentChecks(ctx context.Context, targetID int64, limit int) ([]*domain.EnvCheck, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.RecentChecks(ctx, teamID, targetID, limit)
}
