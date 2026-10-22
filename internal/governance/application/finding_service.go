// Package application 治理域应用服务：缺陷创建/查询/状态流转。
package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// FindingAuditPort 跨域审计端口（Trusted 域实现，避免直接依赖）。
type FindingAuditPort interface {
	Append(ctx context.Context, actor int64, op, asset string, assetID int64, payload map[string]any) error
}

type NoopFindingAudit struct{}

func (NoopFindingAudit) Append(context.Context, int64, string, string, int64, map[string]any) error {
	return nil
}

// CreateFindingInput 创建缺陷输入（仓库/提交必填）。
type CreateFindingInput struct {
	ProjectID     int64
	ServiceID     *int64
	EndpointID    *int64
	TargetID      *int64
	RepositoryID  *int64
	RepoName      string
	RepoURL       string
	Branch        string
	CommitSHA     string
	Module        string
	Endpoint      string
	Title         string
	Description   string
	Severity      domain.Severity
	Category      string
	Expected      string
	Actual        string
	RootCause     string
	Impact        string
	Confidence    domain.Confidence
	Reproduction  map[string]any
	SourceRef     map[string]any
	Evidence      domain.FindingEvidence
	CoverageCells []string
	OwnerID       *int64
	LinkedRunID   *int64
	LinkedCaseID  *int64
}

// FindingService 缺陷应用服务。
type FindingService struct {
	repo  domain.FindingRepository
	gen   *id.Generator
	audit FindingAuditPort
}

// NewFindingService 创建缺陷应用服务。
func NewFindingService(repo domain.FindingRepository, gen *id.Generator) *FindingService {
	return &FindingService{repo: repo, gen: gen, audit: NoopFindingAudit{}}
}

// SetAuditPort 注入 Trusted 审计端口。
func (s *FindingService) SetAuditPort(port FindingAuditPort) {
	if port != nil {
		s.audit = port
	}
}

// Create 创建缺陷；命中 dedupe 时返回已有缺陷（created=false）。
func (s *FindingService) Create(ctx context.Context, in CreateFindingInput) (*domain.Finding, bool, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, false, gperr.Unauthorized("tenant required: missing x-gp-team-id header")
	}
	actor, _ := rls.UserFrom(ctx)
	now := time.Now().UTC()
	category := in.Category
	if category == "" {
		category = "product"
	}
	confidence := in.Confidence
	if confidence == "" {
		confidence = domain.ConfidenceMedium
	}
	f := &domain.Finding{
		ID: s.gen.Next(), TeamID: teamID, ProjectID: in.ProjectID,
		ServiceID: in.ServiceID, EndpointID: in.EndpointID, TargetID: in.TargetID,
		Repository: domain.FindingRepositoryRef{
			ID: in.RepositoryID, Name: in.RepoName, URL: in.RepoURL,
			Branch: in.Branch, CommitSHA: in.CommitSHA,
		},
		Module: in.Module, Endpoint: in.Endpoint, Title: in.Title, Description: in.Description,
		Severity: in.Severity, Category: category, Status: domain.FindingCandidate,
		Expected: in.Expected, Actual: in.Actual, RootCause: in.RootCause, Impact: in.Impact,
		Confidence: confidence, Reproduction: in.Reproduction, SourceRef: in.SourceRef,
		Evidence: in.Evidence, CoverageCells: in.CoverageCells, OwnerID: in.OwnerID,
		LinkedRunID: in.LinkedRunID, LinkedCaseID: in.LinkedCaseID,
		DedupeHash: domain.ComputeFindingDedupeHash(in.RepoURL, in.Endpoint, category,
			in.RootCause, in.Expected, in.Actual),
		CreatedBy: actor, CreatedAt: now, UpdatedAt: now,
	}
	if err := f.Validate(); err != nil {
		return nil, false, gperr.Validation(err.Error())
	}
	if f.Category == "" {
		f.Category = "product"
	}
	if f.Confidence == "" {
		f.Confidence = domain.ConfidenceMedium
	}
	event := &domain.FindingEvent{
		ID: s.gen.Next(), Action: "created", ToStatus: string(domain.FindingCandidate),
		ActorID: actor, CreatedAt: now,
	}
	created, err := s.repo.Create(ctx, f, event)
	if err != nil {
		return nil, false, err
	}
	if !created {
		existing, findErr := s.repo.FindByDedupe(ctx, teamID, f.DedupeHash)
		if findErr != nil {
			return nil, false, findErr
		}
		return existing, false, nil
	}
	s.auditCreate(ctx, actor, f)
	return f, true, nil
}

// Get 查询缺陷详情（含流转记录）。
func (s *FindingService) Get(ctx context.Context, id int64) (*domain.Finding, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	f, err := s.repo.Find(ctx, teamID, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, gperr.NotFound("finding not found")
		}
		return nil, err
	}
	events, err := s.repo.Events(ctx, teamID, id)
	if err != nil {
		return nil, err
	}
	f.Events = events
	return f, nil
}

// List 查询缺陷列表。
func (s *FindingService) List(ctx context.Context, filter domain.FindingFilter) ([]*domain.Finding, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	return s.repo.List(ctx, teamID, filter)
}

// Transition 执行状态流转。
func (s *FindingService) Transition(ctx context.Context, id int64, action domain.FindingAction, reason string) (*domain.Finding, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	actor, _ := rls.UserFrom(ctx)
	f, err := s.repo.Find(ctx, teamID, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, gperr.NotFound("finding not found")
		}
		return nil, err
	}
	from, to, err := f.Transition(action, actor, reason)
	if err != nil {
		return nil, gperr.Conflict(err.Error())
	}
	event := &domain.FindingEvent{
		ID: s.gen.Next(), Action: string(action), FromStatus: string(from), ToStatus: string(to),
		ActorID: actor, Reason: reason, CreatedAt: f.UpdatedAt,
	}
	if err := s.repo.UpdateStatus(ctx, f, event); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, gperr.NotFound("finding not found")
		}
		return nil, err
	}
	s.auditTransition(ctx, actor, f, from, to, reason)
	return s.Get(ctx, id)
}

func (s *FindingService) auditCreate(ctx context.Context, actor int64, f *domain.Finding) {
	if err := s.audit.Append(ctx, actor, "finding.created", "finding", f.ID, map[string]any{
		"severity": string(f.Severity), "status": string(f.Status), "repo": f.Repository.URL,
		"commit": f.Repository.CommitSHA, "dedupe_hash": f.DedupeHash,
	}); err != nil {
		slog.Error("finding audit append failed", "finding_id", f.ID, "error", err)
	}
}

func (s *FindingService) auditTransition(ctx context.Context, actor int64, f *domain.Finding, from, to domain.FindingStatus, reason string) {
	if err := s.audit.Append(ctx, actor, "finding.transition", "finding", f.ID, map[string]any{
		"from": string(from), "to": string(to), "reason": reason,
	}); err != nil {
		slog.Error("finding audit append failed", "finding_id", f.ID, "error", err)
	}
}
