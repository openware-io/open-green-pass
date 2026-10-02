package iam

import (
	"context"
	"errors"
	"testing"
	"time"
)

func validPrincipal() Principal {
	return Principal{Subject: "user-1", TenantID: "tenant-1", ExpiresAt: time.Now().Add(time.Hour)}
}

func TestPrincipalValidateRequiresStableIdentity(t *testing.T) {
	p := validPrincipal()
	p.Subject = ""
	if !errors.Is(p.Validate(time.Now()), ErrInvalidPrincipal) {
		t.Fatal("expected invalid principal")
	}
}

func TestAuthenticationResultExactlyOneOutcome(t *testing.T) {
	if err := (AuthenticationResult{}).Validate(time.Now()); err == nil {
		t.Fatal("expected missing outcome error")
	}
	if err := (AuthenticationResult{Principal: ptr(validPrincipal()), Err: ErrUnauthenticated}).Validate(time.Now()); err == nil {
		t.Fatal("expected mutually exclusive outcome error")
	}
}

func TestStaticProvider(t *testing.T) {
	p := StaticProvider{Credential: "secret", Principal: validPrincipal()}
	if got := p.Authenticate(context.Background(), "bad"); !errors.Is(got.Err, ErrUnauthenticated) {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	got := p.Authenticate(context.Background(), "secret")
	if err := got.Validate(time.Now()); err != nil || got.Principal.Subject != "user-1" {
		t.Fatalf("unexpected result: %#v, %v", got, err)
	}
}

func TestStaticAuthorizer(t *testing.T) {
	a := StaticAuthorizer{Action: "run", Resource: "target:1"}
	if err := a.Authorize(context.Background(), validPrincipal(), "read", "target:1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if err := a.Authorize(context.Background(), validPrincipal(), "run", "target:1"); err != nil {
		t.Fatalf("expected authorization, got %v", err)
	}
}

func ptr[T any](v T) *T { return &v }
