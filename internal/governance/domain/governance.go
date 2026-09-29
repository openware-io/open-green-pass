// Package domain 定义治理域核心概念：被测对象树 / 测试用例 / 质量门禁。
// 本域负责被测对象资产建模、用例生命周期、门禁规则（零三方依赖，见 ENGINEERING-SPEC §8）。
package domain

import "context"

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
	Model   string            // 模型标识（如 gpt-4o / deepseek-v3）
	Provider string           // 模型提供方（openai / deepseek / local 等）
	Params  map[string]string // 额外参数（temperature 等）
}

// Repo 被测仓库（来源溯源：被测对象树 ← 仓库）。
type Repo struct {
	ID            int64
	TeamID        int64
	TargetID      int64 // 归属被测对象节点（工程/服务）
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

