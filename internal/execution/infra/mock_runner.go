// 执行器实现：P1 用 MockRunner 本地闭环（状态机/勾选/重试/明细落库全链路），K8s Job 沙箱 Runner 独立实现（gp1-04-b）。
package infra

import (
	"context"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// MockRunner 本地同步执行器：直接返回 pass（P1 验证编排链路；真实 HTTP/K8s 执行由 K8sRunner 接管）。
type MockRunner struct {
	gen *id.Generator
}

// NewMockRunner 创建本地执行器。
func NewMockRunner(gen *id.Generator) *MockRunner { return &MockRunner{gen: gen} }

// Execute 同步执行用例规格，构造结果（含成本计量占位：ai_tokens/cost_amount 由成本域单点回填）。
func (r *MockRunner) Execute(ctx context.Context, spec []*domain.CaseSpec) ([]*domain.CaseResult, error) {
	results := make([]*domain.CaseResult, 0, len(spec))
	for _, s := range spec {
		now := time.Now().UTC()
		ended := now
		results = append(results, &domain.CaseResult{
			ID: r.gen.Next(), CaseID: s.CaseID, CaseVersion: s.CaseVersion,
			Status: domain.CasePass, ResultText: "mock ok", AttemptSeq: 1,
			Evidence: &domain.EvidenceRef{Hash: "mock-evidence"},
			StartedAt: now, EndedAt: &ended,
		})
	}
	return results, nil
}

var _ domain.RunnerPort = (*MockRunner)(nil)
