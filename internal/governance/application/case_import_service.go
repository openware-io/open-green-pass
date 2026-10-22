// Package application 用例导入：把 DSH 生成 Job 的结构化输出写入 cas_case/cas_version，并回填 cov_cell.case_id。
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

// GenerationOutputReader 读取生成 Job 的终态输出（AI 域实现）。
type GenerationOutputReader interface {
	ReadOutput(ctx context.Context, teamID, jobID int64) (state, output string, err error)
}

// CaseImportService 导入 DSH 生成用例。
type CaseImportService struct {
	cases    *CaseService
	coverage domain.CoverageRepository
	reader   GenerationOutputReader
}

// NewCaseImportService creates the importer.
func NewCaseImportService(cases *CaseService, coverage domain.CoverageRepository, reader GenerationOutputReader) *CaseImportService {
	return &CaseImportService{cases: cases, coverage: coverage, reader: reader}
}

// GeneratedCase is the JSON contract DSH must emit.
type GeneratedCase struct {
	Code            string          `json:"code"`
	Title           string          `json:"title"`
	Kind            string          `json:"kind"`
	Script          json.RawMessage `json:"script"`
	CoverageCellIDs []int64         `json:"coverage_cell_ids"`
}

// Import parses the job output and creates cases; coverage cells are assigned
// to the new case id. It is intentionally a separate, explicit step so a
// malformed AI output never writes cases automatically.
func (s *CaseImportService) Import(ctx context.Context, jobID, targetID int64) ([]*domain.Case, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	if jobID <= 0 || targetID <= 0 {
		return nil, gperr.Validation("job_id and target_id required")
	}
	state, output, err := s.reader.ReadOutput(ctx, teamID, jobID)
	if err != nil {
		return nil, err
	}
	if state != "succeeded" {
		return nil, gperr.Conflict("generation job is not succeeded")
	}
	var generated []GeneratedCase
	if err := json.Unmarshal(extractCaseArray(output), &generated); err != nil {
		return nil, gperr.Validation("generation output is not a valid case array")
	}
	if len(generated) == 0 {
		return nil, gperr.Validation("generation output contains no cases")
	}
	out := make([]*domain.Case, 0, len(generated))
	for _, item := range generated {
		if strings.TrimSpace(item.Code) == "" || strings.TrimSpace(item.Title) == "" {
			return nil, gperr.Validation("generated case code/title required")
		}
		script := item.Script
		if len(script) == 0 {
			script = json.RawMessage(`{}`)
		}
		c, err := s.cases.CreateCase(ctx, domain.Case{
			TargetID: targetID, Code: item.Code, Title: item.Title, Kind: item.Kind,
		}, script)
		if err != nil {
			return nil, fmt.Errorf("create case %s: %w", item.Code, err)
		}
		if err := s.coverage.AssignCase(ctx, teamID, item.CoverageCellIDs, c.ID); err != nil {
			return nil, fmt.Errorf("assign coverage for %s: %w", item.Code, err)
		}
		out = append(out, c)
	}
	return out, nil
}

func extractCaseArray(output string) []byte {
	raw := strings.TrimSpace(output)
	if strings.HasPrefix(raw, "```") {
		if i := strings.Index(raw, "\n"); i >= 0 {
			raw = raw[i+1:]
		}
		raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
	}
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	return []byte(raw)
}
