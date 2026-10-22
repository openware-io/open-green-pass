package dshgw

import (
	"context"
	"fmt"
	"time"
)

// StubRunner is used by the M1 gateway skeleton and tests. It does not invoke
// DSH; the production SDK runner will implement the same interface and keep
// one child process per job.
type StubRunner struct {
	Delay time.Duration
}

// Run returns a deterministic marker so the API/queue can be exercised without
// a model or credentials.
func (r StubRunner) Run(ctx context.Context, job Job) (Result, error) {
	delay := r.Delay
	if delay <= 0 {
		delay = 30 * time.Millisecond
	}
	select {
	case <-time.After(delay):
		return Result{Output: fmt.Sprintf("stub:%s", job.ID)}, nil
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}
