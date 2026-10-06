package application

import (
	"context"
	"sync"
	"testing"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type atomicAuditRepo struct {
	mu     sync.Mutex
	events []domain.AuditEvent
}

func (r *atomicAuditRepo) LatestHash(context.Context, int64) (string, error) { return "", nil }
func (r *atomicAuditRepo) Append(context.Context, *domain.AuditEvent) error  { return nil }

func (r *atomicAuditRepo) AppendWithHead(_ context.Context, _ int64, build func(string) (*domain.AuditEvent, error)) (*domain.AuditEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prev := zeroChainHead
	if n := len(r.events); n > 0 {
		prev = r.events[n-1].Hash
	}
	event, err := build(prev)
	if err != nil {
		return nil, err
	}
	r.events = append(r.events, *event)
	return event, nil
}

func TestAuditServiceUsesAtomicRepositoryForConcurrentAppends(t *testing.T) {
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo := &atomicAuditRepo{}
	svc := NewAuditService(repo, gen)
	ctx := rls.WithTenant(context.Background(), 7)

	const writers = 32
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Append(ctx, 1, "run.created", "run", 99, map[string]any{"source": "test"})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := VerifyAuditChain(repo.events); !got.Valid || got.CheckedCount != writers {
		t.Fatalf("atomic append chain invalid: %+v", got)
	}
}
