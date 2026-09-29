// Package application 可信域应用服务层接口。
// 可信域数据由 gp_trusted_writer 独占写（见 ENGINEERING-SPEC §8.1）；此处声明应用契约（禁 ORM/Temporal）。
package application

// CostService 成本审计应用服务。
type CostService interface {
	// 后续实现：生成/执行两类成本 / 按团队·服务归因 / 用例成本历史对比
}

// EvidenceService 证据链应用服务。
type EvidenceService interface {
	// 后续实现：哈希链锚定 / 截图证据落库与校验
}

// AuditService 操作审计应用服务。
type AuditService interface {
	// 后续实现：操作留痕 / 只读校验
}
