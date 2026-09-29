// Package protocol 提供跨域协议常量（网关透传头、成本计量单位、证据链锚点等）。
package protocol

// 网关透传头（tracing / 租户 / 操作人）。
const (
	// HeaderRequestID 请求追踪 ID 透传头。
	HeaderRequestID = "x-gp-request-id"
	// HeaderTenantID 租户（team_id）透传头，由 gateway tenant 中间件消费。
	HeaderTenantID = "x-gp-team-id"
	// HeaderUserID 操作人（user_id）透传头，审计字段使用。
	HeaderUserID = "x-gp-user-id"
)

// 成本计量：单位定义为"厘"（0.01 元），整型避免浮点误差（成本审计按行精确记账）。
const (
	// CostUnit 成本单位：厘（0.01 元）。
	CostUnit = "厘"
)

// 证据链：审计锚点命名空间（hashchain genesis 校验锚点）。
const (
	// EvidenceChainNS 证据链命名空间前缀。
	EvidenceChainNS = "gp:evidence"
)
