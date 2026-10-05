package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type caseAuthzRepo struct{ cases map[int64]*domain.Case }

func (r *caseAuthzRepo) SaveCase(_ context.Context, c *domain.Case) error {
	r.cases[c.ID] = c
	return nil
}
func (r *caseAuthzRepo) SaveVersion(context.Context, *domain.CaseVersion) error { return nil }
func (r *caseAuthzRepo) FindCase(_ context.Context, _ int64, caseID int64) (*domain.Case, error) {
	return r.cases[caseID], nil
}
func (r *caseAuthzRepo) FindVersion(context.Context, int64, int64, int) (*domain.CaseVersion, error) {
	return nil, nil
}
func (r *caseAuthzRepo) ListCases(context.Context, int64, domain.CaseFilter) ([]*domain.Case, error) {
	return nil, nil
}
func (r *caseAuthzRepo) History(context.Context, int64, int64) ([]*domain.CaseVersion, error) {
	return nil, nil
}

type recordingCaseAuthorizer struct {
	allow    bool
	action   string
	resource string
}

func (a *recordingCaseAuthorizer) Authorize(_ context.Context, _ iam.Principal, action, resource string) error {
	a.action, a.resource = action, resource
	if a.allow {
		return nil
	}
	return iam.ErrForbidden
}

func newAuthorizedCaseMux(t *testing.T, authorizer iam.AuthorizationPort) *http.ServeMux {
	t.Helper()
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo := &caseAuthzRepo{cases: map[int64]*domain.Case{7: {ID: 7, TeamID: 1, TargetID: 101}}}
	svc := application.NewCaseService(repo, gen)
	mux := http.NewServeMux()
	RegisterCasesAuthorized(mux, svc, authorizer)
	return mux
}

func authorizedCaseRequest(t *testing.T, mux *http.ServeMux, method, path, body string, principal bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	ctx := rls.WithTenant(req.Context(), 1)
	if principal {
		ctx = iam.WithPrincipal(ctx, iam.Principal{Subject: "user-1", TenantID: "1"})
	}
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRegisterCasesAuthorizedProtectsCaseActionsByOwningTarget(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		action string
	}{
		{name: "create", method: http.MethodPost, path: "/cases", body: `{"target_id":101,"code":"C-1","title":"case"}`, action: "edit"},
		{name: "version", method: http.MethodPost, path: "/cases/7/versions", body: `{}`, action: "edit"},
		{name: "rollback", method: http.MethodPost, path: "/cases/7/rollback", body: `{"version":1}`, action: "edit"},
		{name: "delete", method: http.MethodDelete, path: "/cases/7", action: "edit"},
		{name: "list", method: http.MethodGet, path: "/cases?target_id=101", action: "view"},
		{name: "history", method: http.MethodGet, path: "/cases/7/history", action: "view"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorizer := &recordingCaseAuthorizer{}
			rec := authorizedCaseRequest(t, newAuthorizedCaseMux(t, authorizer), tt.method, tt.path, tt.body, true)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
			if authorizer.action != tt.action || authorizer.resource != "101" {
				t.Fatalf("authorization = (%q, %q), want (%q, %q)", authorizer.action, authorizer.resource, tt.action, "101")
			}
		})
	}
}

func TestRegisterCasesAuthorizedRequiresPrincipalAndTargetScopedList(t *testing.T) {
	mux := newAuthorizedCaseMux(t, &recordingCaseAuthorizer{allow: true})
	if rec := authorizedCaseRequest(t, mux, http.MethodGet, "/cases?target_id=101", "", false); rec.Code != http.StatusForbidden {
		t.Fatalf("missing principal status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec := authorizedCaseRequest(t, mux, http.MethodGet, "/cases", "", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("unscoped list status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRegisterCasesAuthorizedAllowsViewHistory(t *testing.T) {
	authorizer := &recordingCaseAuthorizer{allow: true}
	rec := authorizedCaseRequest(t, newAuthorizedCaseMux(t, authorizer), http.MethodGet, "/cases/7/history", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var history []any
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
		t.Fatalf("history response must be JSON: %v", err)
	}
}
