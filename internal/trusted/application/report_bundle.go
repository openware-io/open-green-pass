package application

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
)

type reportBundleReader interface {
	FindMany(context.Context, int64, []int64) ([]*domain.Report, error)
}

type BundleRequest struct {
	ReportIDs []int64 `json:"report_ids"`
	Format    string  `json:"format"`
}

type Bundle struct {
	Reports  []*domain.Report `json:"reports"`
	Total    int              `json:"total"`
	Pass     int              `json:"pass"`
	Fail     int              `json:"fail"`
	HTML     string           `json:"html,omitempty"`
	Markdown string           `json:"markdown,omitempty"`
}

func (s *ReportService) BuildBundle(ctx context.Context, ids []int64, format string) (*Bundle, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	reader, ok := s.reports.(reportBundleReader)
	if !ok {
		return nil, fmt.Errorf("report repository does not support bundles")
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("report_ids required")
	}
	reports, err := reader.FindMany(ctx, teamID, ids)
	if err != nil {
		return nil, err
	}
	b := &Bundle{Reports: reports, Total: len(reports)}
	for _, r := range reports {
		if r.Status == "pass" {
			b.Pass++
		}
		if r.Status == "fail" || r.Status == "blocked" {
			b.Fail++
		}
	}
	b.Markdown = renderBundleMarkdown(reports, b)
	b.HTML = "<html><head><meta charset=\"utf-8\"><title>GreenPass report bundle</title></head><body>" + html.EscapeString("GreenPass 跨场景测试报告") + "<pre>" + html.EscapeString(b.Markdown) + "</pre></body></html>"
	if strings.EqualFold(format, "markdown") {
		b.HTML = ""
	}
	return b, nil
}

func renderBundleMarkdown(reports []*domain.Report, b *Bundle) string {
	var out strings.Builder
	out.WriteString("# GreenPass 跨场景测试报告\n\n")
	fmt.Fprintf(&out, "- 报告数：%d\n- 通过：%d\n- 失败/阻断：%d\n\n", b.Total, b.Pass, b.Fail)
	out.WriteString("| 报告 | 场景 | 版本 | 状态 |\n|---|---|---|---|\n")
	for _, r := range reports {
		fmt.Fprintf(&out, "| %d | %s | %s | %s |\n", r.ID, r.Scenario, r.Version, r.Status)
	}
	return out.String()
}
