// Package iam contains provider-independent identity and authorization contracts.
package iam

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrUnauthenticated  = errors.New("iam: unauthenticated")
	ErrForbidden        = errors.New("iam: forbidden")
	ErrInvalidPrincipal = errors.New("iam: invalid principal")
)

// Principal is the normalized identity used by application layers. Provider
// specific claims must not leak beyond the authentication adapter.
type Principal struct {
	Subject   string
	TenantID  string
	Roles     []string
	Issuer    string
	ExpiresAt time.Time
}

func (p Principal) Validate(now time.Time) error {
	if strings.TrimSpace(p.Subject) == "" || strings.TrimSpace(p.TenantID) == "" {
		return ErrInvalidPrincipal
	}
	if !p.ExpiresAt.IsZero() && !now.Before(p.ExpiresAt) {
		return ErrUnauthenticated
	}
	return nil
}

// AuthenticationResult is the only output accepted from an authentication provider.
// Exactly one of Principal and Err must be present.
type AuthenticationResult struct {
	Principal *Principal
	Err       error
}

func (r AuthenticationResult) Validate(now time.Time) error {
	if (r.Principal == nil) == (r.Err == nil) {
		return errors.New("iam: authentication result must contain exactly one outcome")
	}
	if r.Principal != nil {
		return r.Principal.Validate(now)
	}
	return r.Err
}

type AuthenticationProvider interface {
	Authenticate(ctx context.Context, credential string) AuthenticationResult
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalContextKey{}).(Principal)
	return p, ok
}

// AuthorizationPort keeps policy decisions independent from identity provider.
type AuthorizationPort interface {
	Authorize(ctx context.Context, principal Principal, action, resource string) error
}
