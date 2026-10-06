package infra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

type fakeProvider struct {
	name  string
	err   error
	calls int
}

func (p *fakeProvider) Name() string { return p.name }
func (p *fakeProvider) Generate(context.Context, domain.GenerateRequest) (domain.GenerateResult, error) {
	p.calls++
	if p.err != nil {
		return domain.GenerateResult{}, p.err
	}
	return domain.GenerateResult{Text: p.name + " ok", TokensIn: 2, TokensOut: 3}, nil
}

type fakeMeter struct{ calls int }

func (m *fakeMeter) Record(context.Context, domain.GenerateResult) error { m.calls++; return nil }

func TestGatewayFallbackAndIdempotentMeter(t *testing.T) {
	primary := &fakeProvider{name: "primary", err: domain.RetryableError{Err: errors.New("down")}}
	fallback := &fakeProvider{name: "fallback"}
	meter := &fakeMeter{}
	g := NewGateway(1, meter, primary, fallback)
	for i := 0; i < 2; i++ {
		r, err := g.Generate(context.Background(), domain.GenerateRequest{RequestID: "req-1", Model: "primary", Prompt: "hello"})
		if err != nil || r.Text != "fallback ok" {
			t.Fatalf("result=%+v err=%v", r, err)
		}
	}
	if primary.calls != 2 || fallback.calls != 2 || meter.calls != 1 {
		t.Fatalf("calls primary=%d fallback=%d meter=%d", primary.calls, fallback.calls, meter.calls)
	}
}

func TestGatewayDoesNotFallbackForNonRetryableProviderError(t *testing.T) {
	primary := &fakeProvider{name: "primary", err: errors.New("invalid credential")}
	fallback := &fakeProvider{name: "fallback"}
	_, err := NewGateway(1, nil, primary, fallback).Generate(context.Background(), domain.GenerateRequest{RequestID: "req-1", Model: "primary", Prompt: "hello"})
	if err == nil || fallback.calls != 0 {
		t.Fatalf("err=%v fallback calls=%d", err, fallback.calls)
	}
}

func TestGatewayRequiresRequestID(t *testing.T) {
	if _, err := NewGateway(1, nil).Generate(context.Background(), domain.GenerateRequest{}); !errors.Is(err, ErrRequestIDRequired) {
		t.Fatalf("err=%v", err)
	}
}

func TestGatewaySemaphoreHonorsCancellation(t *testing.T) {
	block := &blockingProvider{name: "p", started: make(chan struct{}), release: make(chan struct{})}
	g := NewGateway(1, nil, block)
	go func() { _, _ = g.Generate(context.Background(), domain.GenerateRequest{RequestID: "first"}) }()
	select {
	case <-block.started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := g.Generate(ctx, domain.GenerateRequest{RequestID: "second"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
	close(block.release)
}

type blockingProvider struct {
	name             string
	started, release chan struct{}
}

func (p *blockingProvider) Name() string { return p.name }
func (p *blockingProvider) Generate(ctx context.Context, _ domain.GenerateRequest) (domain.GenerateResult, error) {
	select {
	case <-p.started:
	default:
		close(p.started)
	}
	select {
	case <-p.release:
		return domain.GenerateResult{Text: "ok"}, nil
	case <-ctx.Done():
		return domain.GenerateResult{}, ctx.Err()
	}
}

func TestGatewayCopiesOpaqueSecretRef(t *testing.T) {
	p := &fakeProvider{name: "p"}
	r, err := NewGateway(1, nil, p).Generate(context.Background(), domain.GenerateRequest{RequestID: "r", SecretRef: "secret://team/key"})
	if err != nil || r.SecretRef != "secret://team/key" {
		t.Fatalf("result=%+v err=%v", r, err)
	}
}
