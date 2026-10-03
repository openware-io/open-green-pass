package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openware-io/open-green-pass/internal/iam"
)

func TestTenantRejectsHeaderMismatchingVerifiedPrincipal(t *testing.T) {
	h := Tenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("x-gp-team-id", "20")
	r = r.WithContext(iam.WithPrincipal(r.Context(), iam.Principal{Subject: "u", TenantID: "10"}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestTenantUsesVerifiedPrincipalTenant(t *testing.T) {
	h := Tenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(iam.WithPrincipal(r.Context(), iam.Principal{Subject: "u", TenantID: "10"}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d", w.Code)
	}
}
