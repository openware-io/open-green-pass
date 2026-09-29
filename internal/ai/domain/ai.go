// Package domain 定义 AI 域核心概念：模型配置 / AI 调用计量。
// 本域负责各被测对象可选的 AI 模型与调用成本原始计量（统一 AI 网关，见 TECH-DESIGN §D4）。
package domain

// ModelProvider AI 模型供应商。
type ModelProvider string

const (
	ProviderDoubao    ModelProvider = "doubao"
	ProviderOpenAI    ModelProvider = "openai"
	ProviderAnthropic ModelProvider = "anthropic"
	ProviderLocal     ModelProvider = "local"
)

// ModelConfig 被测对象/用例可选的 AI 模型配置。
type ModelConfig struct {
	ID           int64
	TeamID       int64
	Name         string // 配置名
	Provider     ModelProvider
	Model        string  // 模型标识
	BaseURL      *string // 网关/代理地址
	APIKeyRef    *string // 密钥引用（不落明文）
	MaxTokensIn  int
	MaxTokensOut int
}

// AICall 一次 AI 调用原始计量（成本行按此归集，单位：厘）。
type AICall struct {
	ID        int64
	TeamID    int64
	ModelRef  int64 // 关联 ModelConfig
	TokensIn  int
	TokensOut int
	// CostMillis 本次调用成本（厘）。精度与 cost_line_item 对齐（见 TECH-DESIGN §9.3）。
	CostMillis int64
	Err        bool
}
