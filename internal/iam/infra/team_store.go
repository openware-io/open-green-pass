package infra

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/iam"
	platformdb "github.com/openware-io/open-green-pass/internal/platform/db"
)

type TeamSummary struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	MemberCount  int64  `json:"member_count"`
	ProjectCount int64  `json:"project_count"`
}

type TeamMember struct {
	ID           int64            `json:"id"`
	PrincipalID  string           `json:"principal_id"`
	Username     string           `json:"username"`
	DisplayName  string           `json:"display_name"`
	Email        string           `json:"email"`
	Role         iam.Role         `json:"role"`
	Status       iam.MemberStatus `json:"status"`
	LastActiveAt *time.Time       `json:"last_active_at,omitempty"`
}

type TeamAsset struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	OwnerID   *int64 `json:"owner_id,omitempty"`
	OwnerName string `json:"owner_name"`
}

type AssetGrant struct {
	MemberID        int64          `json:"member_id"`
	DisplayName     string         `json:"display_name"`
	Email           string         `json:"email"`
	Role            iam.Role       `json:"role"`
	Permission      iam.Permission `json:"permission"`
	ConcurrentQuota int64          `json:"concurrent_quota"`
	SandboxQuota    int64          `json:"sandbox_quota"`
	Version         int64          `json:"version"`
}

type TeamStore struct{ db *platformdb.DB }

func NewTeamStore(db *platformdb.DB) *TeamStore { return &TeamStore{db: db} }

func (s *TeamStore) Summary(ctx context.Context, teamID int64) (TeamSummary, error) {
	out := TeamSummary{ID: teamID, Name: "GreenPass 团队"}
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='active'), (SELECT count(*) FROM tgt_target WHERE level=0 AND status='active' AND deleted_at IS NULL) FROM iam_member`).Scan(&out.MemberCount, &out.ProjectCount)
	})
	return out, err
}

func (s *TeamStore) Members(ctx context.Context, teamID int64) ([]TeamMember, error) {
	out := []TeamMember{}
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT m.id,m.principal_id,COALESCE(u.username,''),COALESCE(u.display_name,m.principal_id),COALESCE(u.email,''),m.role,m.status,m.last_active_at FROM iam_member m LEFT JOIN auth_user u ON u.id::text=m.principal_id WHERE m.team_id=$1 ORDER BY m.created_at,m.id`, teamID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var member TeamMember
			if err := rows.Scan(&member.ID, &member.PrincipalID, &member.Username, &member.DisplayName, &member.Email, &member.Role, &member.Status, &member.LastActiveAt); err != nil {
				return err
			}
			out = append(out, member)
		}
		return rows.Err()
	})
	return out, err
}

func (s *TeamStore) Assets(ctx context.Context, teamID int64) ([]TeamAsset, error) {
	out := []TeamAsset{}
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT t.id,t.name,t.kind,o.member_id,COALESCE(u.display_name,'') FROM tgt_target t LEFT JOIN iam_asset_owner o ON o.team_id=t.team_id AND o.target_id=t.id LEFT JOIN iam_member m ON m.id=o.member_id LEFT JOIN auth_user u ON u.id::text=m.principal_id WHERE t.team_id=$1 AND t.status='active' AND t.deleted_at IS NULL ORDER BY t.level,t.name`, teamID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var asset TeamAsset
			if err := rows.Scan(&asset.ID, &asset.Name, &asset.Kind, &asset.OwnerID, &asset.OwnerName); err != nil {
				return err
			}
			out = append(out, asset)
		}
		return rows.Err()
	})
	return out, err
}

func (s *TeamStore) AssetGrants(ctx context.Context, teamID, targetID int64) ([]AssetGrant, error) {
	out := []AssetGrant{}
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT m.id,COALESCE(u.display_name,m.principal_id),COALESCE(u.email,''),m.role,COALESCE(p.permission,'none'),COALESCE(p.concurrent_quota,0),COALESCE(p.sandbox_quota,0),COALESCE(p.version,0) FROM iam_member m LEFT JOIN auth_user u ON u.id::text=m.principal_id LEFT JOIN iam_asset_perm p ON p.team_id=m.team_id AND p.member_id=m.id AND p.target_id=$2 WHERE m.team_id=$1 AND m.status='active' ORDER BY m.id`, teamID, targetID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var grant AssetGrant
			if err := rows.Scan(&grant.MemberID, &grant.DisplayName, &grant.Email, &grant.Role, &grant.Permission, &grant.ConcurrentQuota, &grant.SandboxQuota, &grant.Version); err != nil {
				return err
			}
			out = append(out, grant)
		}
		return rows.Err()
	})
	return out, err
}
