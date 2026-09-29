// 执行域服务级截图开关策略应用服务（PRD R-TEST-13）。
package application

import (
	"context"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// PolicyService 截图策略服务（按 被测对象 × 场景 下发展开开关）。
type PolicyService struct {
	repo domain.PolicyRepository
	gen  *id.Generator
}

// NewPolicyService 创建截图策略服务。
func NewPolicyService(repo domain.PolicyRepository, gen *id.Generator) *PolicyService {
	return &PolicyService{repo: repo, gen: gen}
}

// SetPolicyRequest 下发策略请求。
type SetPolicyRequest struct {
	TargetID          int64 `json:"target_id"`
	ScenarioID        int64 `json:"scenario_id"`
	ScreenshotEnabled bool  `json:"screenshot_enabled"`
}

// Set 下发展开策略（UPSERT）。
func (s *PolicyService) Set(ctx context.Context, req SetPolicyRequest) (*domain.ScreenshotPolicy, error) {
	if _, ok := rls.TenantFrom(ctx); !ok {
		return nil, ErrTenantRequired
	}
	if req.TargetID == 0 {
		return nil, ErrTenantRequired
	}
	scenario := req.ScenarioID
	if scenario == 0 {
		scenario = 1
	}
	return s.repo.Set(ctx, &domain.ScreenshotPolicy{
		TargetID: req.TargetID, ScenarioID: scenario, ScreenshotEnabled: req.ScreenshotEnabled,
	})
}

// Get 读取展开策略（无行默认开启截图）。
func (s *PolicyService) Get(ctx context.Context, teamID, targetID, scenarioID int64) (*domain.ScreenshotPolicy, error) {
	if scenarioID == 0 {
		scenarioID = 1
	}
	return s.repo.Get(ctx, teamID, targetID, scenarioID)
}
