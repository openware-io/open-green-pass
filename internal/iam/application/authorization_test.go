package application

import (
	"context"
	"errors"
	"testing"

	"github.com/openware-io/open-green-pass/internal/iam"
)

type fakeRBAC struct {
	member *iam.Member
	grant  *iam.AssetPermission
}

type recordingAudit struct{ event AuditEvent }

func (a *recordingAudit) Append(_ context.Context, event AuditEvent) error { a.event = event; return nil }

func (f fakeRBAC) SaveMember(context.Context, *iam.Member) error { return nil }
func (f fakeRBAC) FindMember(context.Context, int64, int64) (*iam.Member, error) {
	return f.member, nil
}
func (f fakeRBAC) FindMemberByPrincipal(context.Context, int64, string) (*iam.Member, error) {
	if f.member == nil {
		return nil, errors.New("missing")
	}
	return f.member, nil
}
func (f fakeRBAC) ListMembers(context.Context, int64, iam.MemberStatus) ([]*iam.Member, error) {
	return nil, nil
}
func (f fakeRBAC) SaveAssetOwner(context.Context, *iam.AssetOwner) error { return nil }
func (f fakeRBAC) FindAssetOwner(context.Context, int64, int64) (*iam.AssetOwner, error) {
	return nil, nil
}
func (f fakeRBAC) SaveAssetPermission(context.Context, *iam.AssetPermission) error { return nil }
func (f fakeRBAC) FindAssetPermission(context.Context, int64, int64, int64) (*iam.AssetPermission, error) {
	if f.grant == nil {
		return nil, errors.New("missing")
	}
	return f.grant, nil
}
func (f fakeRBAC) ListAssetPermissions(context.Context, int64, int64) ([]*iam.AssetPermission, error) {
	return nil, nil
}
func (f fakeRBAC) DeleteAssetPermission(context.Context, int64, int64, int64) error { return nil }

func TestAuthorizerFailsClosedWithoutActiveMemberOrGrant(t *testing.T) {
	principal := iam.Principal{Subject: "user-1", TenantID: "10"}
	if err := NewAuthorizer(fakeRBAC{}).Authorize(context.Background(), principal, "view", "100"); err == nil {
		t.Fatal("missing member must be rejected")
	}
	f := fakeRBAC{member: &iam.Member{ID: 1, TeamID: 10, PrincipalID: "user-1", Role: iam.RoleViewer, Status: iam.MemberActive}}
	if err := NewAuthorizer(f).Authorize(context.Background(), principal, "view", "100"); err == nil {
		t.Fatal("missing grant must be rejected")
	}
}

func TestAuthorizerIntersectsRoleAndGrant(t *testing.T) {
	f := fakeRBAC{member: &iam.Member{ID: 1, TeamID: 10, PrincipalID: "user-1", Role: iam.RoleTester, Status: iam.MemberActive}, grant: &iam.AssetPermission{Permission: iam.PermissionExec}}
	a := NewAuthorizer(f)
	p := iam.Principal{Subject: "user-1", TenantID: "10"}
	if err := a.Authorize(context.Background(), p, "exec", "100"); err != nil {
		t.Fatal(err)
	}
	if err := a.Authorize(context.Background(), p, "edit", "100"); err == nil {
		t.Fatal("tester/exec must reject edit")
	}
}

func TestAuthorizerAuditsDeniedDecisionWithoutCredentialMaterial(t *testing.T) {
	f := fakeRBAC{member: &iam.Member{ID: 1, TeamID: 10, PrincipalID: "user-1", Role: iam.RoleTester, Status: iam.MemberActive}, grant: &iam.AssetPermission{Permission: iam.PermissionExec}}
	audit := &recordingAudit{}
	a := NewAuthorizer(f)
	a.SetAuditPort(audit)
	p := iam.Principal{Subject: "user-1", TenantID: "10"}
	if err := a.Authorize(context.Background(), p, "edit", "100"); err == nil {
		t.Fatal("expected policy denial")
	}
	if audit.event.Op != "iam.authorization.denied" || audit.event.AssetID != 100 {
		t.Fatalf("unexpected audit event: %#v", audit.event)
	}
	if audit.event.Payload["reason"] != "policy_denied" || audit.event.Payload["action"] != "edit" {
		t.Fatalf("unexpected audit payload: %#v", audit.event.Payload)
	}
	if _, ok := audit.event.Payload["credential"]; ok {
		t.Fatal("credential must not be present in denial audit")
	}
}
