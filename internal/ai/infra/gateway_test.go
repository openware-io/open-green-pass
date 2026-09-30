package infra

import (
	"context"
	"errors"
	"testing"

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
	primary := &fakeProvider{name: "primary", err: errors.New("down")}
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

func TestGatewayRequiresRequestID(t *testing.T) {
	if _, err := NewGateway(1, nil).Generate(context.Background(), domain.GenerateRequest{}); !errors.Is(err, ErrRequestIDRequired) {
		t.Fatalf("err=%v", err)
	}
}
