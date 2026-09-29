// 可信域报告应用服务：由运行聚合生成测试报告（HTML），支持导出（fmt=html）。
package application

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"time"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

//go:embed report_templates/*.html
var reportTemplates embed.FS

// ReportService 测试报告应用服务（P1 单场景，结构预留跨场景合编 P2）。
type ReportService struct {
	stats    domain.RunStatPort
	cost     domain.CostRepository
	gates    domain.GateRepository
	reports  domain.ReportRepository
	gen      *id.Generator
	tmpl     *template.Template
}

// NewReportService 创建报告服务。
func NewReportService(stats domain.RunStatPort, cost domain.CostRepository,
	gates domain.GateRepository, reports domain.ReportRepository, gen *id.Generator) (*ReportService, error) {
	tmpl, err := template.ParseFS(reportTemplates, "report_templates/report.html")
	if err != nil {
		return nil, err
	}
	return &ReportService{stats: stats, cost: cost, gates: gates, reports: reports, gen: gen, tmpl: tmpl}, nil
}

// GenerateRequest 生成报告请求。
type GenerateRequest struct {
	RunID int64  `json:"run_id"`
	Kind  string `json:"kind"` // project/service/case（P1 单场景 service；跨场景合编 P2）
}

type reportSummary struct {
	Total, Pass, Fail, Tokens int64
	CostTotal                 float64
}

type gateView struct {
	Result, RuleID string
	DecidedAt      time.Time
	DetailJSON     string
}

type caseView struct {
	CaseID, Attempt, Tokens int64
	CaseCode, Kind, Status  string
	Cost                    float64
	EvidenceNote            string
}

type reportView struct {
	Title, Kind, Status string
	RunID               int64
	Head                *domain.RunHead
	CreatedAt           time.Time
	Summary             reportSummary
	Gates               []gateView
	Cases               []caseView
}

// Generate 由运行聚合生成测试报告并落库。
func (s *ReportService) Generate(ctx context.Context, req GenerateRequest) (*domain.Report, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	if req.RunID == 0 {
		return nil, ErrTenantRequired
	}
	kind := req.Kind
	if kind == "" {
		kind = "service"
	}
	head, err := s.stats.RunHead(ctx, teamID, req.RunID)
	if err != nil {
		return nil, err
	}
	cases, err := s.stats.CaseResults(ctx, teamID, req.RunID)
	if err != nil {
		return nil, err
	}
	gates, err := s.gates.ResultsByRun(ctx, teamID, req.RunID)
	if err != nil {
		return nil, err
	}
	// 汇总
	summary := reportSummary{Total: int64(len(cases))}
	for _, c := range cases {
		summary.Tokens += c.TokensIn + c.TokensOut
		summary.CostTotal += c.Cost
		switch c.Status {
		case "pass":
			summary.Pass++
		case "fail":
			summary.Fail++
		}
	}
	status := "pass"
	if summary.Fail > 0 {
		status = "fail"
	}
	if len(gates) > 0 {
		status = string(gates[len(gates)-1].Result)
	}
	// 门禁视图
	var gv []gateView
	for _, g := range gates {
		d, _ := json.Marshal(g.Detail)
		gv = append(gv, gateView{Result: string(g.Result), RuleID: itoa3(g.RuleID),
			DecidedAt: g.DecidedAt, DetailJSON: string(d)})
	}
	// 用例视图
	var cv []caseView
	for _, c := range cases {
		note := ""
		if c.Evidence != nil {
			if u, ok2 := c.Evidence["url"].(string); ok2 && u != "" {
				note = "证据✓"
			} else if h, ok2 := c.Evidence["hash"].(string); ok2 && h != "" {
				note = "哈希✓"
			} else {
				note = "证据记录"
			}
		}
		cv = append(cv, caseView{CaseID: c.CaseID, Attempt: int64(c.Attempt),
			CaseCode: c.CaseCode, Kind: c.Kind, Status: c.Status,
			Tokens: c.TokensIn + c.TokensOut, Cost: c.Cost, EvidenceNote: note})
	}
	view := reportView{
		Title: "软件工程测试治理平台 · 测试报告", Kind: kind, Status: status,
		RunID: req.RunID, Head: head, CreatedAt: time.Now().UTC(),
		Summary: summary, Gates: gv, Cases: cv,
	}
	var buf bytes.Buffer
	if err := s.tmpl.Execute(&buf, view); err != nil {
		return nil, err
	}
	rep := &domain.Report{
		ID: s.gen.Next(), TeamID: teamID, RunID: req.RunID, TargetID: head.TargetID,
		Kind: kind, Title: view.Title, Version: head.Version, Branch: head.Branch,
		Scenario: itoa3(head.ScenarioID), Status: status,
		Summary: map[string]any{
			"total": summary.Total, "pass": summary.Pass, "fail": summary.Fail,
			"cost_total": summary.CostTotal, "tokens": summary.Tokens,
		},
		HTML: buf.String(), CreatedAt: view.CreatedAt,
	}
	if err := s.reports.Save(ctx, rep); err != nil {
		return nil, err
	}
	return rep, nil
}

// Export 取回报告（含渲染 HTML）。
func (s *ReportService) Export(ctx context.Context, id int64) (*domain.Report, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.reports.Find(ctx, teamID, id)
}

func itoa3(n int64) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
