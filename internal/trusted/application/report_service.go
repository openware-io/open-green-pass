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
	stats   domain.RunStatPort
	cost    domain.CostRepository
	gates   domain.GateRepository
	reports domain.ReportRepository
	gen     *id.Generator
	tmpl    *template.Template
}

// NewReportService 创建报告服务。
func NewReportService(stats domain.RunStatPort, cost domain.CostRepository,
	gates domain.GateRepository, reports domain.ReportRepository, gen *id.Generator,
) (*ReportService, error) {
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
	kind := reportKind(req.Kind)
	existing, found, err := s.findExistingReport(ctx, teamID, req.RunID, kind)
	if err != nil {
		return nil, err
	}
	if found {
		return existing, nil
	}
	head, cases, gates, err := s.loadAggregates(ctx, teamID, req.RunID)
	if err != nil {
		return nil, err
	}
	view := buildReportView(req.RunID, kind, head, cases, gates)
	var buf bytes.Buffer
	if err := s.tmpl.Execute(&buf, view); err != nil {
		return nil, err
	}
	rep := &domain.Report{
		ID: s.gen.Next(), TeamID: teamID, RunID: req.RunID, TargetID: head.TargetID,
		Kind: kind, Title: view.Title, Version: head.Version, Branch: head.Branch,
		Scenario: itoa3(head.ScenarioID), Status: view.Status,
		Summary: map[string]any{
			"total": view.Summary.Total, "pass": view.Summary.Pass, "fail": view.Summary.Fail,
			"cost_total": view.Summary.CostTotal, "tokens": view.Summary.Tokens,
		},
		HTML: buf.String(), CreatedAt: view.CreatedAt,
	}
	if err := s.reports.Save(ctx, rep); err != nil {
		return nil, err
	}
	return rep, nil
}

func reportKind(kind string) string {
	if kind == "" {
		return "service"
	}
	return kind
}

func (s *ReportService) findExistingReport(ctx context.Context, teamID, runID int64, kind string) (*domain.Report, bool, error) {
	existing, err := s.reports.FindByRunKind(ctx, teamID, runID, kind)
	if err == nil {
		return existing, true, nil
	}
	if err != domain.ErrReportNotFound {
		return nil, false, err
	}
	return nil, false, nil
}

func (s *ReportService) loadAggregates(ctx context.Context, teamID, runID int64) (*domain.RunHead, []*domain.CaseResult, []*domain.GateResult, error) {
	head, err := s.stats.RunHead(ctx, teamID, runID)
	if err != nil {
		return nil, nil, nil, err
	}
	cases, err := s.stats.CaseResults(ctx, teamID, runID)
	if err != nil {
		return nil, nil, nil, err
	}
	gates, err := s.gates.ResultsByRun(ctx, teamID, runID)
	if err != nil {
		return nil, nil, nil, err
	}
	return head, cases, gates, nil
}

func buildReportView(runID int64, kind string, head *domain.RunHead, cases []*domain.CaseResult, gates []*domain.GateResult) reportView {
	summary := summarizeCases(cases)
	return reportView{
		Title: "软件工程测试治理平台 · 测试报告", Kind: kind, Status: reportStatus(summary, gates),
		RunID: runID, Head: head, CreatedAt: time.Now().UTC(),
		Summary: summary, Gates: gateViews(gates), Cases: caseViews(cases),
	}
}

func summarizeCases(cases []*domain.CaseResult) reportSummary {
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
	return summary
}

func reportStatus(summary reportSummary, gates []*domain.GateResult) string {
	if len(gates) > 0 {
		return string(gates[len(gates)-1].Result)
	}
	if summary.Fail > 0 {
		return "fail"
	}
	return "pass"
}

func gateViews(gates []*domain.GateResult) []gateView {
	var views []gateView
	for _, g := range gates {
		detail, _ := json.Marshal(g.Detail)
		views = append(views, gateView{
			Result: string(g.Result), RuleID: itoa3(g.RuleID),
			DecidedAt: g.DecidedAt, DetailJSON: string(detail),
		})
	}
	return views
}

func caseViews(cases []*domain.CaseResult) []caseView {
	var views []caseView
	for _, c := range cases {
		views = append(views, caseView{
			CaseID: c.CaseID, Attempt: int64(c.Attempt),
			CaseCode: c.CaseCode, Kind: c.Kind, Status: c.Status,
			Tokens: c.TokensIn + c.TokensOut, Cost: c.Cost, EvidenceNote: evidenceNote(c),
		})
	}
	return views
}

func evidenceNote(c *domain.CaseResult) string {
	if c.Evidence == nil {
		return ""
	}
	if url, ok := c.Evidence["url"].(string); ok && url != "" {
		return "证据✓"
	}
	if hash, ok := c.Evidence["hash"].(string); ok && hash != "" {
		return "哈希✓"
	}
	return "证据记录"
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
