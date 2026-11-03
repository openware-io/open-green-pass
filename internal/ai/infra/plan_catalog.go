package infra

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/platform/db"
)

// GenerationPlanCatalog is the read-only governance view used by preflight. It
// never writes catalog tables and it never calls SCM; refs are served from the
// server-refreshed repo_ref cache.
type GenerationPlanCatalog struct {
	db *db.DB
}

func NewGenerationPlanCatalog(database *db.DB) *GenerationPlanCatalog {
	return &GenerationPlanCatalog{db: database}
}

var _ domain.GenerationPlanCatalogPort = (*GenerationPlanCatalog)(nil)

func (c *GenerationPlanCatalog) ReadGenerationScope(ctx context.Context, teamID, projectID, serviceID int64) (*domain.GenerationScope, error) {
	var out *domain.GenerationScope
	err := c.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var err error
		out, err = readGenerationScope(ctx, tx, teamID, projectID, serviceID)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGenerationNotFound
	}
	return out, err
}

func (c *GenerationPlanCatalog) ReadRepositoryReadiness(ctx context.Context, teamID, projectID, repoID int64) (*domain.GenerationRepositoryReadiness, error) {
	var out domain.GenerationRepositoryReadiness
	err := c.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT r.url, COALESCE(c.credential_status,'')
			FROM repo_repo r
			LEFT JOIN repo_scm_connection c ON c.id=r.scm_connection_id AND c.team_id=r.team_id
			WHERE r.team_id=$1 AND r.project_id=$2 AND r.id=$3 AND r.deleted_at IS NULL`,
			teamID, projectID, repoID).Scan(&out.RepoURL, &out.SCMStatus)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGenerationRepositoryNotFound
	}
	if err != nil {
		return nil, err
	}
	switch out.SCMStatus {
	case "configured", "verified":
		out.SCMConfigured = true
	}
	return &out, nil
}

func (c *GenerationPlanCatalog) ReadRepositoryRef(ctx context.Context, teamID, repoID int64, refType, name string) (*domain.GenerationRepositoryRef, error) {
	var out domain.GenerationRepositoryRef
	err := c.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT ref_type,name,commit_sha,fetched_at FROM repo_ref
			WHERE team_id=$1 AND repo_id=$2 AND ref_type=$3 AND name=$4`,
			teamID, repoID, refType, name).Scan(&out.RefType, &out.Name, &out.CommitSHA, &out.FetchedAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGenerationRepositoryRefNotFound
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *GenerationPlanCatalog) FindLatestApprovedBaseline(ctx context.Context, teamID, projectID, repoID, serviceID int64) (*int64, error) {
	var baseline *int64
	err := c.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var id int64
		err := tx.QueryRow(ctx, `
			SELECT id FROM gen_batch
			WHERE team_id=$1 AND target_id=$2 AND repo_id=$3 AND service_id=$4 AND state='approved'
			ORDER BY updated_at DESC,id DESC LIMIT 1`, teamID, projectID, repoID, serviceID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		baseline = &id
		return nil
	})
	return baseline, err
}

// ReadServiceUsage aggregates the most recent approved batches of one service.
// It returns a zero sample (SampleCount=0) when no approved history exists.
func (c *GenerationPlanCatalog) ReadServiceUsage(ctx context.Context, teamID, projectID, repoID, serviceID int64) (*domain.GenerationUsageSample, error) {
	return c.readUsage(ctx, teamID, projectID, &repoID, &serviceID)
}

// ReadFallbackUsage aggregates approved batches across the project. It only
// supplies a low-confidence fallback when the service has no history.
func (c *GenerationPlanCatalog) ReadFallbackUsage(ctx context.Context, teamID, projectID int64) (*domain.GenerationUsageSample, error) {
	return c.readUsage(ctx, teamID, projectID, nil, nil)
}

func (c *GenerationPlanCatalog) readUsage(ctx context.Context, teamID, projectID int64, repoID, serviceID *int64) (*domain.GenerationUsageSample, error) {
	var out domain.GenerationUsageSample
	err := c.db.WithTenant(ctx, func(tx pgx.Tx) error {
		scope := `b.team_id=$1 AND b.target_id=$2 AND b.state='approved'`
		args := []any{teamID, projectID}
		if repoID != nil {
			args = append(args, *repoID)
			scope += ` AND b.repo_id=$3`
		}
		if serviceID != nil {
			args = append(args, *serviceID)
			scope += ` AND b.service_id=$4`
		}
		var avgTokensIn, avgTokensOut, avgDuration float64
		var sampleCount int64
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*),COALESCE(AVG(s.tokens_in),0),COALESCE(AVG(s.tokens_out),0),
				COALESCE(AVG(EXTRACT(EPOCH FROM (s.updated_at-s.created_at))),0)
			FROM (
				SELECT b.tokens_in,b.tokens_out,b.created_at,b.updated_at
				FROM gen_batch b WHERE `+scope+`
				ORDER BY b.updated_at DESC LIMIT 10
			) s`, args...).Scan(&sampleCount, &avgTokensIn, &avgTokensOut, &avgDuration); err != nil {
			return err
		}
		var candidatesMin, candidatesMax int64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(MIN(t.cnt),0),COALESCE(MAX(t.cnt),0) FROM (
				SELECT COUNT(*) AS cnt FROM gen_seed g JOIN gen_batch b ON b.id=g.batch_id
				WHERE `+scope+` GROUP BY b.id ORDER BY b.id DESC LIMIT 10
			) t`, args...).Scan(&candidatesMin, &candidatesMax); err != nil {
			return err
		}
		out.SampleCount = int(sampleCount)
		out.TokensIn = int64(avgTokensIn)
		out.TokensOut = int64(avgTokensOut)
		out.DurationSeconds = int(avgDuration)
		out.CandidatesMin = int(candidatesMin)
		out.CandidatesMax = int(candidatesMax)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ReadGenerationModel reads tgt_target.model_binding. The JSON keys are
// capitalized in production data ({"Provider":..,"Model":..}), so decoding is
// case-insensitive; provider and model are returned separately because the
// billing key derivation belongs to preflight, not storage.
func (c *GenerationPlanCatalog) ReadGenerationModel(ctx context.Context, teamID, projectID int64) (domain.GenerationModelBinding, error) {
	var binding domain.GenerationModelBinding
	err := c.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT COALESCE(model_binding,'{}'::jsonb) FROM tgt_target WHERE team_id=$1 AND id=$2`, teamID, projectID).Scan(&raw); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		var selected struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
		}
		if json.Unmarshal(raw, &selected) != nil {
			return nil
		}
		binding.Provider = strings.TrimSpace(selected.Provider)
		binding.Model = strings.TrimSpace(selected.Model)
		return nil
	})
	return binding, err
}
