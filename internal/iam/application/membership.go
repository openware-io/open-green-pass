package application

import (
	"context"
	"errors"

	"github.com/openware-io/open-green-pass/internal/iam"
)

var ErrLastOwner = errors.New("iam: cannot remove or disable the last owner")

type MembershipService struct {
	repo  iam.RBACRepository
	audit AuditPort
}

func NewMembershipService(repo iam.RBACRepository) *MembershipService {
	return &MembershipService{repo: repo, audit: NoopAuditPort{}}
}

func (s *MembershipService) SetAuditPort(audit AuditPort) {
	if audit != nil {
		s.audit = audit
	}
}

// ChangeRole protects the team invariant that at least one active owner
// remains. Persistence implementations should execute this command in a
// transaction with a row lock when concurrent membership changes are enabled.
func (s *MembershipService) ChangeRole(ctx context.Context, teamID, memberID int64, role iam.Role, status iam.MemberStatus) error {
	if err := role.Validate(); err != nil {
		return err
	}
	if err := status.Validate(); err != nil {
		return err
	}
	m, err := s.repo.FindMember(ctx, teamID, memberID)
	if err != nil {
		return err
	}
	if m.Role == iam.RoleOwner && (role != iam.RoleOwner || status != iam.MemberActive) {
		members, err := s.repo.ListMembers(ctx, teamID, iam.MemberActive)
		if err != nil {
			return err
		}
		owners := 0
		for _, candidate := range members {
			if candidate.Role == iam.RoleOwner {
				owners++
			}
		}
		if owners <= 1 {
			return ErrLastOwner
		}
	}
	previousRole, previousStatus := m.Role, m.Status
	m.Role, m.Status = role, status
	if err := s.repo.SaveMember(ctx, m); err != nil {
		return err
	}
	return s.audit.Append(ctx, AuditEvent{Op: "iam.member.changed", Asset: "member", AssetID: m.ID, Payload: map[string]any{"role_before": string(previousRole), "role_after": string(role), "status_before": string(previousStatus), "status_after": string(status)}})
}
