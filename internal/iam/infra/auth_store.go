package infra

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/iam"
)

const SessionCookie = "gp_session"

type AuthUser struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	TeamID      int64  `json:"team_id"`
	Role        string `json:"role"`
}

type AuthStore struct{ pool *pgxpool.Pool }

func NewAuthStore(pool *pgxpool.Pool) *AuthStore { return &AuthStore{pool: pool} }

func (s *AuthStore) Login(ctx context.Context, username, password string) (AuthUser, string, time.Time, error) {
	var u AuthUser
	err := s.pool.QueryRow(ctx, `SELECT u.id,u.username,u.display_name,u.email,m.team_id,m.role
FROM auth_user u JOIN iam_member m ON m.principal_id=u.id::text
WHERE u.username=lower(btrim($1)) AND u.status='active' AND m.status='active'
AND u.password_hash=crypt($2,u.password_hash) ORDER BY m.team_id LIMIT 1`, username, password).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.TeamID, &u.Role)
	if err != nil {
		return AuthUser{}, "", time.Time{}, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return AuthUser{}, "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	expires := time.Now().UTC().Add(12 * time.Hour)
	_, err = s.pool.Exec(ctx, `INSERT INTO auth_session(token_hash,user_id,team_id,expires_at) VALUES($1,$2,$3,$4)`, hash[:], u.ID, u.TeamID, expires)
	return u, token, expires, err
}

func (s *AuthStore) Authenticate(ctx context.Context, credential string) iam.AuthenticationResult {
	hash := sha256.Sum256([]byte(credential))
	var p iam.Principal
	var role string
	err := s.pool.QueryRow(ctx, `UPDATE auth_session s SET last_seen_at=now()
FROM auth_user u, iam_member m WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now()
AND u.id=s.user_id AND u.status='active' AND m.team_id=s.team_id AND m.principal_id=u.id::text AND m.status='active'
RETURNING u.id::text,s.team_id::text,m.role,s.expires_at`, hash[:]).Scan(&p.Subject, &p.TenantID, &role, &p.ExpiresAt)
	if err != nil {
		return iam.AuthenticationResult{Err: iam.ErrUnauthenticated}
	}
	p.Issuer = "gp-session"
	p.Roles = []string{role}
	return iam.AuthenticationResult{Principal: &p}
}

func (s *AuthStore) Current(ctx context.Context, principal iam.Principal) (AuthUser, error) {
	var u AuthUser
	err := s.pool.QueryRow(ctx, `SELECT u.id,u.username,u.display_name,u.email,m.team_id,m.role FROM auth_user u JOIN iam_member m ON m.principal_id=u.id::text WHERE u.id::text=$1 AND m.team_id::text=$2 AND u.status='active' AND m.status='active'`, principal.Subject, principal.TenantID).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.TeamID, &u.Role)
	return u, err
}

func (s *AuthStore) Logout(ctx context.Context, token string) error {
	if token == "" {
		return errors.New("missing session")
	}
	hash := sha256.Sum256([]byte(token))
	tag, err := s.pool.Exec(ctx, `UPDATE auth_session SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, hash[:])
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
