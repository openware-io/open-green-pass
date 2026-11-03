package api

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// GenerationSSELimiter is the API-level name of the tenant SSE lease contract.
// The interface lives in domain so infra adapters can implement it without an
// api↔infra import cycle; the default in-process implementation is below.
type GenerationSSELimiter = domain.GenerationSSELimiter

// MemoryGenerationSSELimiter is the per-process fallback/default lease store.
// Production wires a Redis-backed limiter so replicas share one tenant budget;
// the memory implementation keeps tests/embedded deployments functional and is
// also the degradation target when Redis is unavailable. Leases are keyed by ID
// so mixed primary/fallback streams can be released independently.
type MemoryGenerationSSELimiter struct {
	mu     sync.Mutex
	active map[int64]map[string]struct{}
	max    int
}

func NewMemoryGenerationSSELimiter(limit int) *MemoryGenerationSSELimiter {
	if limit <= 0 {
		limit = generationSSEMaxFromEnv()
	}
	return &MemoryGenerationSSELimiter{active: make(map[int64]map[string]struct{}), max: limit}
}

func (l *MemoryGenerationSSELimiter) Acquire(_ context.Context, teamID int64, leaseID string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	leases := l.active[teamID]
	if len(leases) >= l.max {
		return false, nil
	}
	if leases == nil {
		leases = make(map[string]struct{}, l.max)
		l.active[teamID] = leases
	}
	leases[leaseID] = struct{}{}
	return true, nil
}

func (l *MemoryGenerationSSELimiter) Release(_ context.Context, teamID int64, leaseID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	leases := l.active[teamID]
	delete(leases, leaseID)
	if len(leases) == 0 {
		delete(l.active, teamID)
	}
	return nil
}

// Renew is a no-op for the process-local lease: the slot lifetime is the
// process lifetime and there is no Redis TTL to extend.
func (l *MemoryGenerationSSELimiter) Renew(context.Context, int64, string) error { return nil }

func (l *MemoryGenerationSSELimiter) count(teamID int64) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.active[teamID])
}

// MaxLiveStreams returns the configured per-tenant limit; useful for logs/tests.
func (l *MemoryGenerationSSELimiter) MaxLiveStreams() int { return l.max }

// generationSSEMaxFromEnv is the shared default per-tenant SSE stream limit.
func generationSSEMaxFromEnv() int {
	value, err := strconv.Atoi(os.Getenv("GP_GENERATION_SSE_MAX_PER_TEAM"))
	if err != nil || value <= 0 {
		return 8
	}
	return value
}

// defaultGenerationSSELimiter is used by embedded/test compositions that do not
// inject a Redis-backed limiter.
func defaultGenerationSSELimiter() domain.GenerationSSELimiter {
	return NewMemoryGenerationSSELimiter(generationSSEMaxFromEnv())
}

// generationSSETooManyStreams writes a retryable 429 for budget exhaustion.
func generationSSETooManyStreams(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "3")
	writeGenerationError(w, http.StatusTooManyRequests, "too_many_streams", "too many concurrent generation event streams for this team")
}
