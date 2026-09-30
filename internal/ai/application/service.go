// Package application AI 域应用服务层接口。
// 统一 AI 网关（D4）实现在 infra；此处声明应用契约（禁 Temporal SDK，见 ENGINEERING-SPEC §8）。
package application

import (
	"context"
	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// ModelService 模型配置应用服务。
type ModelService interface {
	// 后续实现：模型配置 / 按被测对象选型 / 密钥托管
}

// GatewayService 统一 AI 网关应用服务。
type GatewayService interface {
	Generate(context.Context, domain.GenerateRequest) (domain.GenerateResult, error)
}
