package infra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openware-io/open-green-pass/pkg/id"
)

var ErrSCMConnectionNotFound = errors.New("SCM connection not found")

// SCMConnection contains operator-visible metadata only. Its credential is
// deliberately held outside PostgreSQL in the Kubernetes Secret named by Ref.
type SCMConnection struct {
	ID                int64      `json:"id"`
	Provider          string     `json:"provider"`
	Name              string     `json:"name"`
	CredentialStatus  string     `json:"credential_status"`
	FailureSummary    string     `json:"failure_summary,omitempty"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	TokenConfigured   bool       `json:"token_configured"`
	AuthType          string     `json:"auth_type"`
	GitHubAppID       string     `json:"github_app_id,omitempty"`
	InstallationID    *int64     `json:"github_installation_id,omitempty"`
	InstallationLogin string     `json:"installation_account_login,omitempty"`
}

type SCMConnectionStore struct {
	db  *DB
	gen *id.Generator
}

func NewSCMConnectionStore(db *DB, gen *id.Generator) *SCMConnectionStore {
	return &SCMConnectionStore{db: db, gen: gen}
}

func (s *SCMConnectionStore) NextID() int64 { return s.gen.Next() }

func (s *SCMConnectionStore) List(ctx context.Context, teamID int64) ([]SCMConnection, error) {
	out := make([]SCMConnection, 0)
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,provider,name,credential_status,failure_summary,verified_at,auth_type,COALESCE(github_app_id,''),github_installation_id,installation_account_login FROM repo_scm_connection WHERE team_id=$1 ORDER BY created_at DESC,id DESC`, teamID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v SCMConnection
			if err := rows.Scan(&v.ID, &v.Provider, &v.Name, &v.CredentialStatus, &v.FailureSummary, &v.VerifiedAt, &v.AuthType, &v.GitHubAppID, &v.InstallationID, &v.InstallationLogin); err != nil {
				return err
			}
			v.TokenConfigured = true
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}

func (s *SCMConnectionStore) Create(ctx context.Context, item SCMConnection, secretRef string, actorID int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO repo_scm_connection(id,team_id,provider,name,secret_ref,credential_status,failure_summary,verified_at,created_by,auth_type,github_app_id,github_installation_id,installation_account_login) VALUES($1,current_setting('gp.team_id',true)::bigint,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, item.ID, item.Provider, item.Name, secretRef, item.CredentialStatus, item.FailureSummary, item.VerifiedAt, actorID, item.AuthType, nullIfEmpty(item.GitHubAppID), item.InstallationID, item.InstallationLogin)
		return err
	})
}

func (s *SCMConnectionStore) CredentialRef(ctx context.Context, teamID, connectionID int64) (provider, authType, ref, appID string, installationID *int64, err error) {
	err = s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT provider,auth_type,secret_ref,COALESCE(github_app_id,''),github_installation_id FROM repo_scm_connection WHERE team_id=$1 AND id=$2`, teamID, connectionID).Scan(&provider, &authType, &ref, &appID, &installationID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSCMConnectionNotFound
		}
		return err
	})
	return
}

// Delete removes a connection only after clearing repository bindings in the
// same tenant-scoped transaction. It returns the operator-visible metadata and
// the secret reference so the HTTP layer can clean up the PAT Secret.
func (s *SCMConnectionStore) Delete(ctx context.Context, teamID, connectionID int64) (SCMConnection, string, error) {
	var item SCMConnection
	var secretRef string
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT id,provider,name,credential_status,failure_summary,verified_at,auth_type,COALESCE(github_app_id,''),github_installation_id,installation_account_login,secret_ref FROM repo_scm_connection WHERE team_id=$1 AND id=$2 FOR UPDATE`, teamID, connectionID).Scan(&item.ID, &item.Provider, &item.Name, &item.CredentialStatus, &item.FailureSummary, &item.VerifiedAt, &item.AuthType, &item.GitHubAppID, &item.InstallationID, &item.InstallationLogin, &secretRef)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSCMConnectionNotFound
		}
		if err != nil {
			return err
		}
		item.TokenConfigured = true
		if _, err := tx.Exec(ctx, `UPDATE repo_repo SET scm_connection_id=NULL WHERE team_id=$1 AND scm_connection_id=$2`, teamID, connectionID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM repo_scm_connection WHERE team_id=$1 AND id=$2`, teamID, connectionID)
		return err
	})
	return item, secretRef, err
}

// BoundRepositoryURLs returns the repositories currently bound to a
// connection so verification can prove the credential still reads all of them.
func (s *SCMConnectionStore) BoundRepositoryURLs(ctx context.Context, teamID, connectionID int64) ([]string, error) {
	out := make([]string, 0)
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT r.url FROM repo_repo r WHERE r.team_id=$1 AND r.scm_connection_id=$2 AND r.deleted_at IS NULL ORDER BY r.id`, teamID, connectionID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var url string
			if err := rows.Scan(&url); err != nil {
				return err
			}
			out = append(out, url)
		}
		return rows.Err()
	})
	return out, err
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s *SCMConnectionStore) CreateAuthorizationState(ctx context.Context, teamID, actorID, installationID int64, state string) error {
	hash := authorizationStateHash(state)
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		// State is deliberately short-lived and single-use. Opportunistic cleanup
		// keeps the team-scoped table bounded even before a scheduled janitor is
		// introduced; RLS prevents this request from touching another team.
		if _, err := tx.Exec(ctx, `DELETE FROM repo_scm_authorization_state WHERE expires_at < now() OR (consumed_at IS NOT NULL AND consumed_at < now()-interval '1 day')`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO repo_scm_authorization_state(id,team_id,actor_id,installation_id,state_hash,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '10 minutes')`, s.NextID(), teamID, actorID, installationID, hash)
		return err
	})
}

func (s *SCMConnectionStore) ConsumeAuthorizationState(ctx context.Context, teamID, actorID int64, state string) (int64, error) {
	var installationID int64
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `UPDATE repo_scm_authorization_state SET consumed_at=now() WHERE team_id=$1 AND actor_id=$2 AND state_hash=$3 AND consumed_at IS NULL AND expires_at>now() RETURNING installation_id`, teamID, actorID, authorizationStateHash(state)).Scan(&installationID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrSCMConnectionNotFound
	}
	return installationID, err
}

func authorizationStateHash(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}

func (s *SCMConnectionStore) MarkVerification(ctx context.Context, teamID, connectionID int64, status, failure string) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE repo_scm_connection SET credential_status=$3,failure_summary=$4,verified_at=CASE WHEN $3='verified' THEN now() ELSE verified_at END WHERE team_id=$1 AND id=$2`, teamID, connectionID, status, failure)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return ErrSCMConnectionNotFound
		}
		return nil
	})
}
