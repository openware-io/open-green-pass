package infra

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// ReadGenerationScope joins the catalog service to its actual repository and
// constrains both records to the same tenant and project, independently of RLS.
func (s *GenerationBatchStore) ReadGenerationScope(ctx context.Context, teamID, targetID, serviceID int64) (*domain.GenerationScope, error) {
	var scope *domain.GenerationScope
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var err error
		scope, err = readGenerationScope(ctx, tx, teamID, targetID, serviceID)
		return err
	})
	return scope, err
}

func readGenerationScope(ctx context.Context, tx pgx.Tx, teamID, targetID, serviceID int64) (*domain.GenerationScope, error) {
	var scope domain.GenerationScope
	err := tx.QueryRow(ctx, `SELECT s.id,s.team_id,s.project_id,s.repo_id,s.source_path,s.status,s.generation_status,s.profile_status,s.language,s.framework,s.test_tool,s.test_command
		FROM tgt_service s JOIN repo_repo r ON r.id=s.repo_id AND r.team_id=s.team_id AND r.project_id=s.project_id
		WHERE s.team_id=$1 AND s.project_id=$2 AND s.id=$3`, teamID, targetID, serviceID).Scan(
		&scope.ServiceID, &scope.TeamID, &scope.TargetID, &scope.RepoID, &scope.SourcePath, &scope.Status, &scope.GenerationStatus, &scope.ProfileStatus, &scope.Language, &scope.Framework, &scope.TestTool, &scope.TestCommand,
	)
	if err != nil {
		return nil, err
	}
	return &scope, nil
}
