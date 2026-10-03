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
type Authorizer struct{ repo iam.RBACRepository }

func NewAuthorizer(repo iam.RBACRepository) *Authorizer { return &Authorizer{repo: repo} }

func (a *Authorizer) Authorize(ctx context.Context, principal iam.Principal, action, resource string) error {
	if err := principal.Validate(now()); err != nil {
		return err
	}
	teamID, err := strconv.ParseInt(principal.TenantID, 10, 64)
	if err != nil || teamID <= 0 {
		return iam.ErrForbidden
	}
	m, err := a.repo.FindMemberByPrincipal(ctx, teamID, principal.Subject)
	if err != nil {
		return err
	}
	if m.Status != iam.MemberActive {
		return ErrMembershipRequired
	}
	targetID, err := strconv.ParseInt(resource, 10, 64)
	if err != nil || targetID <= 0 {
		return iam.ErrForbidden
	}
	grant, err := a.repo.FindAssetPermission(ctx, teamID, targetID, m.ID)
	if err != nil {
		return err
	}
	return iam.Authorize(m.Role, grant.Permission, action)
}

var now = func() (t time.Time) { return time.Now().UTC() }
