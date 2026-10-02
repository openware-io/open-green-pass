package iam

import "errors"

type Role string
type Permission string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleTester Role = "tester"
	RoleViewer Role = "viewer"

	PermissionFull Permission = "full"
	PermissionEdit Permission = "edit"
	PermissionExec Permission = "exec"
	PermissionView Permission = "view"
	PermissionNone Permission = "none"
)

var ErrInvalidRole = errors.New("iam: invalid role")
var ErrInvalidPermission = errors.New("iam: invalid permission")

func (r Role) Validate() error {
	switch r {
	case RoleOwner, RoleAdmin, RoleTester, RoleViewer:
		return nil
	}
	return ErrInvalidRole
}

func (p Permission) Validate() error {
	switch p {
	case PermissionFull, PermissionEdit, PermissionExec, PermissionView, PermissionNone:
		return nil
	}
	return ErrInvalidPermission
}

// Allows applies the fixed initial role baseline. Explicit asset permissions
// can further restrict a role, but never expand it.
func (r Role) Allows(action string) bool {
	switch r {
	case RoleOwner, RoleAdmin:
		return action != ""
	case RoleTester:
		return action == "view" || action == "exec"
	case RoleViewer:
		return action == "view"
	default:
		return false
	}
}

func (p Permission) Allows(action string) bool {
	switch p {
	case PermissionFull:
		return action != ""
	case PermissionEdit:
		return action == "view" || action == "edit"
	case PermissionExec:
		return action == "view" || action == "exec"
	case PermissionView:
		return action == "view"
	default:
		return false
	}
}

func Authorize(role Role, explicit Permission, action string) error {
	if role.Validate() != nil || explicit.Validate() != nil {
		return ErrForbidden
	}
	if explicit == PermissionNone || !role.Allows(action) || !explicit.Allows(action) {
		return ErrForbidden
	}
	return nil
}
