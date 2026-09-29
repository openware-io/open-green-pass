// Package domain 定义治理域核心概念：被测对象树 / 测试用例 / 质量门禁。
// 本域负责被测对象资产建模、用例生命周期、门禁规则（零三方依赖，见 ENGINEERING-SPEC §8）。
package domain

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
