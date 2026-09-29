// 可信域门禁应用服务：策略装载 + 判定（基于运行用例统计，声明式策略 P1）。
package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// RunStatPort 读取执行域运行元信息与用例统计（trusted 只读 run_*）。
// 定义于 domain，此处为应用层引用约束；实现见 infra.RunStatReader。

// GateService 门禁应用服务。
type GateService struct {
	repo  domain.GateRepository
	audit *AuditService
	stats domain.RunStatPort
	gen   *id.Generator
}

// NewGateService 创建门禁服务。
func NewGateService(repo domain.GateRepository, audit *AuditService, stats domain.RunStatPort, gen *id.Generator) *GateService {
	return &GateService{repo: repo, audit: audit, stats: stats, gen: gen}
}

// UpsertRule 装载门禁规则（版本递增；策略即代码，Rego 进 git）。
func (s *GateService) UpsertRule(ctx context.Context, targetID, scenarioID int64, rego string, enabled bool) (*domain.GateRule, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	ver := 1
	if cur, err := s.repo.ActiveRule(ctx, teamID, targetID, scenarioID); err == nil {
		ver = cur.Version + 1
	}
	rule := &domain.GateRule{
		ID: s.gen.Next(), TeamID: teamID, TargetID: targetID, ScenarioID: scenarioID,
		Rego: rego, Version: ver, Enabled: enabled,
	}
	if err := s.repo.UpsertRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// Evaluate 门禁判定：读运行统计 → 装载 active 规则 → 判定 → 落结果 + 入审计链。
// 无规则→blocked；fail>max_failed 或 pass_rate<min_pass_rate→fail；否则 pass。
func (s *GateService) Evaluate(ctx context.Context, runID int64) (*domain.GateResult, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	targetID, scenarioID, err := s.stats.RunMeta(ctx, teamID, runID)
	if err != nil {
		return nil, err
	}
	total, pass, fail, err := s.stats.CaseStats(ctx, teamID, runID)
	if err != nil {
		return nil, err
	}

	detail := map[string]any{"total": total, "pass": pass, "fail": fail}
	decision := domain.GatePass
	rule, err := s.repo.ActiveRule(ctx, teamID, targetID, scenarioID)
	if err != nil {
		decision = domain.GateBlocked // 无规则/未启用
		detail["reason"] = "no active gate rule"
	} else {
		var p domain.GatePolicy
		_ = json.Unmarshal([]byte(rule.Rego), &p)
		detail["policy"] = p
		if fail > p.MaxFailed {
			decision = domain.GateFail
			detail["reason"] = "failed cases exceed max_failed"
		} else if total > 0 && float64(pass)/float64(total) < p.MinPassRate {
			decision = domain.GateFail
			detail["reason"] = "pass rate below min_pass_rate"
		} else {
			detail["reason"] = "policy satisfied"
		}
	}

	res := &domain.GateResult{
		ID: s.gen.Next(), TeamID: teamID, RunID: runID,
		RuleID: 0, Result: decision, Detail: detail, DecidedAt: time.Now().UTC(),
	}
	if rule != nil {
		res.RuleID = rule.ID
	}
	if err := s.repo.SaveResult(ctx, res); err != nil {
		return nil, err
	}
	// 判定入审计链
	if _, err := s.audit.Append(ctx, 0, "gate.evaluate", "run", runID, detail); err != nil {
		return nil, err
	}
	return res, nil
}

// ResultsByRun 查询运行的门禁判定结果。
func (s *GateService) ResultsByRun(ctx context.Context, runID int64) ([]*domain.GateResult, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.ResultsByRun(ctx, teamID, runID)
}

var _ domain.GatePort = (*GateService)(nil)
