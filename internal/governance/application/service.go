// Package application 治理域应用服务：被测对象树管理（建树/绑仓库/查询/模型绑定）。
// 实现依赖 infra 仓库；RLS 租户从 context 读取（见 gateway tenant 中间件）。
package application

import (
	"context"
	"fmt"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// TargetTreeService 被测对象树应用服务实现。
type TargetTreeService struct {
	repo        domain.TargetRepository
	gen         *id.Generator
	modelPolicy domain.ModelBindingPolicy
}

// NewTargetTreeService 创建被测对象树应用服务。
func NewTargetTreeService(repo domain.TargetRepository, gen *id.Generator) *TargetTreeService {
	return &TargetTreeService{repo: repo, gen: gen, modelPolicy: domain.NoopModelBindingPolicy{}}
}

// SetModelBindingPolicy injects the GP3 model approval/whitelist boundary.
func (s *TargetTreeService) SetModelBindingPolicy(policy domain.ModelBindingPolicy) {
	if policy != nil {
		s.modelPolicy = policy
	}
}

// CreateTarget 创建被测对象树节点（工程/服务组/服务/模块）。
func (s *TargetTreeService) CreateTarget(ctx context.Context, node domain.TargetNode, mb *domain.ModelBinding) (*domain.Target, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	if mb != nil {
		if err := mb.Validate(); err != nil {
			return nil, err
		}
	}
	node.TeamID = teamID
	if node.ID == 0 {
		node.ID = s.gen.Next()
	}
	if node.Status == "" {
		node.Status = "active"
	}
	if node.Kind == "" {
		node.Kind = "other"
	}
	tgt := &domain.Target{Node: node, ModelBinding: mb}
	if err := s.repo.SaveTarget(ctx, tgt); err != nil {
		return nil, err
	}
	return tgt, nil
}

// AttachRepo 绑定被测对象节点的仓库 + 分支/版本（来源溯源）。
func (s *TargetTreeService) AttachRepo(ctx context.Context, targetID int64, r domain.Repo, b domain.RepoBranch) error {
	if _, ok := rls.TenantFrom(ctx); !ok {
		return ErrTenantRequired
	}
	r.TargetID = targetID
	if r.ID == 0 {
		r.ID = s.gen.Next()
	}
	if r.Kind == "" {
		r.Kind = "git"
	}
	if err := s.repo.SaveRepo(ctx, &r); err != nil {
		return err
	}
	b.RepoID = r.ID
	if b.ID == 0 {
		b.ID = s.gen.Next()
	}
	if b.Branch == "" {
		b.Branch = r.DefaultBranch
	}
	return s.repo.SaveBranch(ctx, &b)
}

// ListTargets 列出被测对象树（层级/名称筛选）。
func (s *TargetTreeService) ListTargets(ctx context.Context, f domain.TargetFilter) ([]*domain.Target, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.ListTargets(ctx, teamID, f)
}

// FindTarget 查询单个被测对象（含绑定仓库/分支）。
func (s *TargetTreeService) FindTarget(ctx context.Context, id int64) (*domain.Target, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.FindTarget(ctx, teamID, id)
}

// SetModelBinding updates the provider-independent target model binding.
func (s *TargetTreeService) SetModelBinding(ctx context.Context, targetID int64, binding domain.ModelBinding) (*domain.Target, error) {
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	if err := s.modelPolicy.ValidateBinding(ctx, teamID, targetID, binding); err != nil {
		return nil, err
	}
	target, err := s.repo.FindTarget(ctx, teamID, targetID)
	if err != nil {
		return nil, err
	}
	target.SetModelBinding(binding)
	if err := s.repo.SaveTarget(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}

// ErrTenantRequired 请求缺少租户（x-gp-team-id）。
var ErrTenantRequired = fmt.Errorf("tenant required: missing x-gp-team-id header")
