// Package infra 治理域基础设施：缺陷仓储（def_finding/def_event，RLS 租户经 DB.WithTenant 注入）。
package infra

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
)

// FindingStore 缺陷仓储实现。
type FindingStore struct {
	db *DB
}

// NewFindingStore 创建缺陷仓储。
func NewFindingStore(db *DB) *FindingStore { return &FindingStore{db: db} }

// Create 写入 finding + 首个 event；唯一键命中时 created=false。
func (s *FindingStore) Create(ctx context.Context, f *domain.Finding, event *domain.FindingEvent) (bool, error) {
	created := false
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		evidence, err := json.Marshal(f.Evidence)
		if err != nil {
			return err
		}
		reproduction, err := json.Marshal(f.Reproduction)
		if err != nil {
			return err
		}
		sourceRef, err := json.Marshal(f.SourceRef)
		if err != nil {
			return err
		}
		coverage, err := json.Marshal(f.CoverageCells)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO def_finding(
				id, team_id, project_id, service_id, endpoint_id, target_id, repository_id,
				repo_name, repo_url, branch, commit_sha, module, endpoint, title, description,
				severity, category, status, expected, actual, root_cause, impact, confidence,
				reproduction, source_ref, evidence, coverage_cells, owner_id, linked_run_id,
				linked_case_id, dedupe_hash, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
				$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33)
			ON CONFLICT (team_id, dedupe_hash) DO NOTHING`,
			f.ID, f.TeamID, f.ProjectID, f.ServiceID, f.EndpointID, f.TargetID, f.Repository.ID,
			f.Repository.Name, f.Repository.URL, f.Repository.Branch, f.Repository.CommitSHA,
			f.Module, f.Endpoint, f.Title, f.Description, f.Severity, f.Category, f.Status,
			f.Expected, f.Actual, f.RootCause, f.Impact, f.Confidence, reproduction, sourceRef,
			evidence, coverage, f.OwnerID, f.LinkedRunID, f.LinkedCaseID, f.DedupeHash,
			f.CreatedBy, f.UpdatedBy)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		created = true
		if event == nil {
			return nil
		}
		if event.Payload == nil {
			event.Payload = map[string]any{}
		}
		event.TeamID = f.TeamID
		event.FindingID = f.ID
		_, err = tx.Exec(ctx, `
			INSERT INTO def_event(id, team_id, finding_id, action, from_status, to_status, actor_id, reason, payload)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			event.ID, event.TeamID, event.FindingID, event.Action, event.FromStatus, event.ToStatus,
			event.ActorID, event.Reason, event.Payload)
		return err
	})
	return created, err
}

// Find 按租户/ID 查询缺陷（含首次证据摘要）。
func (s *FindingStore) Find(ctx context.Context, teamID, id int64) (*domain.Finding, error) {
	var f domain.Finding
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, findingSelect+` WHERE team_id=$1 AND id=$2`, teamID, id)
		return scanFinding(row, &f)
	})
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// FindByDedupe 按去重键查询。
func (s *FindingStore) FindByDedupe(ctx context.Context, teamID int64, hash string) (*domain.Finding, error) {
	var f domain.Finding
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, findingSelect+` WHERE team_id=$1 AND dedupe_hash=$2`, teamID, hash)
		return scanFinding(row, &f)
	})
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// List 列出缺陷（筛选 + 分页）。
func (s *FindingStore) List(ctx context.Context, teamID int64, filter domain.FindingFilter) ([]*domain.Finding, error) {
	where := "team_id=$1"
	args := []any{teamID}
	add := func(clause string, value any) {
		args = append(args, value)
		where += " AND " + clause + "=$" + itoa(len(args))
	}
	if filter.Severity != "" {
		add("severity", filter.Severity)
	}
	if filter.Status != "" {
		add("status", filter.Status)
	}
	if filter.Module != "" {
		add("module", filter.Module)
	}
	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		where += " AND (title ILIKE $" + itoa(len(args)) + " OR endpoint ILIKE $" + itoa(len(args)) + " OR module ILIKE $" + itoa(len(args)) + ")"
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args = append(args, limit, filter.Offset)
	query := findingSelect + ` WHERE ` + where + ` ORDER BY updated_at DESC, id DESC LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	var out []*domain.Finding
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f domain.Finding
			if err := scanFinding(rows, &f); err != nil {
				return err
			}
			out = append(out, &f)
		}
		return rows.Err()
	})
	return out, err
}

// UpdateStatus 更新状态并追加事件（同一事务）。
func (s *FindingStore) UpdateStatus(ctx context.Context, f *domain.Finding, event *domain.FindingEvent) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE def_finding SET status=$1, updated_by=$2, updated_at=$3
			WHERE team_id=$4 AND id=$5`,
			f.Status, f.UpdatedBy, f.UpdatedAt, f.TeamID, f.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		if event.Payload == nil {
			event.Payload = map[string]any{}
		}
		event.TeamID = f.TeamID
		event.FindingID = f.ID
		_, err = tx.Exec(ctx, `
			INSERT INTO def_event(id, team_id, finding_id, action, from_status, to_status, actor_id, reason, payload)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			event.ID, event.TeamID, event.FindingID, event.Action, event.FromStatus, event.ToStatus,
			event.ActorID, event.Reason, event.Payload)
		return err
	})
}

// Events 查询流转记录（升序）。
func (s *FindingStore) Events(ctx context.Context, teamID, findingID int64) ([]domain.FindingEvent, error) {
	var out []domain.FindingEvent
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, team_id, finding_id, action, COALESCE(from_status,''), COALESCE(to_status,''),
				actor_id, reason, payload, created_at
			FROM def_event WHERE team_id=$1 AND finding_id=$2 ORDER BY created_at, id`, teamID, findingID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e domain.FindingEvent
			var payload []byte
			if err := rows.Scan(&e.ID, &e.TeamID, &e.FindingID, &e.Action, &e.FromStatus, &e.ToStatus,
				&e.ActorID, &e.Reason, &payload, &e.CreatedAt); err != nil {
				return err
			}
			if len(payload) > 0 {
				_ = json.Unmarshal(payload, &e.Payload)
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

const findingSelect = `SELECT id, team_id, project_id, service_id, endpoint_id, target_id, repository_id,
	repo_name, repo_url, branch, commit_sha, module, endpoint, title, description,
	severity, category, status, expected, actual, root_cause, impact, confidence,
	reproduction, source_ref, evidence, coverage_cells, owner_id, linked_run_id,
	linked_case_id, dedupe_hash, created_by, created_at, updated_by, updated_at
	FROM def_finding`

func scanFinding(row pgx.Row, f *domain.Finding) error {
	var (
		repoID, serviceID, endpointID, targetID, ownerID, runID, caseID *int64
		reproduction, sourceRef, evidence, coverage                     []byte
		createdAt, updatedAt                                            time.Time
	)
	if err := row.Scan(&f.ID, &f.TeamID, &f.ProjectID, &serviceID, &endpointID, &targetID, &repoID,
		&f.Repository.Name, &f.Repository.URL, &f.Repository.Branch, &f.Repository.CommitSHA,
		&f.Module, &f.Endpoint, &f.Title, &f.Description, &f.Severity, &f.Category, &f.Status,
		&f.Expected, &f.Actual, &f.RootCause, &f.Impact, &f.Confidence, &reproduction, &sourceRef,
		&evidence, &coverage, &ownerID, &runID, &caseID, &f.DedupeHash, &f.CreatedBy, &createdAt,
		&f.UpdatedBy, &updatedAt); err != nil {
		return err
	}
	f.Repository.ID = repoID
	f.ServiceID = serviceID
	f.EndpointID = endpointID
	f.TargetID = targetID
	f.OwnerID = ownerID
	f.LinkedRunID = runID
	f.LinkedCaseID = caseID
	_ = json.Unmarshal(reproduction, &f.Reproduction)
	_ = json.Unmarshal(sourceRef, &f.SourceRef)
	_ = json.Unmarshal(evidence, &f.Evidence)
	_ = json.Unmarshal(coverage, &f.CoverageCells)
	f.CreatedAt = createdAt
	f.UpdatedAt = updatedAt
	return nil
}
