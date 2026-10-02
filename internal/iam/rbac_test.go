package iam

import "testing"

func TestAuthorizeUsesRoleAndExplicitPermissionIntersection(t *testing.T) {
	for _, tc := range []struct {
		role       Role
		permission Permission
		action     string
		allowed    bool
	}{
		{RoleOwner, PermissionFull, "edit", true},
		{RoleTester, PermissionExec, "exec", true},
		{RoleTester, PermissionExec, "edit", false},
		{RoleAdmin, PermissionView, "exec", false},
		{RoleViewer, PermissionFull, "edit", false},
		{RoleOwner, PermissionNone, "view", false},
	} {
		err := Authorize(tc.role, tc.permission, tc.action)
		if (err == nil) != tc.allowed {
			t.Errorf("%+v err=%v", tc, err)
		}
	}
}
