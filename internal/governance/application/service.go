// Package application 治理域应用服务层接口。
// 实现依赖 infra 仓库（P1 落地）；此处仅声明契约（禁 ORM/Temporal，见 ENGINEERING-SPEC §8）。
package application

// TargetTreeService 被测对象树应用服务。
type TargetTreeService interface {
	// 后续实现：树查询 / 团队权限分配 / 仓库关联 / 来源溯源
}

// TestcaseService 用例管理应用服务。
type TestcaseService interface {
	// 后续实现：用例生成批次 / 版本 / 按版本回退 / 历史对比
}

// GateService 质量门禁应用服务。
type GateService interface {
	// 后续实现：门禁规则配置 / 校验执行结果
}
