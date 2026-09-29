// Package application 执行域应用服务层接口。
// 工作流编排实现在 infra/workflow（Temporal）；此处声明应用契约（禁 Temporal SDK，见 ENGINEERING-SPEC §8）。
package application

// RunService 测试执行应用服务。
type RunService interface {
	// 后续实现：启动执行 / 暂停全部 / 勾选用例子集执行 / 按用例树范围筛选
}

// WorkflowService 执行编排应用服务。
type WorkflowService interface {
	// 后续实现：用例执行工作流 / 门禁校验工作流
}
