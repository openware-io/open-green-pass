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
	platformdb "github.com/openware-io/open-green-pass/internal/platform/db"
)

type WechatConfig struct {
	TeamID      int64     `json:"team_id"`
	Enabled     bool      `json:"enabled"`
	AppID       string    `json:"app_id"`
	CallbackURI string    `json:"callback_uri"`
	SecretRef   string    `json:"-"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type OAuthState struct {
	TeamID, UserID int64
	Purpose        string
}
type WechatIdentity struct {
	OpenID, UnionID        string
	UserID                 int64
	DisplayName, AvatarURL string
}

type WechatStore struct {
	db   *platformdb.DB
	pool *pgxpool.Pool
}

func NewWechatStore(db *platformdb.DB, pool *pgxpool.Pool) *WechatStore {
	return &WechatStore{db: db, pool: pool}
}

func (s *WechatStore) Config(ctx context.Context, teamID int64) (WechatConfig, error) {
	var out WechatConfig
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT team_id,enabled,app_id,callback_uri,secret_ref,updated_at FROM auth_provider_config WHERE team_id=$1 AND provider='wechat'`, teamID).Scan(&out.TeamID, &out.Enabled, &out.AppID, &out.CallbackURI, &out.SecretRef, &out.UpdatedAt)
	})
	return out, err
}
func (s *WechatStore) PublicConfig(ctx context.Context, teamID int64) (WechatConfig, error) {
	var out WechatConfig
	err := s.pool.QueryRow(ctx, `SELECT team_id,enabled,app_id,callback_uri,secret_ref,updated_at FROM auth_provider_config WHERE team_id=$1 AND provider='wechat'`, teamID).Scan(&out.TeamID, &out.Enabled, &out.AppID, &out.CallbackURI, &out.SecretRef, &out.UpdatedAt)
	return out, err
}
func (s *WechatStore) SaveConfig(ctx context.Context, value WechatConfig, actorID int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO auth_provider_config(team_id,provider,enabled,app_id,callback_uri,secret_ref,updated_by) VALUES($1,'wechat',$2,$3,$4,$5,$6) ON CONFLICT(team_id,provider) DO UPDATE SET enabled=EXCLUDED.enabled,app_id=EXCLUDED.app_id,callback_uri=EXCLUDED.callback_uri,secret_ref=EXCLUDED.secret_ref,updated_by=EXCLUDED.updated_by,updated_at=now()`, value.TeamID, value.Enabled, value.AppID, value.CallbackURI, value.SecretRef, actorID)
		return err
	})
}
func (s *WechatStore) NewState(ctx context.Context, teamID, userID int64, purpose string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	_, err := s.pool.Exec(ctx, `INSERT INTO auth_oauth_state(state_hash,team_id,user_id,purpose,expires_at) VALUES($1,$2,NULLIF($3,0),$4,$5)`, hash[:], teamID, userID, purpose, time.Now().UTC().Add(10*time.Minute))
	return token, err
}
func (s *WechatStore) ConsumeState(ctx context.Context, token string) (OAuthState, error) {
	hash := sha256.Sum256([]byte(token))
	var out OAuthState
	err := s.pool.QueryRow(ctx, `UPDATE auth_oauth_state SET consumed_at=now() WHERE state_hash=$1 AND consumed_at IS NULL AND expires_at>now() RETURNING team_id,COALESCE(user_id,0),purpose`, hash[:]).Scan(&out.TeamID, &out.UserID, &out.Purpose)
	return out, err
}
func (s *WechatStore) Bind(ctx context.Context, identity WechatIdentity) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO auth_external_identity(provider,external_id,union_id,user_id,display_name,avatar_url) VALUES('wechat',$1,$2,$3,$4,$5) ON CONFLICT(provider,user_id) DO UPDATE SET external_id=EXCLUDED.external_id,union_id=EXCLUDED.union_id,display_name=EXCLUDED.display_name,avatar_url=EXCLUDED.avatar_url,updated_at=now()`, identity.OpenID, identity.UnionID, identity.UserID, identity.DisplayName, identity.AvatarURL)
	return err
}
func (s *WechatStore) IdentityForUser(ctx context.Context, userID int64) (WechatIdentity, error) {
	var out WechatIdentity
	err := s.pool.QueryRow(ctx, `SELECT external_id,union_id,user_id,display_name,avatar_url FROM auth_external_identity WHERE provider='wechat' AND user_id=$1`, userID).Scan(&out.OpenID, &out.UnionID, &out.UserID, &out.DisplayName, &out.AvatarURL)
	return out, err
}
func (s *WechatStore) UserForIdentity(ctx context.Context, openID string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx, `SELECT user_id FROM auth_external_identity WHERE provider='wechat' AND external_id=$1`, openID).Scan(&userID)
	return userID, err
}
func (s *WechatStore) Unbind(ctx context.Context, userID int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM auth_external_identity WHERE provider='wechat' AND user_id=$1`, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}
func (s *AuthStore) CreateSession(ctx context.Context, userID, teamID int64) (AuthUser, string, time.Time, error) {
	var u AuthUser
	err := s.pool.QueryRow(ctx, `SELECT u.id,u.username,u.display_name,u.email,m.team_id,m.role FROM auth_user u JOIN iam_member m ON m.principal_id=u.id::text WHERE u.id=$1 AND m.team_id=$2 AND u.status='active' AND m.status='active'`, userID, teamID).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.TeamID, &u.Role)
	if err != nil {
		return AuthUser{}, "", time.Time{}, err
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return AuthUser{}, "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	expires := time.Now().UTC().Add(12 * time.Hour)
	_, err = s.pool.Exec(ctx, `INSERT INTO auth_session(token_hash,user_id,team_id,expires_at) VALUES($1,$2,$3,$4)`, hash[:], userID, teamID, expires)
	return u, token, expires, err
}

var _ = errors.Is
