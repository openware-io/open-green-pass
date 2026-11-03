package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	aidomain "github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/iam"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

type fakeGenerationJobService struct {
	job         *aidomain.GenerationJob
	err         error
	createCalls int
	getCalls    int
	cancelCalls int
}

func (f *fakeGenerationJobService) Create(context.Context, application.CreateGenerationJobInput) (*aidomain.GenerationJob, bool, error) {
	f.createCalls++
	if f.err != nil {
		return nil, false, f.err
	}
	return f.job, true, nil
}

func (f *fakeGenerationJobService) Get(context.Context, int64) (*aidomain.GenerationJob, error) {
	f.getCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.job, nil
}

func (f *fakeGenerationJobService) Cancel(context.Context, int64) error {
	f.cancelCalls++
	return f.err
}

func newJobAPI(t *testing.T, authorizer iam.AuthorizationPort) (*http.ServeMux, *fakeGenerationJobService) {
	t.Helper()
	job := &aidomain.GenerationJob{
		ID: 7, TeamID: 10, ProjectID: 20, Module: "m", State: aidomain.GenerationJobQueued,
		Prompt: "p", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	svc := &fakeGenerationJobService{job: job}
	mux := http.NewServeMux()
	RegisterGenerationJobsAuthorized(mux, svc, authorizer)
	return mux, svc
}

func TestGenerationJobCreateRequiresIAM(t *testing.T) {
	authorizer := &testGenerationAuthorizer{allowed: true}
	mux, svc := newJobAPI(t, authorizer)
	body := `{"project_id":20,"prompt":"p","idempotency_key":"k"}`
	if rec := generationRequest(t, mux, http.MethodPost, "/ai/generation-jobs", body, 10, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous job create status = %d body=%s", rec.Code, rec.Body.String())
	}
	authorizer.allowed = false
	if rec := generationRequest(t, mux, http.MethodPost, "/ai/generation-jobs", body, 10, true); rec.Code != http.StatusForbidden || authorizer.action != "edit" || authorizer.resource != "20" {
		t.Fatalf("denied job create status = %d action=%q resource=%q", rec.Code, authorizer.action, authorizer.resource)
	}
	if svc.createCalls != 0 {
		t.Fatalf("denied job create reached service: %d", svc.createCalls)
	}
	authorizer.allowed = true
	if rec := generationRequest(t, mux, http.MethodPost, "/ai/generation-jobs", body, 10, true); rec.Code != http.StatusAccepted || svc.createCalls != 1 {
		t.Fatalf("allowed job create status = %d calls=%d body=%s", rec.Code, svc.createCalls, rec.Body.String())
	}
}

func TestGenerationJobGetRequiresIAM(t *testing.T) {
	authorizer := &testGenerationAuthorizer{allowed: true}
	mux, _ := newJobAPI(t, authorizer)
	authorizer.allowed = false
	if rec := generationRequest(t, mux, http.MethodGet, "/ai/generation-jobs/7", "", 10, true); rec.Code != http.StatusForbidden || authorizer.action != "view" {
		t.Fatalf("denied job get status = %d action=%q", rec.Code, authorizer.action)
	}
	authorizer.allowed = true
	if rec := generationRequest(t, mux, http.MethodGet, "/ai/generation-jobs/7", "", 10, true); rec.Code != http.StatusOK {
		t.Fatalf("allowed job get status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGenerationJobCancelRequiresIAM(t *testing.T) {
	authorizer := &testGenerationAuthorizer{allowed: true}
	mux, svc := newJobAPI(t, authorizer)
	authorizer.allowed = false
	if rec := generationRequest(t, mux, http.MethodDelete, "/ai/generation-jobs/7", "", 10, true); rec.Code != http.StatusForbidden || authorizer.action != "edit" {
		t.Fatalf("denied job cancel status = %d action=%q", rec.Code, authorizer.action)
	}
	if svc.cancelCalls != 0 {
		t.Fatalf("denied job cancel reached service: %d", svc.cancelCalls)
	}
	authorizer.allowed = true
	if rec := generationRequest(t, mux, http.MethodDelete, "/ai/generation-jobs/7", "", 10, true); rec.Code != http.StatusAccepted || svc.cancelCalls != 1 {
		t.Fatalf("allowed job cancel status = %d calls=%d", rec.Code, svc.cancelCalls)
	}
}

func TestGenerationJobMissingReturns404(t *testing.T) {
	svc := &fakeGenerationJobService{err: gperr.NotFound("generation job not found")}
	mux := http.NewServeMux()
	RegisterGenerationJobsAuthorized(mux, svc, &testGenerationAuthorizer{allowed: true})
	if rec := generationRequest(t, mux, http.MethodGet, "/ai/generation-jobs/999", "", 10, true); rec.Code != http.StatusNotFound {
		t.Fatalf("missing job get status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := generationRequest(t, mux, http.MethodDelete, "/ai/generation-jobs/999", "", 10, true); rec.Code != http.StatusNotFound {
		t.Fatalf("missing job cancel status = %d body=%s", rec.Code, rec.Body.String())
	}
}
