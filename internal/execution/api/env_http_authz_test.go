package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openware-io/open-green-pass/internal/iam"
)

type recordingEnvAuthorizer struct {
	action   string
	resource string
	allow    bool
}

func (a *recordingEnvAuthorizer) Authorize(_ context.Context, _ iam.Principal, action, resource string) error {
	a.action, a.resource = action, resource
	if a.allow {
		return nil
	}
	return iam.ErrForbidden
}

func authorizedEnvRequest(t *testing.T, mux *http.ServeMux, method, path, body string, principal bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if principal {
		req = req.WithContext(iam.WithPrincipal(req.Context(), iam.Principal{Subject: "user-1", TenantID: "1"}))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRegisterAuthorizedProtectsEnvironmentAndScreenshotRoutes(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		action string
	}{
		{name: "register environment runtime", method: http.MethodPost, path: "/targets/101/env/runtime", body: `{}`, action: "exec"},
		{name: "check target version", method: http.MethodPost, path: "/targets/101/version-check", body: `{}`, action: "exec"},
		{name: "read version history", method: http.MethodGet, path: "/targets/101/version-checks", action: "view"},
		{name: "set screenshot policy", method: http.MethodPut, path: "/targets/101/screenshot-policy", body: `{}`, action: "edit"},
		{name: "read screenshot policy", method: http.MethodGet, path: "/targets/101/screenshot-policy", action: "view"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorizer := &recordingEnvAuthorizer{}
			mux := http.NewServeMux()
			RegisterAuthorized(mux, nil, nil, nil, authorizer)

			rec := authorizedEnvRequest(t, mux, tt.method, tt.path, tt.body, true)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
			if authorizer.action != tt.action || authorizer.resource != "101" {
				t.Fatalf("authorization = (%q, %q), want (%q, %q)", authorizer.action, authorizer.resource, tt.action, "101")
			}
		})
	}
}

func TestRegisterAuthorizedEnvironmentRoutesRequirePrincipal(t *testing.T) {
	authorizer := &recordingEnvAuthorizer{allow: true}
	mux := http.NewServeMux()
	RegisterAuthorized(mux, nil, nil, nil, authorizer)

	rec := authorizedEnvRequest(t, mux, http.MethodGet, "/targets/101/version-checks", "", false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if authorizer.action != "" || authorizer.resource != "" {
		t.Fatalf("authorizer called without principal: (%q, %q)", authorizer.action, authorizer.resource)
	}
}
