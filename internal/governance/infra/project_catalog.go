package infra

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/pkg/id"
)

type ProjectRepository struct {
	ID                  int64  `json:"id"`
	ProjectID           int64  `json:"project_id"`
	URL                 string `json:"url"`
	DefaultBranch       string `json:"default_branch"`
	SCMConnectionID     *int64 `json:"scm_connection_id,omitempty"`
	SCMConnectionStatus string `json:"scm_connection_status"`
}
type ProjectService struct {
	ID                int64    `json:"id"`
	ProjectID         int64    `json:"project_id"`
	RepoID            int64    `json:"repo_id"`
	Name              string   `json:"name"`
	SourcePath        string   `json:"source_path"`
	Kind              string   `json:"kind"`
	BuildContext      string   `json:"build_context"`
	Language          string   `json:"language"`
	Framework         string   `json:"framework"`
	TestTool          string   `json:"test_tool"`
	TestCommand       string   `json:"test_command"`
	DockerfilePath    *string  `json:"dockerfile_path,omitempty"`
	ManifestPath      *string  `json:"manifest_path,omitempty"`
	Status            string   `json:"status"`
	GenerationStatus  string   `json:"generation_status"`
	ProfileStatus     string   `json:"profile_status"`
	ProfileSource     string   `json:"profile_source"`
	ProfileConfidence *float64 `json:"profile_confidence,omitempty"`
	ProfileEvidence   []any    `json:"profile_evidence,omitempty"`
	TestStrategy      string   `json:"test_strategy"`
	ProfileAnalyzedAt *string  `json:"profile_analyzed_at,omitempty"`
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
	// Keep the HTTP contract array-shaped when a project has no repositories.
	// A nil slice is encoded as JSON null, while clients consume this as a list.
	out := make([]ProjectRepository, 0)
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT r.id,r.project_id,r.url,r.default_branch,r.scm_connection_id,COALESCE(c.credential_status,'missing') FROM repo_repo r LEFT JOIN repo_scm_connection c ON c.id=r.scm_connection_id AND c.team_id=r.team_id WHERE r.project_id=$1 AND r.deleted_at IS NULL ORDER BY r.created_at`, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v ProjectRepository
			if err := rows.Scan(&v.ID, &v.ProjectID, &v.URL, &v.DefaultBranch, &v.SCMConnectionID, &v.SCMConnectionStatus); err != nil {
				return err
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}

func (s *ProjectCatalog) BindSCMConnection(ctx context.Context, projectID, repoID, connectionID int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE repo_repo r SET scm_connection_id=$3
			WHERE r.id=$2 AND r.project_id=$1 AND r.deleted_at IS NULL
			  AND EXISTS (SELECT 1 FROM repo_scm_connection c WHERE c.id=$3 AND c.team_id=r.team_id AND c.credential_status IN ('configured','verified'))`, projectID, repoID, connectionID)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
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
		return tx.QueryRow(ctx, `INSERT INTO tgt_service(id,project_id,repo_id,name,source_path,kind,build_context,language,framework,test_tool,test_command,dockerfile_path,manifest_path,created_by) SELECT $1,$2,r.id,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULL FROM repo_repo r WHERE r.id=$3 AND r.project_id=$2 RETURNING project_id`, v.ID, v.ProjectID, v.RepoID, v.Name, v.SourcePath, v.Kind, v.BuildContext, v.Language, v.Framework, v.TestTool, v.TestCommand, v.DockerfilePath, v.ManifestPath).Scan(&v.ProjectID)
	})
	return v, err
}

func (s *ProjectCatalog) ListServices(ctx context.Context, projectID int64) ([]ProjectService, error) {
	// Keep the HTTP contract array-shaped when a project has no services.
	// A nil slice is encoded as JSON null, while clients consume this as a list.
	out := make([]ProjectService, 0)
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,project_id,repo_id,name,source_path,kind,build_context,language,framework,test_tool,test_command,dockerfile_path,manifest_path,status,generation_status,profile_status,profile_source,profile_confidence,profile_evidence,test_strategy,profile_analyzed_at::text FROM tgt_service WHERE project_id=$1 ORDER BY repo_id,source_path`, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v ProjectService
			var evidence []byte
			if err := rows.Scan(&v.ID, &v.ProjectID, &v.RepoID, &v.Name, &v.SourcePath, &v.Kind, &v.BuildContext, &v.Language, &v.Framework, &v.TestTool, &v.TestCommand, &v.DockerfilePath, &v.ManifestPath, &v.Status, &v.GenerationStatus, &v.ProfileStatus, &v.ProfileSource, &v.ProfileConfidence, &evidence, &v.TestStrategy, &v.ProfileAnalyzedAt); err != nil {
				return err
			}
			if len(evidence) > 0 {
				_ = json.Unmarshal(evidence, &v.ProfileEvidence)
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}

var ErrServiceProfileIncomplete = errors.New("service technical profile must be complete and reviewed before generation is enabled")

func (s *ProjectCatalog) UpdateService(ctx context.Context, projectID, serviceID int64, kind, generationStatus, language, framework string) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return updateProjectService(ctx, tx, projectID, serviceID, kind, generationStatus, language, framework)
	})
}

func updateProjectService(ctx context.Context, tx pgx.Tx, projectID, serviceID int64, kind, generationStatus, language, framework string) error {
	command, err := tx.Exec(ctx, `UPDATE tgt_service SET
		kind=CASE WHEN $3 <> '' THEN $3 ELSE kind END,
		generation_status=CASE WHEN $4 <> '' THEN $4 ELSE generation_status END,
		language=CASE WHEN $5 <> '' THEN $5 ELSE language END,
		framework=CASE WHEN $6 <> '' THEN $6 ELSE framework END,
		profile_status=CASE WHEN $4='confirmed' THEN 'confirmed'
			WHEN ($5<>'' OR $6<>'') THEN 'suggested'
			ELSE profile_status END
		WHERE id=$1 AND project_id=$2 AND (
			$4 <> 'confirmed' OR (
				btrim(CASE WHEN $5 <> '' THEN $5 ELSE language END) <> ''
				AND btrim(CASE WHEN $6 <> '' THEN $6 ELSE framework END) <> ''
				AND profile_status IN ('suggested','confirmed')
			)
		)`, serviceID, projectID, kind, generationStatus, language, framework)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 0 {
		return nil
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tgt_service WHERE id=$1 AND project_id=$2)`, serviceID, projectID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrServiceProfileIncomplete
	}
	return pgx.ErrNoRows
}
