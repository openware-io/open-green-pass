// Package application 执行域应用服务：测试环境版本校验（登记运行版本 / 比对校验 / 校验历史）。
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// EnvService 版本校验应用服务实现。
type EnvService struct {
	repo domain.EnvRepository
	gen  *id.Generator
}

// ExecutionBaseURL returns the operator-configured origin for an environment.
// It deliberately fails closed: a generated declarative request must never
// fall back to a fake/smoke command when no destination was registered.
func (s *EnvService) ExecutionBaseURL(ctx context.Context, targetID int64, env string) (string, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return "", ErrTenantRequired
	}
	runtime, err := s.repo.FindRuntime(ctx, teamID, targetID, env)
	if err != nil {
		return "", fmt.Errorf("environment runtime is not registered for target %d environment %q", targetID, env)
	}
	if strings.TrimSpace(runtime.BaseURL) == "" {
		return "", fmt.Errorf("environment base_url is not configured for target %d environment %q", targetID, env)
	}
	return domain.ValidateRuntimeBaseURL(runtime.BaseURL)
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

// GetRuntime 查询已登记的环境运行版本。
func (s *EnvService) GetRuntime(ctx context.Context, targetID int64, env string) (*domain.EnvRuntime, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	runtime, err := s.repo.FindRuntime(ctx, teamID, targetID, env)
	if err != nil {
		var missing *domain.ErrNoEnvRuntime
		if errors.As(err, &missing) {
			return nil, gperr.NotFound("env runtime not found")
		}
		return nil, err
	}
	return runtime, nil
}

// RegisterRuntime 登记/更新被测对象在指定环境的运行版本（upsert）。
func (s *EnvService) RegisterRuntime(ctx context.Context, targetID int64, env, version string) (*domain.EnvRuntime, error) {
	return s.RegisterRuntimeWithBaseURL(ctx, targetID, env, version, "")
}

// RegisterRuntimeWithBaseURL registers both the version evidence and the
// trusted environment origin used by declarative API test cases.
func (s *EnvService) RegisterRuntimeWithBaseURL(ctx context.Context, targetID int64, env, version, baseURL string) (*domain.EnvRuntime, error) {
	return s.RegisterRuntimeBaseline(ctx, targetID, env, version, baseURL, nil, domain.EnvironmentConfirmation{})
}

// RegisterRuntimeBaseline persists a user-confirmed, non-sensitive execution
// baseline. Requirements can be suggested by analysis, but the confirmation is
// an explicit operator action. CredentialRef is a reference only; secrets are
// rejected here so they cannot leak into the application database or audit.
func (s *EnvService) RegisterRuntimeBaseline(ctx context.Context, targetID int64, env, version, baseURL string, requirements []domain.EnvironmentRequirement, confirmation domain.EnvironmentConfirmation) (*domain.EnvRuntime, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	if err := validateEnvironmentConfirmation(requirements, confirmation); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	r := &domain.EnvRuntime{
		ID: s.gen.Next(), TeamID: teamID, TargetID: targetID,
		Env: env, RunningVersion: version, BaseURL: baseURL, CheckedAt: now,
		Requirements: requirements, Confirmation: confirmation,
	}
	if confirmation.Confirmed {
		r.ConfirmedAt = &now
	}
	if err := s.repo.UpsertRuntime(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func validateEnvironmentConfirmation(requirements []domain.EnvironmentRequirement, confirmation domain.EnvironmentConfirmation) error {
	if !confirmation.Confirmed {
		return fmt.Errorf("environment requirements must be explicitly confirmed")
	}
	if len(confirmation.CredentialRef) > 160 || strings.ContainsAny(confirmation.CredentialRef, "\r\n") {
		return fmt.Errorf("credential reference is invalid")
	}
	for key, value := range confirmation.Values {
		if strings.TrimSpace(key) == "" || len(key) > 80 || len(value) > 500 || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("environment confirmation value is invalid")
		}
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "cookie") {
			return fmt.Errorf("environment confirmation accepts credential references, not secret values")
		}
	}
	for _, requirement := range requirements {
		if strings.TrimSpace(requirement.Key) == "" || len(requirement.Key) > 80 || len(requirement.Label) > 120 || len(requirement.Reason) > 500 || len(requirement.Evidence) > 500 {
			return fmt.Errorf("environment requirement is invalid")
		}
		if requirement.Required && !requirement.Sensitive && strings.TrimSpace(confirmation.Values[requirement.Key]) == "" {
			return fmt.Errorf("required environment field %q is not confirmed", requirement.Label)
		}
		if requirement.Required && requirement.Sensitive && strings.TrimSpace(confirmation.CredentialRef) == "" {
			return fmt.Errorf("required credential reference is not confirmed")
		}
	}
	return nil
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
