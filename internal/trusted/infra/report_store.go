// 可信域报告仓储实现（rpt_report，RLS 注入租户）。
package infra

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// ReportStore 报告仓储实现。
type ReportStore struct {
	db  *db.DB
	gen *id.Generator
}

// NewReportStore 创建报告仓储。
func NewReportStore(db *db.DB, gen *id.Generator) *ReportStore {
	return &ReportStore{db: db, gen: gen}
}

// Save 保存报告。
func (s *ReportStore) Save(ctx context.Context, r *domain.Report) error {
	sm, _ := json.Marshal(r.Summary)
	ev, _ := json.Marshal(r.Evidence)
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO rpt_report(id, team_id, run_id, target_id, kind, title, version, branch, scenario,
				status, summary, evidence_ref, report_html, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			r.ID, r.TeamID, r.RunID, r.TargetID, r.Kind, r.Title, r.Version, r.Branch, r.Scenario,
			r.Status, sm, ev, r.HTML, r.CreatedAt)
		return err
	})
}

// Find 查询报告（无行→ErrReportNotFound）。
func (s *ReportStore) Find(ctx context.Context, teamID, id int64) (*domain.Report, error) {
	var r domain.Report
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var sm, ev []byte
		err := tx.QueryRow(ctx, `
			SELECT id, team_id, run_id, target_id, kind, title, version, branch, scenario,
				status, summary, evidence_ref, report_html, created_at
			FROM rpt_report WHERE team_id=$1 AND id=$2`, teamID, id).
			Scan(&r.ID, &r.TeamID, &r.RunID, &r.TargetID, &r.Kind, &r.Title, &r.Version, &r.Branch,
				&r.Scenario, &r.Status, &sm, &ev, &r.HTML, &r.CreatedAt)
		if err != nil {
			return err
		}
		_ = json.Unmarshal(sm, &r.Summary)
		_ = json.Unmarshal(ev, &r.Evidence)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrReportNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

var _ domain.ReportRepository = (*ReportStore)(nil)
