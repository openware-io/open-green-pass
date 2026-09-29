// Package domain 定义可信域核心概念：成本行 / 证据链 / 审计操作。
// 本域数据由 gp_trusted_writer 独占写（aud_*/cost_*/gate_result），见 ENGINEERING-SPEC §8.1。
package domain

// CostCategory 成本大类（生成用例 / 执行测试），见 PRD §成本审计。
type CostCategory string

const (
	CostGenerate CostCategory = "generate" // 生成测试用例
	CostExecute  CostCategory = "execute"  // 执行测试用例
)

// CostLine 成本行（按用例执行粒度，幂等）。
type CostLine struct {
	ID             int64
	TeamID         int64
	Category       CostCategory
	ModelRef       int64
	TokensIn       int
	TokensOut      int
	CostMillis     int64  // 成本（厘）
	IdempotencyKey string // 幂等键（UNIQUE，防重复记账）
	BizRef         *int64 // 关联业务对象（用例/执行）
	BizSeq         *int   // 关联业务对象的尝试序号（成本对比到历史）
}

// EvidenceBlock 证据链块（哈希链，hashchain 计算，防篡改）。
type EvidenceBlock struct {
	ID       int64
	TeamID   int64
	Seq      int64
	PrevHash string
	DataHash string
	DataRef  string // 证据对象引用（截图/报告）
}
