package iam

import (
	"context"
	"time"
)

// StaticProvider is a deterministic provider for unit and integration tests.
type StaticProvider struct {
	Credential string
	Principal  Principal
}

func (p StaticProvider) Authenticate(_ context.Context, credential string) AuthenticationResult {
	if credential == "" || credential != p.Credential {
		return AuthenticationResult{Err: ErrUnauthenticated}
	}
	return AuthenticationResult{Principal: &p.Principal}
}

// StaticAuthorizer allows an explicit action/resource pair. It is intentionally
// test-only policy plumbing and does not represent production RBAC.
type StaticAuthorizer struct {
	Action   string
	Resource string
}

func (a StaticAuthorizer) Authorize(_ context.Context, principal Principal, action, resource string) error {
	if err := principal.Validate(now()); err != nil {
		return err
	}
	if action != a.Action || resource != a.Resource {
		return ErrForbidden
	}
	return nil
}

var now = func() time.Time { return time.Now() }
