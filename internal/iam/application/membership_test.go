package application

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/iam"
)

type membershipRepo struct {
	member  *iam.Member
	members []*iam.Member
	saved   *iam.Member
}

func (r *membershipRepo) SaveMember(_ context.Context, m *iam.Member) error { r.saved = m; return nil }
func (r *membershipRepo) FindMember(context.Context, int64, int64) (*iam.Member, error) {
	return r.member, nil
}
func (r *membershipRepo) FindMemberByPrincipal(context.Context, int64, string) (*iam.Member, error) {
	return nil, nil
}
func (r *membershipRepo) ListMembers(context.Context, int64, iam.MemberStatus) ([]*iam.Member, error) {
	return r.members, nil
}
func (r *membershipRepo) SaveAssetOwner(context.Context, *iam.AssetOwner) error { return nil }
func (r *membershipRepo) FindAssetOwner(context.Context, int64, int64) (*iam.AssetOwner, error) {
	return nil, nil
}
func (r *membershipRepo) SaveAssetPermission(context.Context, *iam.AssetPermission) error { return nil }
func (r *membershipRepo) FindAssetPermission(context.Context, int64, int64, int64) (*iam.AssetPermission, error) {
	return nil, nil
}
func (r *membershipRepo) ListAssetPermissions(context.Context, int64, int64) ([]*iam.AssetPermission, error) {
	return nil, nil
}
func (r *membershipRepo) DeleteAssetPermission(context.Context, int64, int64, int64) error {
	return nil
}

func TestMembershipServiceProtectsLastOwner(t *testing.T) {
	r := &membershipRepo{member: &iam.Member{ID: 1, TeamID: 10, Role: iam.RoleOwner, Status: iam.MemberActive}, members: []*iam.Member{{ID: 1, Role: iam.RoleOwner, Status: iam.MemberActive}}}
	if err := NewMembershipService(r).ChangeRole(context.Background(), 10, 1, iam.RoleAdmin, iam.MemberActive); err != ErrLastOwner {
		t.Fatalf("err=%v", err)
	}
}
func TestMembershipServiceAllowsOwnerHandoff(t *testing.T) {
	r := &membershipRepo{member: &iam.Member{ID: 1, TeamID: 10, Role: iam.RoleOwner, Status: iam.MemberActive}, members: []*iam.Member{{ID: 1, Role: iam.RoleOwner, Status: iam.MemberActive}, {ID: 2, Role: iam.RoleOwner, Status: iam.MemberActive}}}
	if err := NewMembershipService(r).ChangeRole(context.Background(), 10, 1, iam.RoleAdmin, iam.MemberActive); err != nil {
		t.Fatal(err)
	}
	if r.saved.Role != iam.RoleAdmin {
		t.Fatalf("role=%s", r.saved.Role)
	}
}
