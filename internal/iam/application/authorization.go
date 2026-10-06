package application

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/openware-io/open-green-pass/internal/iam"
)

var ErrMembershipRequired = errors.New("iam: active membership required")

// Authorizer resolves a verified Principal to a tenant membership and explicit
// asset grant. It fails closed when a grant is absent.
type Authorizer struct {
	repo  iam.RBACRepository
	audit AuditPort
}

func NewAuthorizer(repo iam.RBACRepository) *Authorizer { return &Authorizer{repo: repo, audit: NoopAuditPort{}} }

// SetAuditPort records denied decisions without making the audit store part of
// the authorization availability path. Payloads contain only bounded policy
// labels; credentials and provider claims never cross this boundary.
func (a *Authorizer) SetAuditPort(port AuditPort) {
	if port != nil {
		a.audit = port
	}
}

func (a *Authorizer) deny(ctx context.Context, principal iam.Principal, action, resource, reason string, err error) error {
	teamID, _ := strconv.ParseInt(principal.TenantID, 10, 64)
	if a.audit != nil {
		_ = a.audit.Append(ctx, AuditEvent{Op: "iam.authorization.denied", Asset: "target", AssetID: resourceID(resource), Payload: map[string]any{
			"action": action, "reason": reason, "subject": principal.Subject, "team_id": teamID,
		}})
	}
	return err
}

func resourceID(resource string) int64 { id, _ := strconv.ParseInt(resource, 10, 64); return id }

func (a *Authorizer) Authorize(ctx context.Context, principal iam.Principal, action, resource string) error {
	if err := principal.Validate(now()); err != nil {
		return a.deny(ctx, principal, action, resource, "invalid_principal", err)
	}
	teamID, err := strconv.ParseInt(principal.TenantID, 10, 64)
	if err != nil || teamID <= 0 {
		return a.deny(ctx, principal, action, resource, "invalid_tenant", iam.ErrForbidden)
	}
	m, err := a.repo.FindMemberByPrincipal(ctx, teamID, principal.Subject)
	if err != nil {
		return a.deny(ctx, principal, action, resource, "membership_not_found", err)
	}
	if m.Status != iam.MemberActive {
		return a.deny(ctx, principal, action, resource, "inactive_membership", ErrMembershipRequired)
	}
	targetID, err := strconv.ParseInt(resource, 10, 64)
	if err != nil || targetID <= 0 {
		return a.deny(ctx, principal, action, resource, "invalid_resource", iam.ErrForbidden)
	}
	grant, err := a.repo.FindAssetPermission(ctx, teamID, targetID, m.ID)
	if err != nil {
		return a.deny(ctx, principal, action, resource, "asset_grant_not_found", err)
	}
	if err := iam.Authorize(m.Role, grant.Permission, action); err != nil {
		return a.deny(ctx, principal, action, resource, "policy_denied", err)
	}
	return nil
}

var now = func() (t time.Time) { return time.Now().UTC() }
