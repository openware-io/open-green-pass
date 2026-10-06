package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openware-io/open-green-pass/internal/iam"
)

func TestPrincipalAuthRequiresCredentialWithoutFallback(t *testing.T) {
	h := PrincipalAuth(iam.StaticProvider{Credential: "ok", Principal: iam.Principal{Subject: "u", TenantID: "t"}}, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestPrincipalAuthAllowsMetricsWithoutEndUserCredential(t *testing.T) {
	h := PrincipalAuth(nil, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestPrincipalAuthBuildsDevPrincipalOnlyWhenFallbackExplicitlyEnabled(t *testing.T) {
	h := PrincipalAuth(nil, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := iam.PrincipalFromContext(r.Context())
		if !ok || p.Subject != "42" || p.TenantID != "10" || p.Issuer != "gp-dev-header" {
			t.Fatalf("principal=%+v present=%v", p, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("x-gp-team-id", "10")
	r.Header.Set("x-gp-user-id", "42")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestPrincipalAuthRejectsDevHeaderWhenFallbackDisabled(t *testing.T) {
	h := PrincipalAuth(nil, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("x-gp-team-id", "10")
	r.Header.Set("x-gp-user-id", "42")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestPrincipalAuthInjectsValidatedPrincipal(t *testing.T) {
	h := PrincipalAuth(iam.StaticProvider{Credential: "ok", Principal: iam.Principal{Subject: "u", TenantID: "t"}}, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := iam.PrincipalFromContext(r.Context()); !ok {
			t.Fatal("principal missing")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer ok")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d", w.Code)
	}
}
