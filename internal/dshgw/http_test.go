package dshgw

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandlerAuthAndCreate(t *testing.T) {
	m := NewManager(StubRunner{Delay: 5 * time.Millisecond}, 1, 2)
	defer m.Close()
	handler := NewHandler(m, "secret")

	req := httptest.NewRequest(http.MethodPost, "/internal/dsh/jobs", bytes.NewBufferString(`{"job_id":"j1","prompt":"x"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/internal/dsh/jobs", bytes.NewBufferString(`{"job_id":"j1","prompt":"x"}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var job Job
	if err := json.NewDecoder(rec.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	if job.ID != "j1" || job.State != StateQueued {
		t.Fatalf("job=%+v", job)
	}

	req = httptest.NewRequest(http.MethodGet, "/internal/dsh/jobs/j1/events", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("events status=%d", rec.Code)
	}
}
