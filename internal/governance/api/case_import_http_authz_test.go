package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type fakeCaseImporter struct {
	projectID   int64
	importCalls int
	lastJob     int64
	lastTarget  int64
	err         error
}

func (f *fakeCaseImporter) JobProject(context.Context, int64) (int64, error) {
	return f.projectID, f.err
}

func (f *fakeCaseImporter) Import(_ context.Context, jobID, targetID int64) ([]*domain.Case, error) {
	f.importCalls++
	f.lastJob, f.lastTarget = jobID, targetID
	if f.err != nil {
		return nil, f.err
	}
	return []*domain.Case{}, nil
}

type caseImportAuthorizer struct {
	allowed  bool
	action   string
	resource string
	calls    int
}

func (a *caseImportAuthorizer) Authorize(_ context.Context, _ iam.Principal, action, resource string) error {
	a.calls++
	a.action, a.resource = action, resource
	if a.allowed {
		return nil
	}
	return iam.ErrForbidden
}

func postCaseImport(t *testing.T, mux *http.ServeMux, body string, withPrincipal bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/generation/jobs/9/cases:import", strings.NewReader(body))
	req = req.WithContext(rls.WithTenant(req.Context(), 10))
	if withPrincipal {
		req = req.WithContext(iam.WithPrincipal(req.Context(), iam.Principal{Subject: "40", TenantID: "10"}))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestCaseImportRequiresIAM(t *testing.T) {
	authorizer := &caseImportAuthorizer{allowed: true}
	svc := &fakeCaseImporter{projectID: 20}
	mux := http.NewServeMux()
	RegisterCaseImportAuthorized(mux, svc, authorizer)

	if rec := postCaseImport(t, mux, `{"target_id":20}`, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous import status = %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.importCalls != 0 {
		t.Fatalf("anonymous import reached service: %d", svc.importCalls)
	}

	authorizer.allowed = false
	rec := postCaseImport(t, mux, `{"target_id":20}`, true)
	if rec.Code != http.StatusForbidden || authorizer.action != "edit" || authorizer.resource != "20" {
		t.Fatalf("denied import status = %d action=%q resource=%q body=%s", rec.Code, authorizer.action, authorizer.resource, rec.Body.String())
	}
	if svc.importCalls != 0 {
		t.Fatalf("denied import reached service: %d", svc.importCalls)
	}

	authorizer.allowed = true
	if rec := postCaseImport(t, mux, `{"target_id":20}`, true); rec.Code != http.StatusCreated || svc.importCalls != 1 || svc.lastJob != 9 || svc.lastTarget != 20 {
		t.Fatalf("allowed import status = %d calls=%d job=%d target=%d body=%s", rec.Code, svc.importCalls, svc.lastJob, svc.lastTarget, rec.Body.String())
	}

	// A client-supplied target that is not the job's project is rejected before
	// authorization/import.
	before := svc.importCalls
	if rec := postCaseImport(t, mux, `{"target_id":21}`, true); rec.Code != http.StatusBadRequest || svc.importCalls != before {
		t.Fatalf("mismatched target status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCaseImportMissingJobReturns404(t *testing.T) {
	svc := &fakeCaseImporter{err: gperr.NotFound("generation job not found")}
	authorizer := &caseImportAuthorizer{allowed: true}
	mux := http.NewServeMux()
	RegisterCaseImportAuthorized(mux, svc, authorizer)
	rec := postCaseImport(t, mux, `{"target_id":20}`, true)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing job import status = %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.importCalls != 0 {
		t.Fatalf("missing job import reached Import: %d", svc.importCalls)
	}
}
