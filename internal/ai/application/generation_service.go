package application

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// CreateGenerationBatchCommand contains only source identifiers and no source
// content. The source is frozen later by the FetchSource activity.
type CreateGenerationBatchCommand struct {
	ID               int64
	TeamID           int64
	TargetID         int64
	RepoID           int64
	ServiceID        *int64
	SourcePath       string
	RefType          string
	Branch           string
	RequestedVersion string
	IdempotencyKey   string
	ActorID          int64
}

// GenerationBatchService owns idempotent batch creation and optimistic state
// transitions. It intentionally has no dependency on Temporal, SCM, AI, or
// governance case storage.
type GenerationBatchService struct {
	batches  domain.GenerationBatchRepository
	signals  GenerationSignalPort
	audit    GenerationAuditPort
	workflow GenerationWorkflowPort
	scope    GenerationScopePort
	now      func() time.Time
}

type GenerationReviewDecision string

const (
	GenerationReviewApprove GenerationReviewDecision = "approve"
	GenerationReviewReject  GenerationReviewDecision = "reject"
)

type ReviewGenerationCommand struct {
	TeamID, BatchID, ActorID, ExpectedVersion int64
	Decision                                  GenerationReviewDecision
	RequestID                                 string
}

type RollbackGenerationCommand struct {
	TeamID, BatchID, ActorID, ExpectedVersion int64
	RequestID                                 string
}

type GenerationSignal struct {
	TeamID, BatchID, ActorID, StateVersion int64
	Decision                               GenerationReviewDecision
	RequestID                              string
}

type GenerationSignalPort interface {
	SignalReview(context.Context, GenerationSignal) error
	SignalRollback(context.Context, GenerationSignal) error
	Cancel(context.Context, GenerationSignal) error
}

// GenerationWorkflowPort starts the durable non-human generation pipeline.
// Keeping it behind an application port prevents HTTP handlers from depending
// directly on Temporal.
type GenerationWorkflowPort interface {
	Start(context.Context, int64, int64) error
}

type GenerationScopePort interface {
	ReadGenerationScope(context.Context, int64, int64, int64) (*domain.GenerationScope, error)
}

type GenerationAuditEvent struct {
	TeamID, BatchID, ActorID, StateVersion int64
	Action, RequestID                      string
	OccurredAt                             time.Time
}

type GenerationAuditPort interface {
	AppendGenerationAudit(context.Context, GenerationAuditEvent) error
}

func NewGenerationBatchService(batches domain.GenerationBatchRepository) *GenerationBatchService {
	return &GenerationBatchService{batches: batches, now: func() time.Time { return time.Now().UTC() }}
}

func (s *GenerationBatchService) Find(ctx context.Context, teamID, batchID int64) (*domain.GenerationBatch, error) {
	if s == nil || s.batches == nil {
		return nil, errors.New("generation batch repository is required")
	}
	return s.batches.Find(ctx, teamID, batchID)
}

const DefaultGenerationBatchLimit = 50
const MaxGenerationBatchLimit = 100

func (s *GenerationBatchService) ListByTarget(ctx context.Context, teamID, targetID int64, limit int) ([]*domain.GenerationBatch, error) {
	if s == nil || s.batches == nil {
		return nil, errors.New("generation batch repository is required")
	}
	if teamID <= 0 || targetID <= 0 {
		return nil, errors.New("generation batch list requires tenant and target")
	}
	if limit < 1 || limit > MaxGenerationBatchLimit {
		return nil, errors.New("generation batch limit must be between 1 and 100")
	}
	return s.batches.ListByTarget(ctx, teamID, targetID, limit)
}

func (s *GenerationBatchService) SetReviewPorts(signals GenerationSignalPort, audit GenerationAuditPort) {
	s.signals, s.audit = signals, audit
}

func (s *GenerationBatchService) SetWorkflowPort(workflow GenerationWorkflowPort) {
	s.workflow = workflow
}

func (s *GenerationBatchService) SetScopePort(scope GenerationScopePort) {
	s.scope = scope
}

func (s *GenerationBatchService) validateScope(ctx context.Context, cmd CreateGenerationBatchCommand) (string, error) {
	if cmd.TeamID <= 0 || cmd.TargetID <= 0 || cmd.RepoID <= 0 || cmd.ServiceID == nil || *cmd.ServiceID <= 0 {
		return "", errors.New("请选择当前项目中已确认的目录服务后生成用例")
	}
	if s.scope == nil {
		return "", errors.New("generation scope reader is required")
	}
	scope, err := s.scope.ReadGenerationScope(ctx, cmd.TeamID, cmd.TargetID, *cmd.ServiceID)
	if err != nil || scope == nil {
		return "", errors.New("当前项目中不存在该目录服务或仓库")
	}
	if scope.TeamID != cmd.TeamID || scope.TargetID != cmd.TargetID || scope.ServiceID != *cmd.ServiceID || scope.RepoID != cmd.RepoID {
		return "", errors.New("目录服务不属于所选项目和仓库")
	}
	if scope.Status != "active" || scope.GenerationStatus != "confirmed" || scope.ProfileStatus != "confirmed" {
		return "", errors.New("请先启用目录服务并确认技术画像与用例生成范围")
	}
	if strings.TrimSpace(scope.Language) == "" || strings.TrimSpace(scope.Framework) == "" {
		return "", errors.New("目录服务的技术画像不完整，请补充语言和框架后重试")
	}
	canonical, err := generationSourcePath(scope.SourcePath)
	if err != nil {
		return "", errors.New("目录服务的源码目录无效，请先修正目录登记")
	}
	if strings.TrimSpace(cmd.SourcePath) != "" {
		requested, err := generationSourcePath(cmd.SourcePath)
		if err != nil || requested != canonical {
			return "", errors.New("请求的源码目录与已确认的目录服务不一致")
		}
	}
	return canonical, nil
}

func generationSourcePath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\x00:\r\n") {
		return "", errors.New("invalid repository-relative path")
	}
	clean := path.Clean(value)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("source path escapes repository")
	}
	if clean == "." {
		return "", nil
	}
	return clean, nil
}

func sameGenerationRequest(batch *domain.GenerationBatch, cmd CreateGenerationBatchCommand) bool {
	return batch.TargetID == cmd.TargetID && batch.RepoID == cmd.RepoID && batch.ServiceID != nil && cmd.ServiceID != nil && *batch.ServiceID == *cmd.ServiceID && batch.SourcePath == cmd.SourcePath && batch.RefType == cmd.RefType && batch.Branch == cmd.Branch && batch.RequestedVersion == cmd.RequestedVersion
}

// Create returns the existing batch for a repeated tenant-scoped idempotency
// key. Repository implementations must enforce the same uniqueness in durable
// storage to preserve this guarantee across processes.
func (s *GenerationBatchService) Create(ctx context.Context, cmd CreateGenerationBatchCommand) (*domain.GenerationBatch, bool, error) {
	if s == nil || s.batches == nil {
		return nil, false, errors.New("generation batch repository is required")
	}
	canonicalPath, err := s.validateScope(ctx, cmd)
	if err != nil {
		return nil, false, err
	}
	cmd.SourcePath = canonicalPath
	if cmd.RefType == "" {
		cmd.RefType = "branch"
	}
	if cmd.RefType != "branch" && cmd.RefType != "tag" {
		return nil, false, errors.New("代码引用类型必须是 branch 或 tag")
	}
	existing, err := s.batches.FindByIdempotencyKey(ctx, cmd.TeamID, cmd.IdempotencyKey)
	if err == nil {
		if !sameGenerationRequest(existing, cmd) {
			return nil, false, errors.New("idempotency key is already used for a different generation scope")
		}
		if s.workflow != nil && existing.State == domain.GenerationEnqueued {
			if startErr := s.workflow.Start(ctx, existing.ID, existing.TeamID); startErr != nil {
				return nil, false, fmt.Errorf("start generation workflow: %w", startErr)
			}
		}
		return existing, false, nil
	}
	if !errors.Is(err, domain.ErrGenerationNotFound) {
		return nil, false, err
	}
	batch, err := domain.NewGenerationBatch(cmd.ID, cmd.TeamID, cmd.TargetID, cmd.RepoID, cmd.ActorID, cmd.Branch, cmd.RequestedVersion, cmd.IdempotencyKey, s.now())
	if err != nil {
		return nil, false, err
	}
	batch.ServiceID = cmd.ServiceID
	batch.SourcePath = cmd.SourcePath
	batch.RefType = cmd.RefType
	// The baseline is system-owned. Only an already approved batch is effective
	// and therefore eligible; drafts, failed runs, and user input never affect
	// the iteration chain.
	base, baseErr := s.batches.FindLatestApprovedByScope(ctx, cmd.TeamID, cmd.TargetID, cmd.RepoID, *cmd.ServiceID)
	if baseErr == nil {
		batch.BaseBatchID = &base.ID
	} else if !errors.Is(baseErr, domain.ErrGenerationNotFound) {
		return nil, false, fmt.Errorf("find effective generation baseline: %w", baseErr)
	}
	if err := s.batches.Create(ctx, batch); err != nil {
		// A competing create may have won. Re-read instead of leaking a
		// duplicate-key error as a failed idempotent request.
		if got, findErr := s.batches.FindByIdempotencyKey(ctx, cmd.TeamID, cmd.IdempotencyKey); findErr == nil {
			if !sameGenerationRequest(got, cmd) {
				return nil, false, errors.New("idempotency key is already used for a different generation scope")
			}
			return got, false, nil
		}
		return nil, false, err
	}
	if err := s.appendLifecycleAudit(ctx, batch, "generation.batch.created"); err != nil {
		return nil, false, fmt.Errorf("append generation creation audit: %w", err)
	}
	if s.workflow != nil {
		if err := s.workflow.Start(ctx, batch.ID, batch.TeamID); err != nil {
			return nil, false, fmt.Errorf("start generation workflow: %w", err)
		}
	}
	return batch, true, nil
}

func (s *GenerationBatchService) Transition(ctx context.Context, teamID, batchID, expectedVersion int64, next domain.GenerationBatchState) (*domain.GenerationBatch, error) {
	if s == nil || s.batches == nil {
		return nil, errors.New("generation batch repository is required")
	}
	batch, err := s.batches.Find(ctx, teamID, batchID)
	if err != nil {
		return nil, err
	}
	if err := batch.Transition(next, expectedVersion, s.now()); err != nil {
		return nil, err
	}
	if err := s.batches.CompareAndSwap(ctx, batch, expectedVersion); err != nil {
		return nil, fmt.Errorf("persist generation batch transition: %w", err)
	}
	if err := s.appendLifecycleAudit(ctx, batch, "generation.state."+string(next)); err != nil {
		return nil, fmt.Errorf("append generation lifecycle audit: %w", err)
	}
	return batch, nil
}

func (s *GenerationBatchService) appendLifecycleAudit(ctx context.Context, batch *domain.GenerationBatch, action string) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.AppendGenerationAudit(ctx, GenerationAuditEvent{TeamID: batch.TeamID, BatchID: batch.ID, ActorID: batch.CreatedBy, StateVersion: batch.StateVersion, Action: action, OccurredAt: s.now()})
}

func (s *GenerationBatchService) Review(ctx context.Context, cmd ReviewGenerationCommand) (*domain.GenerationBatch, error) {
	if cmd.TeamID <= 0 || cmd.BatchID <= 0 || cmd.ActorID <= 0 || cmd.ExpectedVersion <= 0 || cmd.RequestID == "" {
		return nil, errors.New("review requires tenant, batch, actor, version, and request id")
	}
	var next domain.GenerationBatchState
	switch cmd.Decision {
	case GenerationReviewApprove:
		next = domain.GenerationCommitting
	case GenerationReviewReject:
		next = domain.GenerationRejected
	default:
		return nil, errors.New("review decision must be approve or reject")
	}
	batch, err := s.Transition(ctx, cmd.TeamID, cmd.BatchID, cmd.ExpectedVersion, next)
	if err != nil {
		return nil, err
	}
	event := GenerationAuditEvent{TeamID: cmd.TeamID, BatchID: cmd.BatchID, ActorID: cmd.ActorID, StateVersion: batch.StateVersion, Action: "generation.review." + string(cmd.Decision), RequestID: cmd.RequestID, OccurredAt: s.now()}
	if s.audit != nil {
		if err := s.audit.AppendGenerationAudit(ctx, event); err != nil {
			return nil, fmt.Errorf("append generation review audit: %w", err)
		}
	}
	if s.signals != nil {
		if err := s.signals.SignalReview(ctx, GenerationSignal{TeamID: cmd.TeamID, BatchID: cmd.BatchID, ActorID: cmd.ActorID, StateVersion: batch.StateVersion, Decision: cmd.Decision, RequestID: cmd.RequestID}); err != nil {
			return nil, fmt.Errorf("signal generation review: %w", err)
		}
	}
	return batch, nil
}

type CancelGenerationCommand struct {
	TeamID, BatchID, ActorID, ExpectedVersion int64
	RequestID                                 string
}

// Cancel stops an in-flight batch and removes its disposable seeds/snapshot.
// The batch row remains as an auditable tombstone.
func (s *GenerationBatchService) Cancel(ctx context.Context, cmd CancelGenerationCommand) (*domain.GenerationBatch, error) {
	if cmd.TeamID <= 0 || cmd.BatchID <= 0 || cmd.ActorID <= 0 || cmd.ExpectedVersion <= 0 || cmd.RequestID == "" {
		return nil, errors.New("cancel requires tenant, batch, actor, version, and request id")
	}
	batch, err := s.Transition(ctx, cmd.TeamID, cmd.BatchID, cmd.ExpectedVersion, domain.GenerationCancelled)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		if err := s.audit.AppendGenerationAudit(ctx, GenerationAuditEvent{TeamID: cmd.TeamID, BatchID: cmd.BatchID, ActorID: cmd.ActorID, StateVersion: batch.StateVersion, Action: "generation.cancel", RequestID: cmd.RequestID, OccurredAt: s.now()}); err != nil {
			return nil, fmt.Errorf("append generation cancel audit: %w", err)
		}
	}
	if s.signals != nil {
		if err := s.signals.Cancel(ctx, GenerationSignal{TeamID: cmd.TeamID, BatchID: cmd.BatchID, ActorID: cmd.ActorID, StateVersion: batch.StateVersion, RequestID: cmd.RequestID}); err != nil {
			return nil, fmt.Errorf("cancel generation workflow: %w", err)
		}
	}
	if err := s.batches.CleanupBatch(ctx, cmd.TeamID, cmd.BatchID); err != nil {
		return nil, fmt.Errorf("cleanup cancelled generation batch: %w", err)
	}
	return batch, nil
}

func (s *GenerationBatchService) Rollback(ctx context.Context, cmd RollbackGenerationCommand) (*domain.GenerationBatch, error) {
	if cmd.TeamID <= 0 || cmd.BatchID <= 0 || cmd.ActorID <= 0 || cmd.ExpectedVersion <= 0 || cmd.RequestID == "" {
		return nil, errors.New("rollback requires tenant, batch, actor, version, and request id")
	}
	batch, err := s.Transition(ctx, cmd.TeamID, cmd.BatchID, cmd.ExpectedVersion, domain.GenerationRolledBack)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		if err := s.audit.AppendGenerationAudit(ctx, GenerationAuditEvent{TeamID: cmd.TeamID, BatchID: cmd.BatchID, ActorID: cmd.ActorID, StateVersion: batch.StateVersion, Action: "generation.rollback", RequestID: cmd.RequestID, OccurredAt: s.now()}); err != nil {
			return nil, fmt.Errorf("append generation rollback audit: %w", err)
		}
	}
	if s.signals != nil {
		if err := s.signals.SignalRollback(ctx, GenerationSignal{TeamID: cmd.TeamID, BatchID: cmd.BatchID, ActorID: cmd.ActorID, StateVersion: batch.StateVersion, RequestID: cmd.RequestID}); err != nil {
			return nil, fmt.Errorf("signal generation rollback: %w", err)
		}
	}
	return batch, nil
}
