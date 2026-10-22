// 缺陷（Finding）领域模型：严重级别、状态机、去重与证据摘要。
// 零第三方依赖；持久化由 infra 实现。
package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Severity 缺陷严重级别。
type Severity string

const (
	SeverityP0 Severity = "P0"
	SeverityP1 Severity = "P1"
	SeverityP2 Severity = "P2"
	SeverityP3 Severity = "P3"
)

// FindingStatus 缺陷生命周期状态。
type FindingStatus string

const (
	FindingCandidate         FindingStatus = "candidate"
	FindingTriaged           FindingStatus = "triaged"
	FindingConfirmed         FindingStatus = "confirmed"
	FindingFixing            FindingStatus = "fixing"
	FindingPendingRegression FindingStatus = "pending_regression"
	FindingClosed            FindingStatus = "closed"
	FindingFalsePositive     FindingStatus = "false_positive"
)

// FindingAction 状态流转动作（API 使用稳定英文标识）。
type FindingAction string

const (
	ActionTriage            FindingAction = "triage"
	ActionConfirm           FindingAction = "confirm"
	ActionFalsePositive     FindingAction = "false_positive"
	ActionPendingRegression FindingAction = "pending_regression"
	ActionClose             FindingAction = "close"
	ActionReopen            FindingAction = "reopen"
)

// Confidence 根因置信度。
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// FindingRepositoryRef 缺陷归属仓库/冻结提交（来源可追溯）。
type FindingRepositoryRef struct {
	ID        *int64
	Name      string
	URL       string
	Branch    string
	CommitSHA string
}

// FindingEvidence 首轮证据摘要；完整证据后续落 evd_evidence。
type FindingEvidence struct {
	Request  string `json:"request,omitempty"`
	Response string `json:"response,omitempty"`
	Log      string `json:"log,omitempty"`
	Trace    string `json:"trace,omitempty"`
}

// FindingEvent 缺陷流转事件（append-only）。
type FindingEvent struct {
	ID         int64
	TeamID     int64
	FindingID  int64
	Action     string
	FromStatus string
	ToStatus   string
	ActorID    int64
	Reason     string
	Payload    map[string]any
	CreatedAt  time.Time
}

// Finding 缺陷聚合（repo/commit 必填；状态机见 Transition）。
type Finding struct {
	ID            int64
	TeamID        int64
	ProjectID     int64
	ServiceID     *int64
	EndpointID    *int64
	TargetID      *int64
	Repository    FindingRepositoryRef
	Module        string
	Endpoint      string
	Title         string
	Description   string
	Severity      Severity
	Category      string
	Status        FindingStatus
	Expected      string
	Actual        string
	RootCause     string
	Impact        string
	Confidence    Confidence
	Reproduction  map[string]any
	SourceRef     map[string]any
	Evidence      FindingEvidence
	CoverageCells []string
	OwnerID       *int64
	LinkedRunID   *int64
	LinkedCaseID  *int64
	DedupeHash    string
	CreatedBy     int64
	CreatedAt     time.Time
	UpdatedBy     *int64
	UpdatedAt     time.Time
	Events        []FindingEvent
}

// FindingFilter 缺陷列表筛选。
type FindingFilter struct {
	Severity string
	Status   string
	Query    string
	Module   string
	Limit    int
	Offset   int
}

// FindingRepository 缺陷仓储端口；实现负责 RLS 与事务。
type FindingRepository interface {
	// Create 在同一事务中写 finding + 首个 event；created=false 表示去重命中。
	Create(ctx context.Context, f *Finding, event *FindingEvent) (bool, error)
	Find(ctx context.Context, teamID, id int64) (*Finding, error)
	FindByDedupe(ctx context.Context, teamID int64, hash string) (*Finding, error)
	List(ctx context.Context, teamID int64, f FindingFilter) ([]*Finding, error)
	UpdateStatus(ctx context.Context, f *Finding, event *FindingEvent) error
	Events(ctx context.Context, teamID, findingID int64) ([]FindingEvent, error)
	// CountOpen returns unresolved P0/P1 findings for a project/module.
	CountOpen(ctx context.Context, teamID, projectID int64, module string) (p0, p1 int, err error)
}

// Validate 校验缺陷必填字段与枚举。
func (f *Finding) Validate() error {
	if strings.TrimSpace(f.Title) == "" {
		return NewErr("finding title required")
	}
	if !f.Severity.Valid() {
		return NewErr("finding severity invalid")
	}
	if f.Status == "" {
		f.Status = FindingCandidate
	}
	if !f.Status.Valid() {
		return NewErr("finding status invalid")
	}
	if strings.TrimSpace(f.Repository.URL) == "" {
		return NewErr("finding repository url required")
	}
	if strings.TrimSpace(f.Repository.CommitSHA) == "" {
		return NewErr("finding commit sha required")
	}
	if f.Confidence == "" {
		f.Confidence = ConfidenceMedium
	}
	if !f.Confidence.Valid() {
		return NewErr("finding confidence invalid")
	}
	if f.Category == "" {
		f.Category = "product"
	}
	return nil
}

// Valid 判断严重级别。
func (s Severity) Valid() bool {
	return s == SeverityP0 || s == SeverityP1 || s == SeverityP2 || s == SeverityP3
}

// Valid 判断状态。
func (s FindingStatus) Valid() bool {
	switch s {
	case FindingCandidate, FindingTriaged, FindingConfirmed, FindingFixing, FindingPendingRegression, FindingClosed, FindingFalsePositive:
		return true
	}
	return false
}

// Valid 判断置信度。
func (c Confidence) Valid() bool {
	return c == ConfidenceHigh || c == ConfidenceMedium || c == ConfidenceLow
}

// Transition 执行状态流转；返回 from/to，非法流转返回领域错误。
func (f *Finding) Transition(action FindingAction, actor int64, reason string) (FindingStatus, FindingStatus, error) {
	from := f.Status
	if from == "" {
		from = FindingCandidate
	}
	var to FindingStatus
	switch action {
	case ActionTriage:
		if from != FindingCandidate {
			return from, from, NewErr("only candidate can be triaged")
		}
		to = FindingTriaged
	case ActionConfirm:
		if from != FindingCandidate && from != FindingTriaged {
			return from, from, NewErr("only candidate/triaged can be confirmed")
		}
		to = FindingConfirmed
	case ActionFalsePositive:
		if from == FindingClosed {
			return from, from, NewErr("closed finding cannot be marked false positive")
		}
		to = FindingFalsePositive
	case ActionPendingRegression:
		if from != FindingConfirmed && from != FindingFixing {
			return from, from, NewErr("only confirmed/fixing can pending regression")
		}
		to = FindingPendingRegression
	case ActionClose:
		if from != FindingPendingRegression {
			return from, from, NewErr("only pending_regression can be closed")
		}
		to = FindingClosed
	case ActionReopen:
		if from != FindingClosed && from != FindingFalsePositive {
			return from, from, NewErr("only closed/false_positive can be reopened")
		}
		to = FindingConfirmed
	default:
		return from, from, NewErr("unknown finding action")
	}
	f.Status = to
	f.UpdatedAt = time.Now().UTC()
	if actor > 0 {
		f.UpdatedBy = &actor
	}
	_ = reason
	return from, to, nil
}

// ComputeFindingDedupeHash 计算稳定去重键；不包含 run/时间/证据。
func ComputeFindingDedupeHash(repoURL, endpoint, category, rootCause, expected, actual string) string {
	parts := []string{
		normalize(repoURL), normalize(endpoint), normalize(category),
		normalize(rootCause), normalize(expected), normalize(actual),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}
