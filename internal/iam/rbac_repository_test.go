package iam

import "testing"

func TestMemberAndGrantValidation(t *testing.T) {
	m := Member{ID: 1, TeamID: 10, PrincipalID: "p-1", Role: RoleTester, Status: MemberActive}
	if err := m.Validate(); err != nil {
		t.Fatalf("valid member: %v", err)
	}
	if err := (Member{ID: 1, TeamID: 10, PrincipalID: "p-1", Role: RoleTester, Status: MemberStatus("bad")}).Validate(); err != ErrInvalidMemberStatus {
		t.Fatalf("status error = %v", err)
	}
	if err := (AssetPermission{Permission: PermissionExec, ConcurrentQuota: -1}).Validate(); err != ErrInvalidQuota {
		t.Fatalf("quota error = %v", err)
	}
}
