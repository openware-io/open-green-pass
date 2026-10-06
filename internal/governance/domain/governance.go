// Package domain 定义治理域核心概念：被测对象树 / 测试用例 / 质量门禁。
// 本域负责被测对象资产建模、用例生命周期、门禁规则（零三方依赖，见 ENGINEERING-SPEC §8）。
package domain

import (
	"context"
	"encoding/json"
	"time"
)

// TargetNodeType 被测对象树节点类型（层级规范，见 PRD §被测对象）。
type TargetNodeType string

const (
	// NodeProject 工程（资产树顶层）。
	NodeProject TargetNodeType = "project"
	// NodeServiceGroup 服务组。
	NodeServiceGroup TargetNodeType = "service_group"
	// NodeService 服务。
	NodeService TargetNodeType = "service"
	// NodeModule 模块。
	NodeModule TargetNodeType = "module"
)

// TargetNode 被测对象树节点。
type TargetNode struct {
	ID       int64
	TeamID   int64 // RLS 租户列
	Type     TargetNodeType
	Name     string
	Kind     string // api_service / web_app / mobile_app / contract / ai_model / other
	Status   string // active / archived（默认 active）
	ParentID *int64 // 顶层为 nil
	RepoID   *int64 // 关联上游仓库（来源可溯源）
	OwnerID  int64  // 负责人（工程负责人完整权限）
}

// Testcase 测试用例（versioned，支持按版本回退与历史对比，见 PRD §用例管理）。
type Testcase struct {
	ID           int64
	TeamID       int64
	TargetNodeID int64  // 关联被测对象
	Scenario     string // 测试场景（API/契约/UI/性能/并发等）
	Name         string
	Version      int     // 用例版本（每次生成 +1）
	Status       string  // draft / active / deprecated
	ScriptRef    *string // 执行脚本引用（详见用例执行载体）
	DataRef      *string // 关联测试数据
}

// GateRule 质量门禁规则（对某被测对象配置：指标 + 阈值）。
type GateRule struct {
	ID           int64
	TeamID       int64
	TargetNodeID int64
	Metric       string // pass_rate / cost_budget / coverage 等
	Operator     string // > >= < <= ==
	Threshold    float64
	Enabled      bool
}

// ========= GP1-01 被测对象树 + 仓库绑定（来源溯源）=========

// ModelBinding 被测对象节点可选 AI 模型绑定（每工程可自选模型）。
type ModelBinding struct {
	Model    string            // 模型标识（如 gpt-4o / deepseek-v3）
	Provider string            // 模型提供方（openai / deepseek / local 等）
	Params   map[string]string // 额外参数（temperature 等）
}

// ModelStatus controls whether a registered model may be selected.
type ModelStatus string

const (
	ModelPending  ModelStatus = "pending"
	ModelApproved ModelStatus = "approved"
	ModelDisabled ModelStatus = "disabled"
)

// ModelConfig is the provider-independent model catalogue entry. Secret and
// endpoint material are references only; their values never enter this type.
type ModelConfig struct {
	ID             int64
	TeamID         int64
	Name           string
	Provider       string
	ModelKey       string
	Capabilities   []string
	Status         ModelStatus
	SecretRef      *string
	EndpointRef    *string
	DefaultTimeout int
	MaxTokensIn    int
	MaxTokensOut   int
	Revision       int
	CreatedBy      int64
}

func (m ModelConfig) Validate() error {
	if m.Name == "" || m.Provider == "" || m.ModelKey == "" {
		return NewErr("model config name, provider and model key required")
	}
	if m.Status == "" {
		return NewErr("model config status required")
	}
	return nil
}

// PriceSnapshot is immutable pricing selected for one model invocation.
type PriceSnapshot struct {
	ID            int64
	TeamID        int64
	ModelID       int64
	Currency      string
	InputPer1K    float64
	OutputPer1K   float64
	EffectiveFrom time.Time
	EffectiveTo   *time.Time
	Status        string
	ApprovedBy    *int64
	Revision      int
}

func (p PriceSnapshot) Validate() error {
	if p.TeamID == 0 || p.ModelID == 0 || p.Currency == "" || p.EffectiveFrom.IsZero() {
		return NewErr("price snapshot model, currency and effective time required")
	}
	if p.InputPer1K < 0 || p.OutputPer1K < 0 {
		return NewErr("price snapshot unit price cannot be negative")
	}
	if p.EffectiveTo != nil && !p.EffectiveTo.After(p.EffectiveFrom) {
		return NewErr("price snapshot effective range invalid")
	}
	return nil
}

// ModelGovernanceRepository is the persistence boundary for GP3-05.
type ModelGovernanceRepository interface {
	CreateModel(context.Context, *ModelConfig) error
	FindModel(context.Context, int64, int64) (*ModelConfig, error)
	ListModels(context.Context, int64) ([]*ModelConfig, error)
	AddPrice(context.Context, *PriceSnapshot) error
	ResolvePrice(context.Context, int64, int64, time.Time) (*PriceSnapshot, error)
}

// SecretResolver resolves an opaque reference at the infrastructure boundary.
// Model governance and provider-independent code never handles secret values
// or persists them; adapters consume this port only at call time.
type SecretResolver interface {
	Resolve(context.Context, string) (string, error)
}

// ResolvedModel is the immutable, provider-independent routing input returned
// by governance. Prices are copied as a snapshot for auditable metering.
type ResolvedModel struct {
	Model      ModelConfig
	Price      PriceSnapshot
	ResolvedAt time.Time
}

// ModelBindingPolicy is the governance boundary for model approval. The
// default application wiring uses NoopModelBindingPolicy until GP3-05's
// persistent whitelist/approval store is selected.
type ModelBindingPolicy interface {
	ValidateBinding(context.Context, int64, int64, ModelBinding) error
}

type NoopModelBindingPolicy struct{}

func (NoopModelBindingPolicy) ValidateBinding(context.Context, int64, int64, ModelBinding) error {
	return nil
}

var _ ModelBindingPolicy = NoopModelBindingPolicy{}

// Validate checks the provider-independent binding contract. Approval,
// whitelist and price resolution remain GP3 governance concerns.
func (b ModelBinding) Validate() error {
	if b.Model == "" {
		return NewErr("model binding model required")
	}
	if b.Provider == "" {
		return NewErr("model binding provider required")
	}
	return nil
}

// Repo 被测仓库（来源溯源：被测对象树 ← 仓库）。
type Repo struct {
	ID            int64
	TeamID        int64
	TargetID      int64  // 归属被测对象节点（工程/服务）
	Kind          string // git / svn / ...
	URL           string
	DefaultBranch string
	CredRef       *string // 凭据引用（密钥不落库，仅 Secret 名）
	CreatedBy     int64
}

// RepoBranch 仓库分支/版本（版本校验依据）。
type RepoBranch struct {
	ID        int64
	RepoID    int64
	Branch    string
	Version   string
	HeadSHA   *string
	CreatedBy int64
}

// Target 被测对象树节点聚合（含来源仓库与版本链）。
type Target struct {
	Node         TargetNode
	Repo         *Repo
	Branch       *RepoBranch
	ModelBinding *ModelBinding // 本工程/节点可选 AI 模型绑定（持久化到 model_binding JSONB）
}

// AttachRepo 绑定仓库与分支/版本（来源可溯源；同一节点可换绑）。
func (t *Target) AttachRepo(r Repo, b RepoBranch) {
	t.Repo = &r
	t.Branch = &b
	t.Node.RepoID = &r.ID
}

// SetModelBinding 设置本工程/节点的 AI 模型绑定。
func (t *Target) SetModelBinding(b ModelBinding) { t.ModelBinding = &b }

// ResolveVersionChain 解析目标版本链（当前绑定分支的最新版本）。
func (t *Target) ResolveVersionChain() (*RepoBranch, error) {
	if t.Branch == nil {
		return nil, ErrNoRepoBound
	}
	return t.Branch, nil
}

// ErrNoRepoBound 被测对象未绑定仓库/版本。
var ErrNoRepoBound = NewErr("target has no bound repo/branch")

// NewErr 构造带消息的领域错误（供零三方依赖使用）。
func NewErr(msg string) *DomainErr { return &DomainErr{msg: msg} }

// DomainErr 领域层错误。
type DomainErr struct{ msg string }

func (e *DomainErr) Error() string { return e.msg }

// TargetFilter 被测对象树查询筛选（层级维度 + 被测对象维度）。
type TargetFilter struct {
	Type  TargetNodeType // 为空表示全部层级
	Level *int           // 维度筛选：0/1/2/3
	Name  string         // 名称模糊
}

// TargetRepository 被测对象树仓储端口（实现位于 infra，RLS 由实现注入租户）。
type TargetRepository interface {
	SaveTarget(ctx context.Context, t *Target) error
	FindTarget(ctx context.Context, teamID, id int64) (*Target, error)
	ListTargets(ctx context.Context, teamID int64, f TargetFilter) ([]*Target, error)
	SaveRepo(ctx context.Context, r *Repo) error
	SaveBranch(ctx context.Context, b *RepoBranch) error
}

// ListTargetsByParent 列出某节点的直接子节点（用于资产树展开）。
type ChildrenFilter struct {
	ParentID *int64
	Type     TargetNodeType
}

// ========= GP1-02 用例版本化（变化可辨 + 来源可溯源）=========

// ChangeType 用例版本变化类型（随迭代可辨新增/更新/删除/回退）。
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"    // 新增（初始版本）
	ChangeUpdated  ChangeType = "updated"  // 更新（内容变更）
	ChangeDeleted  ChangeType = "deleted"  // 删除（标记，保留历史）
	ChangeRollback ChangeType = "rollback" // 回退（内容指向旧版本，非物理删）
)

// Case 测试用例（版本化主体，cas_case）。
type Case struct {
	ID             int64
	TeamID         int64
	TargetID       int64  // 归属被测对象节点（服务/模块）
	Code           string // 用例编号（按树命名空间稳定，如 im-saas-gw-001）
	Title          string
	Kind           string // 场景族：api/web/ui/contract/perf/mobile/weaknet/ai...
	CurrentVersion int    // 当前生效版本号
	Status         string // active / archived / deleted
	CreatedBy      int64  // 创建人（操作审计）
}

// CaseVersion 用例版本（只增不改；回退=新增指向旧内容的新版本）。
type CaseVersion struct {
	ID           int64
	TeamID       int64 // RLS 租户列
	CaseID       int64
	Version      int
	ChangeType   ChangeType
	SourceRepoID *int64           // 来源仓库（可溯源）
	SourceBranch *string          // 来源分支（可溯源）
	ScriptJSON   *json.RawMessage // 脚本/HTTP/规则/压测 + 参数/数据/断言
	ApprovedBy   *int64
	ApprovedAt   *time.Time
	CreatedBy    int64
	CreatedAt    time.Time
}

// CaseFilter 用例查询筛选。
type CaseFilter struct {
	TargetID *int64 // 按被测对象节点
	Kind     string // 按场景族
	Code     string // 按编号模糊
	Status   string // 按状态
}

// CaseRepository 用例仓储端口（实现位于 infra，RLS 由实现注入租户）。
type CaseRepository interface {
	SaveCase(ctx context.Context, c *Case) error
	SaveVersion(ctx context.Context, v *CaseVersion) error
	FindCase(ctx context.Context, teamID, id int64) (*Case, error)
	FindVersion(ctx context.Context, teamID, caseID int64, version int) (*CaseVersion, error)
	ListCases(ctx context.Context, teamID int64, f CaseFilter) ([]*Case, error)
	History(ctx context.Context, teamID, caseID int64) ([]*CaseVersion, error)
}
