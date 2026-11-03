package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

type recordingSSELimiter struct {
	allow           bool
	renewErr        error
	acquired        int
	released        int
	renewed         int
	acquiredLeaseID string
	releasedLeaseID string
	renewedLeaseID  string
}

func (l *recordingSSELimiter) Acquire(_ context.Context, _ int64, leaseID string) (bool, error) {
	l.acquired++
	l.acquiredLeaseID = leaseID
	return l.allow, nil
}

func (l *recordingSSELimiter) Release(_ context.Context, _ int64, leaseID string) error {
	l.released++
	l.releasedLeaseID = leaseID
	return nil
}

func (l *recordingSSELimiter) Renew(_ context.Context, _ int64, leaseID string) error {
	l.renewed++
	l.renewedLeaseID = leaseID
	return l.renewErr
}

func sseHandlerRequest(t *testing.T, handler http.Handler, runID int64, timeout time.Duration) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	req := httptest.NewRequest(http.MethodGet, "/generation/runs/"+strconv.FormatInt(runID, 10)+"/events", nil).WithContext(ctx)
	req = req.WithContext(rls.WithTenant(req.Context(), 10))
	req = req.WithContext(iam.WithPrincipal(req.Context(), iam.Principal{Subject: "40", TenantID: "10"}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestGenerationRunEventsAcquiresRenewsAndReleasesLease(t *testing.T) {
	svc, runRepo, eventRepo := newSSETestService(t)
	seedRunningRun(t, runRepo, 730)
	limiter := &recordingSSELimiter{allow: true}
	handler := newRunEventHandler(svc, eventRepo, nil, limiter, &testGenerationAuthorizer{allowed: true})
	handler.snapshotInterval = 40 * time.Millisecond
	handler.heartbeatInterval = 30 * time.Millisecond
	handler.fallbackInterval = 5 * time.Second
	mux := http.NewServeMux()
	mux.HandleFunc("GET /generation/runs/{id}/events", handler.stream)
	rec := sseHandlerRequest(t, mux, 730, 220*time.Millisecond)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if limiter.acquired != 1 || limiter.released != 1 {
		t.Fatalf("lease acquire/release = %d/%d, want 1/1", limiter.acquired, limiter.released)
	}
	if limiter.renewed == 0 {
		t.Fatal("long-lived SSE did not renew its lease")
	}
	if limiter.acquiredLeaseID == "" {
		t.Fatal("SSE acquired an empty lease id")
	}
	if limiter.releasedLeaseID != limiter.acquiredLeaseID {
		t.Fatalf("release lease id = %q, want %q", limiter.releasedLeaseID, limiter.acquiredLeaseID)
	}
	if limiter.renewedLeaseID != limiter.acquiredLeaseID {
		t.Fatalf("renew lease id = %q, want %q", limiter.renewedLeaseID, limiter.acquiredLeaseID)
	}
}

func TestGenerationRunEventsRejectsWhenTenantBudgetExhausted(t *testing.T) {
	svc, runRepo, eventRepo := newSSETestService(t)
	seedRunningRun(t, runRepo, 731)
	limiter := &recordingSSELimiter{allow: false}
	handler := newRunEventHandler(svc, eventRepo, nil, limiter, &testGenerationAuthorizer{allowed: true})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /generation/runs/{id}/events", handler.stream)
	rec := sseHandlerRequest(t, mux, 731, 100*time.Millisecond)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if limiter.acquired != 1 || limiter.released != 0 {
		t.Fatalf("denied stream lease acquire/release = %d/%d", limiter.acquired, limiter.released)
	}
	var body generationErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "too_many_streams" {
		t.Fatalf("error code = %q", body.Error)
	}
}

func TestGenerationRunEventsClosesWhenLeaseRenewalFails(t *testing.T) {
	svc, runRepo, eventRepo := newSSETestService(t)
	seedRunningRun(t, runRepo, 732)
	limiter := &recordingSSELimiter{allow: true, renewErr: errors.New("primary and fallback unavailable")}
	handler := newRunEventHandler(svc, eventRepo, nil, limiter, &testGenerationAuthorizer{allowed: true})
	handler.heartbeatInterval = 20 * time.Millisecond
	handler.snapshotInterval = 5 * time.Second
	handler.fallbackInterval = 5 * time.Second
	mux := http.NewServeMux()
	mux.HandleFunc("GET /generation/runs/{id}/events", handler.stream)
	started := time.Now()
	rec := sseHandlerRequest(t, mux, 732, 5*time.Second)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("SSE did not close after renewal failure; took %v", elapsed)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if limiter.released != 1 {
		t.Fatalf("release calls = %d, want deferred release after failed renewal", limiter.released)
	}
}
