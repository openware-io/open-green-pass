// Package infra contains PostgreSQL persistence for IAM state.
package infra

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/iam"
	platformdb "github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type RBACStore struct {
	db  *platformdb.DB
	gen *id.Generator
}

func NewRBACStore(db *platformdb.DB, gen *id.Generator) *RBACStore {
	return &RBACStore{db: db, gen: gen}
}

func (s *RBACStore) SaveMember(ctx context.Context, member *iam.Member) error {
	if err := member.Validate(); err != nil {
		return err
	}
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		if member.ID == 0 {
			member.ID = s.gen.Next()
		}
		_, err := tx.Exec(ctx, `INSERT INTO iam_member(id,team_id,principal_id,role,status,last_active_at,created_by,updated_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (team_id,principal_id) DO UPDATE SET role=EXCLUDED.role,status=EXCLUDED.status,last_active_at=EXCLUDED.last_active_at,updated_by=EXCLUDED.updated_by,updated_at=now()`,
			member.ID, member.TeamID, member.PrincipalID, member.Role, member.Status, member.LastActiveAt, member.CreatedBy, member.UpdatedBy)
		return err
	})
}

func (s *RBACStore) FindMember(ctx context.Context, teamID, memberID int64) (*iam.Member, error) {
	var out iam.Member
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id,team_id,principal_id,role,status,last_active_at,COALESCE(created_by,0),created_at,COALESCE(updated_by,0),updated_at
FROM iam_member WHERE team_id=$1 AND id=$2`, teamID, memberID).Scan(&out.ID, &out.TeamID, &out.PrincipalID, &out.Role, &out.Status, &out.LastActiveAt, &out.CreatedBy, &out.CreatedAt, &out.UpdatedBy, &out.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *RBACStore) FindMemberByPrincipal(ctx context.Context, teamID int64, principalID string) (*iam.Member, error) {
	var out iam.Member
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id,team_id,principal_id,role,status,last_active_at,COALESCE(created_by,0),created_at,COALESCE(updated_by,0),updated_at
FROM iam_member WHERE team_id=$1 AND principal_id=$2`, teamID, principalID).Scan(&out.ID, &out.TeamID, &out.PrincipalID, &out.Role, &out.Status, &out.LastActiveAt, &out.CreatedBy, &out.CreatedAt, &out.UpdatedBy, &out.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *RBACStore) ListMembers(ctx context.Context, teamID int64, status iam.MemberStatus) ([]*iam.Member, error) {
	var out []*iam.Member
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,team_id,principal_id,role,status,last_active_at,COALESCE(created_by,0),created_at,COALESCE(updated_by,0),updated_at
FROM iam_member WHERE team_id=$1 AND ($2='' OR status=$2) ORDER BY id`, teamID, status)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var m iam.Member
			if err := rows.Scan(&m.ID, &m.TeamID, &m.PrincipalID, &m.Role, &m.Status, &m.LastActiveAt, &m.CreatedBy, &m.CreatedAt, &m.UpdatedBy, &m.UpdatedAt); err != nil {
				return err
			}
			out = append(out, &m)
		}
		return rows.Err()
	})
	return out, err
}

func (s *RBACStore) SaveAssetOwner(ctx context.Context, owner *iam.AssetOwner) error {
	if owner.TeamID <= 0 || owner.TargetID <= 0 || owner.MemberID <= 0 {
		return iam.ErrInvalidPrincipal
	}
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO iam_asset_owner(team_id,target_id,member_id,assigned_at,assigned_by)
VALUES($1,$2,$3,COALESCE($4,now()),$5)
ON CONFLICT (team_id,target_id) DO UPDATE SET member_id=EXCLUDED.member_id,assigned_at=EXCLUDED.assigned_at,assigned_by=EXCLUDED.assigned_by`, owner.TeamID, owner.TargetID, owner.MemberID, owner.AssignedAt, owner.AssignedBy)
		return err
	})
}

func (s *RBACStore) FindAssetOwner(ctx context.Context, teamID, targetID int64) (*iam.AssetOwner, error) {
	var out iam.AssetOwner
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT team_id,target_id,member_id,assigned_at,assigned_by FROM iam_asset_owner WHERE team_id=$1 AND target_id=$2`, teamID, targetID).Scan(&out.TeamID, &out.TargetID, &out.MemberID, &out.AssignedAt, &out.AssignedBy)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *RBACStore) SaveAssetPermission(ctx context.Context, grant *iam.AssetPermission) error {
	if grant.TeamID <= 0 || grant.TargetID <= 0 || grant.MemberID <= 0 {
		return iam.ErrInvalidPrincipal
	}
	if err := grant.Validate(); err != nil {
		return err
	}
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		if grant.Version == 0 {
			grant.Version = 1
		}
		_, err := tx.Exec(ctx, `INSERT INTO iam_asset_perm(team_id,target_id,member_id,permission,concurrent_quota,sandbox_quota,version,created_by,updated_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT(team_id,target_id,member_id) DO UPDATE SET permission=EXCLUDED.permission,concurrent_quota=EXCLUDED.concurrent_quota,sandbox_quota=EXCLUDED.sandbox_quota,version=iam_asset_perm.version+1,updated_by=EXCLUDED.updated_by,updated_at=now()`, grant.TeamID, grant.TargetID, grant.MemberID, grant.Permission, grant.ConcurrentQuota, grant.SandboxQuota, grant.Version, grant.CreatedBy, grant.UpdatedBy)
		return err
	})
}

func (s *RBACStore) FindAssetPermission(ctx context.Context, teamID, targetID, memberID int64) (*iam.AssetPermission, error) {
	var out iam.AssetPermission
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT team_id,target_id,member_id,permission,concurrent_quota,sandbox_quota,version,created_by,created_at,updated_by,updated_at FROM iam_asset_perm WHERE team_id=$1 AND target_id=$2 AND member_id=$3`, teamID, targetID, memberID).Scan(&out.TeamID, &out.TargetID, &out.MemberID, &out.Permission, &out.ConcurrentQuota, &out.SandboxQuota, &out.Version, &out.CreatedBy, &out.CreatedAt, &out.UpdatedBy, &out.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *RBACStore) ListAssetPermissions(ctx context.Context, teamID, targetID int64) ([]*iam.AssetPermission, error) {
	var out []*iam.AssetPermission
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT team_id,target_id,member_id,permission,concurrent_quota,sandbox_quota,version,created_by,created_at,updated_by,updated_at FROM iam_asset_perm WHERE team_id=$1 AND target_id=$2 ORDER BY member_id`, teamID, targetID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p iam.AssetPermission
			if err := rows.Scan(&p.TeamID, &p.TargetID, &p.MemberID, &p.Permission, &p.ConcurrentQuota, &p.SandboxQuota, &p.Version, &p.CreatedBy, &p.CreatedAt, &p.UpdatedBy, &p.UpdatedAt); err != nil {
				return err
			}
			out = append(out, &p)
		}
		return rows.Err()
	})
	return out, err
}

func (s *RBACStore) DeleteAssetPermission(ctx context.Context, teamID, targetID, memberID int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM iam_asset_perm WHERE team_id=$1 AND target_id=$2 AND member_id=$3`, teamID, targetID, memberID)
		return err
	})
}

var _ iam.RBACRepository = (*RBACStore)(nil)
