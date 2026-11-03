package infra

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/pkg/clock"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// blockingTransport blocks a request until its context is cancelled, proving
// that a user stop reaches the HTTP client instead of waiting for the full
// per-request timeout.
type blockingTransport struct {
	once    sync.Once
	started chan struct{}
}

func (b *blockingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	b.once.Do(func() { close(b.started) })
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestHTTPRunnerStopsWhenContextCancelled(t *testing.T) {
	started := make(chan struct{})
	gen, err := id.New(1, clock.System())
	if err != nil {
		t.Fatalf("id generator: %v", err)
	}
	runner := NewHTTPRunner(gen, "")
	runner.client = &http.Client{Transport: &blockingTransport{started: started}}

	specs := []*domain.CaseSpec{{
		CaseID: 1, CaseVersion: 1, BaseURL: "http://greenpass.invalid",
		Script: map[string]any{
			"request":    map[string]any{"method": "GET", "path": "/slow"},
			"assertions": []any{map[string]any{"type": "status", "expected_status": 200}},
		},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Execute(ctx, specs)
		done <- runErr
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request never reached the transport")
	}
	start := time.Now()
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("runner err = %v, want context.Canceled", runErr)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("runner took %s to stop after cancel", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not return after context cancellation")
	}
}
