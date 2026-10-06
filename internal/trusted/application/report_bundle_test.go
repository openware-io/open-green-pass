package application

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type bundleRepo struct{ reports []*domain.Report }

func (r *bundleRepo) Save(context.Context, *domain.Report) error { return nil }
func (r *bundleRepo) Find(context.Context, int64, int64) (*domain.Report, error) {
	return nil, domain.ErrReportNotFound
}
func (r *bundleRepo) FindByRunKind(context.Context, int64, int64, string) (*domain.Report, error) {
	return nil, domain.ErrReportNotFound
}
func (r *bundleRepo) FindMany(context.Context, int64, []int64) ([]*domain.Report, error) {
	return r.reports, nil
}

func TestBuildBundleRendersSummary(t *testing.T) {
	g, _ := id.New(1, nil)
	repo := &bundleRepo{reports: []*domain.Report{{ID: 1, Scenario: "api", Version: "v1", Status: "pass"}, {ID: 2, Scenario: "web", Version: "v2", Status: "fail"}}}
	s, err := NewReportService(nil, nil, nil, repo, g)
	if err != nil {
		t.Fatal(err)
	}
	ctx := rls.WithTenant(context.Background(), 100)
	b, err := s.BuildBundle(ctx, []int64{1, 2}, "html")
	if err != nil {
		t.Fatal(err)
	}
	if b.Total != 2 || b.Pass != 1 || b.Fail != 1 || b.HTML == "" || b.Markdown == "" {
		t.Fatalf("bundle=%+v", b)
	}
}

func TestBundleRenderersProduceDownloadableArtifacts(t *testing.T) {
	b := &Bundle{Markdown: "# GreenPass\n\n- pass: 1"}
	pdf, contentType, extension, err := b.RenderBundle("pdf")
	if err != nil || contentType != "application/pdf" || extension != "pdf" || !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatalf("pdf type=%s extension=%s err=%v prefix=%q", contentType, extension, err, pdf[:min(8, len(pdf))])
	}
	docx, contentType, extension, err := b.RenderBundle("docx")
	if err != nil || contentType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" || extension != "docx" {
		t.Fatalf("docx type=%s extension=%s err=%v", contentType, extension, err)
	}
	reader, err := zip.NewReader(bytes.NewReader(docx), int64(len(docx)))
	if err != nil {
		t.Fatal(err)
	}
	var document string
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		var data bytes.Buffer
		_, _ = data.ReadFrom(rc)
		_ = rc.Close()
		document = data.String()
	}
	if !strings.Contains(document, "GreenPass") {
		t.Fatalf("document.xml=%q", document)
	}
}

func TestBuildBundleRejectsUnknownFormat(t *testing.T) {
	g, _ := id.New(1, nil)
	repo := &bundleRepo{reports: []*domain.Report{{ID: 1, Scenario: "api", Version: "v1", Status: "pass"}}}
	s, err := NewReportService(nil, nil, nil, repo, g)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.BuildBundle(rls.WithTenant(context.Background(), 100), []int64{1}, "xlsx")
	if !errors.Is(err, ErrUnsupportedBundleFormat) {
		t.Fatalf("err=%v, want unsupported format", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
