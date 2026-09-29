// Package application 治理域应用服务：测试用例版本化管理（新增/更新/删除/回退/历史）。
package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// CaseService 用例版本化应用服务实现。
type CaseService struct {
	repo domain.CaseRepository
	gen  *id.Generator
}

// NewCaseService 创建用例版本化应用服务。
func NewCaseService(repo domain.CaseRepository, gen *id.Generator) *CaseService {
	return &CaseService{repo: repo, gen: gen}
}

// CreateCase 新建用例（初始版本 v1，change=added）。
func (s *CaseService) CreateCase(ctx context.Context, c domain.Case, script json.RawMessage) (*domain.Case, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	userID, _ := rls.UserFrom(ctx)
	c.TeamID = teamID
	c.CreatedBy = userID
	c.ID = s.gen.Next()
	c.CurrentVersion = 1
	if c.Status == "" {
		c.Status = "active"
	}
	if c.Kind == "" {
		c.Kind = "api"
	}
	v := &domain.CaseVersion{
		ID: s.gen.Next(), CaseID: c.ID, Version: 1, TeamID: teamID,
		ChangeType: domain.ChangeAdded, ScriptJSON: &script, CreatedBy: userID,
	}
	if err := s.repo.SaveCase(ctx, &c); err != nil {
		return nil, err
	}
	if err := s.repo.SaveVersion(ctx, v); err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateVersion 为用例新增版本（内容更新，change=updated；版本号=历史最大+1）。
func (s *CaseService) CreateVersion(ctx context.Context, caseID int64, script json.RawMessage, sourceRepo *int64, sourceBranch *string) (*domain.Case, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	c, err := s.repo.FindCase(ctx, teamID, caseID)
	if err != nil {
		return nil, err
	}
	hist, err := s.repo.History(ctx, teamID, caseID)
	if err != nil {
		return nil, err
	}
	newVer := maxVersion(hist) + 1
	c.CurrentVersion = newVer
	v := &domain.CaseVersion{
		ID: s.gen.Next(), CaseID: caseID, Version: newVer, TeamID: teamID,
		ChangeType: domain.ChangeUpdated, ScriptJSON: &script,
		SourceRepoID: sourceRepo, SourceBranch: sourceBranch,
	}
	if err := s.repo.SaveCase(ctx, c); err != nil {
		return nil, err
	}
	if err := s.repo.SaveVersion(ctx, v); err != nil {
		return nil, err
	}
	return c, nil
}

// MarkDeleted 标记用例删除（change=deleted；保留历史，status=deleted）。
func (s *CaseService) MarkDeleted(ctx context.Context, caseID int64) error {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return ErrTenantRequired
	}
	c, err := s.repo.FindCase(ctx, teamID, caseID)
	if err != nil {
		return err
	}
	hist, err := s.repo.History(ctx, teamID, caseID)
	if err != nil {
		return err
	}
	newVer := maxVersion(hist) + 1
	c.Status = "deleted"
	v := &domain.CaseVersion{ID: s.gen.Next(), CaseID: caseID, Version: newVer, TeamID: teamID, ChangeType: domain.ChangeDeleted}
	if err := s.repo.SaveCase(ctx, c); err != nil {
		return err
	}
	return s.repo.SaveVersion(ctx, v)
}

// RollbackTo 回退用例到指定版本（current_version=v；新增一条指向旧版本内容的新版本，change=rollback，非物理删）。
func (s *CaseService) RollbackTo(ctx context.Context, caseID int64, targetVersion int) (*domain.Case, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	c, err := s.repo.FindCase(ctx, teamID, caseID)
	if err != nil {
		return nil, err
	}
	hist, err := s.repo.History(ctx, teamID, caseID)
	if err != nil {
		return nil, err
	}
	// 校验目标版本存在
	var old *domain.CaseVersion
	for i := range hist {
		if hist[i].Version == targetVersion {
			old = hist[i]
			break
		}
	}
	if old == nil {
		return nil, fmt.Errorf("case version %d not found", targetVersion)
	}
	newVer := maxVersion(hist) + 1
	c.CurrentVersion = targetVersion
	v := &domain.CaseVersion{
		ID: s.gen.Next(), CaseID: caseID, Version: newVer, TeamID: teamID,
		ChangeType: domain.ChangeRollback, ScriptJSON: old.ScriptJSON,
	}
	if err := s.repo.SaveCase(ctx, c); err != nil {
		return nil, err
	}
	if err := s.repo.SaveVersion(ctx, v); err != nil {
		return nil, err
	}
	return c, nil
}

// ListCases 列出用例（被测对象/场景族/编号/状态筛选）。
func (s *CaseService) ListCases(ctx context.Context, f domain.CaseFilter) ([]*domain.Case, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.ListCases(ctx, teamID, f)
}

// FindCase 查询单个用例。
func (s *CaseService) FindCase(ctx context.Context, id int64) (*domain.Case, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.FindCase(ctx, teamID, id)
}

// History 查询用例版本历史（只增不改，可对比）。
func (s *CaseService) History(ctx context.Context, caseID int64) ([]*domain.CaseVersion, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.History(ctx, teamID, caseID)
}

// maxVersion 计算版本历史的最大版本号（不存在时为 0）。
func maxVersion(hist []*domain.CaseVersion) int {
	m := 0
	for _, v := range hist {
		if v.Version > m {
			m = v.Version
		}
	}
	return m
}
