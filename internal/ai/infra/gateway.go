package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

var ErrNoProvider = errors.New("no AI provider available")
var ErrRequestIDRequired = errors.New("request id required")

// Gateway routes to the preferred provider, then fallbacks in order. The
// semaphore limits in-flight calls across all providers in one process.
type Gateway struct {
	mu        sync.Mutex
	providers map[string]domain.ModelProviderPort
	order     []string
	semaphore chan struct{}
	meter     domain.MeterPort
	seen      map[string]struct{}
}

func NewGateway(concurrency int, meter domain.MeterPort, providers ...domain.ModelProviderPort) *Gateway {
	if concurrency < 1 {
		concurrency = 1
	}
	g := &Gateway{providers: make(map[string]domain.ModelProviderPort), semaphore: make(chan struct{}, concurrency), meter: meter, seen: make(map[string]struct{})}
	for _, p := range providers {
		if p != nil {
			g.providers[p.Name()] = p
			g.order = append(g.order, p.Name())
		}
	}
	return g
}

func (g *Gateway) Generate(ctx context.Context, req domain.GenerateRequest) (domain.GenerateResult, error) {
	if strings.TrimSpace(req.RequestID) == "" {
		return domain.GenerateResult{}, fmt.Errorf("%w: %w", domain.ErrInvalidRequest, ErrRequestIDRequired)
	}
	if req.MaxTokens < 0 {
		return domain.GenerateResult{}, fmt.Errorf("%w: max tokens must not be negative", domain.ErrInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return domain.GenerateResult{}, err
	}
	select {
	case g.semaphore <- struct{}{}:
	case <-ctx.Done():
		return domain.GenerateResult{}, ctx.Err()
	}
	defer func() { <-g.semaphore }()
	order := g.orderFor(req.Model)
	var last error
	for _, name := range order {
		p := g.providers[name]
		if p == nil {
			continue
		}
		res, err := p.Generate(ctx, req)
		if err != nil {
			last = err
			if !domain.IsRetryable(err) {
				return domain.GenerateResult{}, err
			}
			continue
		}
		res.RequestID = req.RequestID
		res.SecretRef = req.SecretRef
		if res.Model == "" {
			res.Model = name
		}
		if g.meter != nil {
			if err := g.recordOnce(ctx, res); err != nil {
				return domain.GenerateResult{}, err
			}
		}
		return res, nil
	}
	if last == nil {
		last = ErrNoProvider
	}
	return domain.GenerateResult{}, fmt.Errorf("%w: %v", ErrNoProvider, last)
}

func (g *Gateway) orderFor(preferred string) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if preferred == "" {
		return append([]string(nil), g.order...)
	}
	out := []string{preferred}
	for _, n := range g.order {
		if n != preferred {
			out = append(out, n)
		}
	}
	return out
}

func (g *Gateway) recordOnce(ctx context.Context, res domain.GenerateResult) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.seen[res.RequestID]; ok {
		return nil
	}
	// Serialize the first record for a request ID. Mark it only after the
	// meter succeeds, so transient meter failures can be retried safely.
	if err := g.meter.Record(ctx, res); err != nil {
		return err
	}
	g.seen[res.RequestID] = struct{}{}
	return nil
}
