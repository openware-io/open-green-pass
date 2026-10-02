package application

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"
	"unicode"

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

var ErrUnsupportedBundleFormat = fmt.Errorf("unsupported report bundle format")

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
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "html"
	}
	if format != "html" && format != "markdown" && format != "pdf" && format != "docx" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedBundleFormat, format)
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
	if format == "markdown" {
		b.HTML = ""
	}
	return b, nil
}

// RenderBundle produces a downloadable artifact from an already authorized
// bundle. PDF and DOCX are dependency-free reference renderers; production
// typography/font and office-compatibility validation remains integration work.
func (b *Bundle) RenderBundle(format string) (content []byte, contentType, extension string, err error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "pdf":
		return renderPDF(b.Markdown), "application/pdf", "pdf", nil
	case "docx":
		data, renderErr := renderDOCX(b)
		return data, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "docx", renderErr
	default:
		return nil, "", "", fmt.Errorf("%w: %s", ErrUnsupportedBundleFormat, format)
	}
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

func renderDOCX(b *Bundle) ([]byte, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	files := map[string]string{
		"[Content_Types].xml":          `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":                  `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(w, content); err != nil {
			return nil, err
		}
	}
	w, err := zw.Create("word/document.xml")
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(w, docxDocument(b)); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func docxDocument(b *Bundle) string {
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, line := range strings.Split(b.Markdown, "\n") {
		out.WriteString(`<w:p><w:r><w:t xml:space="preserve">`)
		out.WriteString(html.EscapeString(line))
		out.WriteString(`</w:t></w:r></w:p>`)
	}
	out.WriteString(`<w:sectPr/></w:body></w:document>`)
	return out.String()
}

func renderPDF(markdown string) []byte {
	lines := strings.Split(markdown, "\n")
	var stream strings.Builder
	stream.WriteString("BT /F1 10 Tf 50 760 Td\n")
	for i, line := range lines {
		if i > 0 {
			stream.WriteString("0 -14 Td\n")
		}
		stream.WriteString("(")
		stream.WriteString(pdfASCII(line))
		stream.WriteString(") Tj\n")
	}
	stream.WriteString("ET\n")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		"<< /Length " + strconv.Itoa(len(stream.String())) + " >>\nstream\n" + stream.String() + "endstream",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i := 1; i < len(offsets); i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}

func pdfASCII(s string) string {
	var out strings.Builder
	for _, r := range s {
		if r > unicode.MaxASCII || r == '\n' || r == '\r' {
			out.WriteByte('?')
			continue
		}
		if r == '(' || r == ')' || r == '\\' {
			out.WriteByte('\\')
		}
		out.WriteRune(r)
	}
	return out.String()
}
