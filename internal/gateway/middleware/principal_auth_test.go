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
