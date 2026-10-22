// Package infra 治理域基础设施：覆盖账本仓储（ctr_endpoint/cov_cell，RLS）。
package infra

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// CoverageStore 覆盖账本仓储。
type CoverageStore struct {
	db  *DB
	gen *id.Generator
}

// NewCoverageStore 创建覆盖账本仓储。
func NewCoverageStore(db *DB, gen *id.Generator) *CoverageStore {
	return &CoverageStore{db: db, gen: gen}
}

// SaveEndpoints upsert 契约端点并返回带 ID 的端点。
func (s *CoverageStore) SaveEndpoints(ctx context.Context, teamID int64, endpoints []domain.Endpoint) ([]domain.Endpoint, error) {
	out := make([]domain.Endpoint, 0, len(endpoints))
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		for _, ep := range endpoints {
			id := ep.ID
			if id == 0 {
				id = s.gen.Next()
			}
			var got int64
			err := tx.QueryRow(ctx, `
				INSERT INTO ctr_endpoint(
					id, team_id, project_id, service_id, module, method, path, operation_id,
					auth_mode, source_file, source_line)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
				ON CONFLICT (team_id, project_id, method, path, version) DO UPDATE SET
					service_id=EXCLUDED.service_id, module=EXCLUDED.module,
					operation_id=EXCLUDED.operation_id, auth_mode=EXCLUDED.auth_mode,
					source_file=EXCLUDED.source_file, source_line=EXCLUDED.source_line,
					updated_at=now()
				RETURNING id`,
				id, teamID, ep.ProjectID, ep.ServiceID, ep.Module, ep.Method, ep.Path,
				ep.OperationID, ep.AuthMode, ep.SourceFile, ep.SourceLine).Scan(&got)
			if err != nil {
				return err
			}
			ep.ID = got
			ep.TeamID = teamID
			out = append(out, ep)
		}
		return nil
	})
	return out, err
}

// SaveCells upsert Coverage Cell；已生成的 case/status 不会被重跑覆盖。
func (s *CoverageStore) SaveCells(ctx context.Context, teamID int64, cells []domain.CoverageCell) ([]domain.CoverageCell, error) {
	out := make([]domain.CoverageCell, 0, len(cells))
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		for _, cell := range cells {
			id := cell.ID
			if id == 0 {
				id = s.gen.Next()
			}
			var endpointID *int64
			if cell.EndpointID > 0 {
				endpointID = &cell.EndpointID
			}
			var got int64
			err := tx.QueryRow(ctx, `
				INSERT INTO cov_cell(
					id, team_id, project_id, service_id, endpoint_id, endpoint_hash, method, path, module,
					dimension, equivalence_class, role, state, required, priority, status)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
				ON CONFLICT (team_id, endpoint_hash, dimension, equivalence_class, role, state)
				DO UPDATE SET
					project_id=EXCLUDED.project_id, service_id=EXCLUDED.service_id,
					endpoint_id=EXCLUDED.endpoint_id, method=EXCLUDED.method, path=EXCLUDED.path,
					required=EXCLUDED.required, priority=EXCLUDED.priority, updated_at=now()
				RETURNING id`,
				id, teamID, cell.ProjectID, cell.ServiceID, endpointID, cell.EndpointHash,
				cell.Method, cell.Path, cell.Module, cell.Dimension, cell.EquivalenceClass, cell.Role,
				cell.State, cell.Required, cell.Priority, cell.Status).Scan(&got)
			if err != nil {
				return err
			}
			cell.ID = got
			cell.TeamID = teamID
			out = append(out, cell)
		}
		return nil
	})
	return out, err
}

// ListCells 查询覆盖账本。
func (s *CoverageStore) ListCells(ctx context.Context, teamID int64, f domain.CoverageFilter) ([]domain.CoverageCell, error) {
	where := "team_id=$1"
	args := []any{teamID}
	if f.ProjectID > 0 {
		args = append(args, f.ProjectID)
		where += " AND project_id=$" + itoa(len(args))
	}
	if f.ServiceID != nil {
		args = append(args, *f.ServiceID)
		where += " AND service_id=$" + itoa(len(args))
	}
	if f.Module != "" {
		args = append(args, f.Module)
		where += " AND module=$" + itoa(len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where += " AND status=$" + itoa(len(args))
	}
	if f.Required != nil {
		args = append(args, *f.Required)
		where += " AND required=$" + itoa(len(args))
	}
	var out []domain.CoverageCell
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, team_id, project_id, service_id, endpoint_id, endpoint_hash, method, path, module,
				dimension, equivalence_class, role, state, required, priority, status, case_id, waiver_id
			FROM cov_cell WHERE `+where+` ORDER BY method, path, dimension, equivalence_class`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.CoverageCell
			if err := rows.Scan(&c.ID, &c.TeamID, &c.ProjectID, &c.ServiceID, &c.EndpointID,
				&c.EndpointHash, &c.Method, &c.Path, &c.Module, &c.Dimension, &c.EquivalenceClass,
				&c.Role, &c.State, &c.Required, &c.Priority, &c.Status, &c.CaseID, &c.WaiverID); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Summary 计算覆盖汇总（Design/Execution/Pass 分开）。
func (s *CoverageStore) Summary(ctx context.Context, teamID int64, f domain.CoverageFilter) (domain.CoverageSummary, error) {
	cells, err := s.ListCells(ctx, teamID, f)
	if err != nil {
		return domain.CoverageSummary{}, err
	}
	var sum domain.CoverageSummary
	for _, c := range cells {
		if !c.Required {
			continue
		}
		sum.TotalRequired++
		if c.WaiverID != nil || c.Status == "waived" {
			sum.Waived++
			continue
		}
		if c.CaseID != nil || (c.Status != "planned" && c.Status != "blocked") {
			sum.DesignCovered++
		}
		if c.Status == "executed" || c.Status == "passed" {
			sum.Executed++
		}
		if c.Status == "passed" {
			sum.Passed++
		}
	}
	sum.Uncovered = sum.TotalRequired - sum.DesignCovered - sum.Waived
	if sum.Uncovered < 0 {
		sum.Uncovered = 0
	}
	return sum, nil
}
