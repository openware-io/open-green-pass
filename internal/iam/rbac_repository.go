package iam

import (
	"context"
	"time"
)

// MemberStatus is the lifecycle state of a team membership.
type MemberStatus string

const (
	MemberActive   MemberStatus = "active"
	MemberDisabled MemberStatus = "disabled"
	MemberInvited  MemberStatus = "invited"
)

func (s MemberStatus) Validate() error {
	switch s {
	case MemberActive, MemberDisabled, MemberInvited:
		return nil
	default:
		return ErrInvalidMemberStatus
	}
}

// Member is a principal's membership in one tenant. Provider-specific
// identity claims are intentionally not persisted here; PrincipalID is the
// stable identifier issued by the authentication boundary.
type Member struct {
	ID           int64
	TeamID       int64
	PrincipalID  string
	Role         Role
	Status       MemberStatus
	LastActiveAt *time.Time
	CreatedBy    int64
	CreatedAt    time.Time
	UpdatedBy    int64
	UpdatedAt    time.Time
}

func (m Member) Validate() error {
	if m.TeamID <= 0 || m.ID <= 0 || m.PrincipalID == "" {
		return ErrInvalidPrincipal
	}
	if err := m.Role.Validate(); err != nil {
		return err
	}
	return m.Status.Validate()
}

// AssetOwner identifies the single responsible member for a target asset.
// The database uniqueness constraint enforces one owner per team/asset.
type AssetOwner struct {
	TeamID     int64
	TargetID   int64
	MemberID   int64
	AssignedAt time.Time
	AssignedBy int64
}

// AssetPermission is an explicit member-to-asset grant. Quotas are limits;
// zero has the domain meaning defined by the quota service and is retained as
// a concrete value (rather than being conflated with SQL NULL).
type AssetPermission struct {
	TeamID          int64
	TargetID        int64
	MemberID        int64
	Permission      Permission
	ConcurrentQuota int64
	SandboxQuota    int64
	Version         int64
	CreatedBy       int64
	CreatedAt       time.Time
	UpdatedBy       int64
	UpdatedAt       time.Time
}

var ErrInvalidMemberStatus = domainError("iam: invalid member status")
var ErrInvalidQuota = domainError("iam: quota must be non-negative")

// Validate checks values that must hold independently of persistence.
func (p AssetPermission) Validate() error {
	if err := p.Permission.Validate(); err != nil {
		return err
	}
	if p.ConcurrentQuota < 0 || p.SandboxQuota < 0 {
		return ErrInvalidQuota
	}
	return nil
}

// RBACRepository is the persistence port for team memberships and explicit
// asset grants. Implementations must execute queries through the tenant-aware
// DB boundary so PostgreSQL RLS remains effective.
type RBACRepository interface {
	SaveMember(ctx context.Context, member *Member) error
	FindMember(ctx context.Context, teamID, memberID int64) (*Member, error)
	FindMemberByPrincipal(ctx context.Context, teamID int64, principalID string) (*Member, error)
	ListMembers(ctx context.Context, teamID int64, status MemberStatus) ([]*Member, error)
	SaveAssetOwner(ctx context.Context, owner *AssetOwner) error
	FindAssetOwner(ctx context.Context, teamID, targetID int64) (*AssetOwner, error)
	SaveAssetPermission(ctx context.Context, grant *AssetPermission) error
	FindAssetPermission(ctx context.Context, teamID, targetID, memberID int64) (*AssetPermission, error)
	ListAssetPermissions(ctx context.Context, teamID, targetID int64) ([]*AssetPermission, error)
	DeleteAssetPermission(ctx context.Context, teamID, targetID, memberID int64) error
}

// domainError keeps this package free from a dependency on an infrastructure
// error type while still exposing sentinel errors with stable messages.
type domainError string

func (e domainError) Error() string { return string(e) }
