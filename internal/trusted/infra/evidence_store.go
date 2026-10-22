// Package infra 证据元数据仓储（evd_evidence，Trusted writer）。
package infra

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// EvidenceStore 证据元数据仓储。
type EvidenceStore struct {
	db  *db.DB
	gen *id.Generator
}

// NewEvidenceStore 创建仓储。
func NewEvidenceStore(database *db.DB, gen *id.Generator) *EvidenceStore {
	return &EvidenceStore{db: database, gen: gen}
}

// Create 写入不可变证据元数据。
func (s *EvidenceStore) Create(ctx context.Context, e *domain.Evidence) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var width, height, duration *int
		width, height, duration = e.Width, e.Height, e.DurationMS
		_, err := tx.Exec(ctx, `
			INSERT INTO evd_evidence(
				id, team_id, finding_id, kind, storage_uri, mime_type, size_bytes,
				width, height, duration_ms, sha256, thumbnail_uri, redacted, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			e.ID, e.TeamID, e.FindingID, e.Kind, e.StorageURI, e.MimeType, e.SizeBytes,
			width, height, duration, e.SHA256, e.ThumbnailURI, e.Redacted, e.CreatedBy)
		return err
	})
}

// List 列出 finding 证据。
func (s *EvidenceStore) List(ctx context.Context, teamID, findingID int64) ([]*domain.Evidence, error) {
	var out []*domain.Evidence
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, team_id, finding_id, kind, storage_uri, mime_type, size_bytes,
				width, height, duration_ms, sha256, thumbnail_uri, redacted, created_by, created_at
			FROM evd_evidence WHERE team_id=$1 AND finding_id=$2 ORDER BY created_at`, teamID, findingID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e domain.Evidence
			if err := rows.Scan(&e.ID, &e.TeamID, &e.FindingID, &e.Kind, &e.StorageURI,
				&e.MimeType, &e.SizeBytes, &e.Width, &e.Height, &e.DurationMS, &e.SHA256,
				&e.ThumbnailURI, &e.Redacted, &e.CreatedBy, &e.CreatedAt); err != nil {
				return err
			}
			out = append(out, &e)
		}
		return rows.Err()
	})
	return out, err
}

// Find 查询单条证据。
func (s *EvidenceStore) Find(ctx context.Context, teamID, findingID, id int64) (*domain.Evidence, error) {
	var e domain.Evidence
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id, team_id, finding_id, kind, storage_uri, mime_type, size_bytes,
				width, height, duration_ms, sha256, thumbnail_uri, redacted, created_by, created_at
			FROM evd_evidence WHERE team_id=$1 AND finding_id=$2 AND id=$3`,
			teamID, findingID, id).Scan(&e.ID, &e.TeamID, &e.FindingID, &e.Kind, &e.StorageURI,
			&e.MimeType, &e.SizeBytes, &e.Width, &e.Height, &e.DurationMS, &e.SHA256,
			&e.ThumbnailURI, &e.Redacted, &e.CreatedBy, &e.CreatedAt)
	})
	if err != nil {
		return nil, err
	}
	return &e, nil
}
