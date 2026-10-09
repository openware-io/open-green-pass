package infra

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/pkg/id"
)

type ProjectRepository struct {
	ID            int64  `json:"id"`
	ProjectID     int64  `json:"project_id"`
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
}
type ProjectService struct {
	ID             int64   `json:"id"`
	ProjectID      int64   `json:"project_id"`
	RepoID         int64   `json:"repo_id"`
	Name           string  `json:"name"`
	SourcePath     string  `json:"source_path"`
	Kind           string  `json:"kind"`
	BuildContext   string  `json:"build_context"`
	DockerfilePath *string `json:"dockerfile_path,omitempty"`
	ManifestPath   *string `json:"manifest_path,omitempty"`
	Status         string  `json:"status"`
}
type ProjectCatalog struct {
	db  *DB
	gen *id.Generator
}

func NewProjectCatalog(db *DB, gen *id.Generator) *ProjectCatalog {
	return &ProjectCatalog{db: db, gen: gen}
}

func (s *ProjectCatalog) AddRepository(ctx context.Context, projectID int64, url, branch string) (ProjectRepository, error) {
	var out ProjectRepository
	url = strings.TrimSpace(url)
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT id FROM tgt_target WHERE id=$1 AND level=0 AND status='active'`, projectID).Scan(&out.ProjectID); err != nil {
			return err
		}
		out.ProjectID = projectID
		out.ID = s.gen.Next()
		out.URL = url
		out.DefaultBranch = branch
		_, err := tx.Exec(ctx, `INSERT INTO repo_repo(id,project_id,kind,url,default_branch,created_by) VALUES($1,$2,'git',$3,$4,NULL)`, out.ID, projectID, url, branch)
		return err
	})
	return out, err
}
func (s *ProjectCatalog) ListRepositories(ctx context.Context, projectID int64) ([]ProjectRepository, error) {
	var out []ProjectRepository
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,project_id,url,default_branch FROM repo_repo WHERE project_id=$1 AND deleted_at IS NULL ORDER BY created_at`, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v ProjectRepository
			if err := rows.Scan(&v.ID, &v.ProjectID, &v.URL, &v.DefaultBranch); err != nil {
				return err
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}
func (s *ProjectCatalog) AddService(ctx context.Context, v ProjectService) (ProjectService, error) {
	v.ID = s.gen.Next()
	if strings.TrimSpace(v.SourcePath) == "" {
		v.SourcePath = "."
	}
	if strings.TrimSpace(v.BuildContext) == "" {
		v.BuildContext = v.SourcePath
	}
	if v.Kind == "" {
		v.Kind = "other"
	}
	v.Status = "active"
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `INSERT INTO tgt_service(id,project_id,repo_id,name,source_path,kind,build_context,dockerfile_path,manifest_path,created_by) SELECT $1,$2,r.id,$4,$5,$6,$7,$8,$9,NULL FROM repo_repo r WHERE r.id=$3 AND r.project_id=$2 RETURNING project_id`, v.ID, v.ProjectID, v.RepoID, v.Name, v.SourcePath, v.Kind, v.BuildContext, v.DockerfilePath, v.ManifestPath).Scan(&v.ProjectID)
	})
	return v, err
}
func (s *ProjectCatalog) ListServices(ctx context.Context, projectID int64) ([]ProjectService, error) {
	var out []ProjectService
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,project_id,repo_id,name,source_path,kind,build_context,dockerfile_path,manifest_path,status FROM tgt_service WHERE project_id=$1 ORDER BY repo_id,source_path`, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v ProjectService
			if err := rows.Scan(&v.ID, &v.ProjectID, &v.RepoID, &v.Name, &v.SourcePath, &v.Kind, &v.BuildContext, &v.DockerfilePath, &v.ManifestPath, &v.Status); err != nil {
				return err
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}

func (s *ProjectCatalog) UpdateServiceKind(ctx context.Context, projectID, serviceID int64, kind string) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE tgt_service SET kind=$3 WHERE id=$1 AND project_id=$2`, serviceID, projectID, kind)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
}
