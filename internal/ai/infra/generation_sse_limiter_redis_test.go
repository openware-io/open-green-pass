package infra

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// fakeRedisSSEState emulates the ZSET lease scripts over an in-memory map so
// per-stream cleanup, key TTL and shared counts can be asserted without Redis.
type fakeRedisSSEState struct {
	mu      sync.Mutex
	scores  map[string]map[string]int64 // key -> member -> expiry ms
	keyTTL  map[string]int64
	evalErr error
	calls   int
}

type fakeRedisSSEEvaler struct{ state *fakeRedisSSEState }

func (f *fakeRedisSSEEvaler) EvalInt(_ context.Context, script string, keys []string, args ...any) (int64, error) {
	f.state.mu.Lock()
	defer f.state.mu.Unlock()
	f.state.calls++
	if f.state.evalErr != nil {
		return 0, f.state.evalErr
	}
	if len(keys) != 1 {
		return 0, errors.New("expected one key")
	}
	switch script {
	case acquireGenerationSSELua:
		return f.fakeAcquire(keys[0], args)
	case renewGenerationSSELua:
		return f.fakeRenew(keys[0], args)
	case releaseGenerationSSELua:
		return f.fakeRelease(keys[0], args)
	default:
		return 0, errors.New("unknown script")
	}
}

func (f *fakeRedisSSEEvaler) fakeAcquire(key string, args []any) (int64, error) {
	now, err := strconv.ParseInt(args[0].(string), 10, 64)
	if err != nil {
		return 0, err
	}
	ttl, err := strconv.ParseInt(args[1].(string), 10, 64)
	if err != nil {
		return 0, err
	}
	limit, err := strconv.ParseInt(args[2].(string), 10, 64)
	if err != nil {
		return 0, err
	}
	leaseID := args[3].(string)
	members := f.state.scores[key]
	for member, expiry := range members {
		if expiry <= now {
			delete(members, member)
		}
	}
	if int64(len(members)) >= limit {
		return 0, nil
	}
	if members == nil {
		members = map[string]int64{}
		f.state.scores[key] = members
	}
	members[leaseID] = now + ttl
	f.state.keyTTL[key] = ttl
	return 1, nil
}

func (f *fakeRedisSSEEvaler) fakeRenew(key string, args []any) (int64, error) {
	now, err := strconv.ParseInt(args[0].(string), 10, 64)
	if err != nil {
		return 0, err
	}
	ttl, err := strconv.ParseInt(args[1].(string), 10, 64)
	if err != nil {
		return 0, err
	}
	leaseID := args[2].(string)
	members := f.state.scores[key]
	if _, exists := members[leaseID]; !exists {
		return 0, nil
	}
	members[leaseID] = now + ttl
	f.state.keyTTL[key] = ttl
	return 1, nil
}

func (f *fakeRedisSSEEvaler) fakeRelease(key string, args []any) (int64, error) {
	leaseID := args[0].(string)
	members := f.state.scores[key]
	if _, exists := members[leaseID]; !exists {
		return 0, nil
	}
	delete(members, leaseID)
	if len(members) == 0 {
		delete(f.state.scores, key)
		delete(f.state.keyTTL, key)
	}
	return 1, nil
}

func (f *fakeRedisSSEEvaler) expireLease(teamKey, leaseID string) {
	f.state.mu.Lock()
	defer f.state.mu.Unlock()
	members := f.state.scores[teamKey]
	if members == nil {
		return
	}
	members[leaseID] = time.Now().Add(-time.Second).UnixMilli()
}

func TestRedisGenerationSSELimiterSharesLeaseCountAcrossInstances(t *testing.T) {
	state := &fakeRedisSSEState{scores: map[string]map[string]int64{}, keyTTL: map[string]int64{}}
	first := newRedisGenerationSSELimiter(&fakeRedisSSEEvaler{state: state}, 2, GenerationSSELeaseTTL)
	second := newRedisGenerationSSELimiter(&fakeRedisSSEEvaler{state: state}, 2, GenerationSSELeaseTTL)
	ctx := context.Background()

	if ok, err := first.Acquire(ctx, 7, "lease-a"); err != nil || !ok {
		t.Fatalf("first acquire = %v, %v", ok, err)
	}
	if ok, err := second.Acquire(ctx, 7, "lease-b"); err != nil || !ok {
		t.Fatalf("second replica must see shared count: %v, %v", ok, err)
	}
	if ok, err := first.Acquire(ctx, 7, "lease-c"); err != nil || ok {
		t.Fatalf("shared limit must reject the third stream: %v, %v", ok, err)
	}
	if ok, _ := second.Acquire(ctx, 8, "lease-d"); !ok {
		t.Fatal("different tenants must have independent budgets")
	}
	if err := first.Release(ctx, 7, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := second.Acquire(ctx, 7, "lease-c"); !ok {
		t.Fatal("released lease capacity must be visible to the other replica")
	}
	if err := second.Renew(ctx, 7, "lease-b"); err != nil {
		t.Fatal(err)
	}
	key := generationSSEKeyPrefix + "7"
	if state.keyTTL[key] != 90000 {
		t.Fatalf("lease ttl = %d, want 90000ms", state.keyTTL[key])
	}
	if _, exists := state.scores[key]["lease-b"]; !exists {
		t.Fatal("renew removed the lease member")
	}
	if state.calls == 0 {
		t.Fatal("fake redis was not exercised")
	}
}

func TestRedisGenerationSSELimiterCleansCrashedPodLeaseOnAcquire(t *testing.T) {
	state := &fakeRedisSSEState{scores: map[string]map[string]int64{}, keyTTL: map[string]int64{}}
	podA := newRedisGenerationSSELimiter(&fakeRedisSSEEvaler{state: state}, 1, GenerationSSELeaseTTL)
	podB := newRedisGenerationSSELimiter(&fakeRedisSSEEvaler{state: state}, 1, GenerationSSELeaseTTL)
	ctx := context.Background()
	key := generationSSEKeyPrefix + "9"

	if ok, err := podA.Acquire(ctx, 9, "crashed-lease"); err != nil || !ok {
		t.Fatalf("pod A acquire = %v, %v", ok, err)
	}
	// Simulate pod A death: its lease score is now in the past. The next Acquire
	// by pod B must purge it instead of rejecting a fresh stream forever.
	(&fakeRedisSSEEvaler{state: state}).expireLease(key, "crashed-lease")
	if ok, err := podB.Acquire(ctx, 9, "fresh-lease"); err != nil || !ok {
		t.Fatalf("pod B acquire after crash = %v, %v", ok, err)
	}
	if len(state.scores[key]) != 1 {
		t.Fatalf("lease set after cleanup = %v, want only fresh-lease", state.scores[key])
	}
	if _, exists := state.scores[key]["fresh-lease"]; !exists {
		t.Fatal("fresh lease missing after cleanup")
	}
}

func TestRedisGenerationSSELimiterBusyTenantCleanupKeepsCapacity(t *testing.T) {
	state := &fakeRedisSSEState{scores: map[string]map[string]int64{}, keyTTL: map[string]int64{}}
	podA := newRedisGenerationSSELimiter(&fakeRedisSSEEvaler{state: state}, 2, GenerationSSELeaseTTL)
	podB := newRedisGenerationSSELimiter(&fakeRedisSSEEvaler{state: state}, 2, GenerationSSELeaseTTL)
	ctx := context.Background()
	key := generationSSEKeyPrefix + "11"

	if ok, _ := podA.Acquire(ctx, 11, "live-lease"); !ok {
		t.Fatal("live lease not admitted")
	}
	if ok, _ := podA.Acquire(ctx, 11, "crashed-lease"); !ok {
		t.Fatal("crashed lease not admitted")
	}
	(&fakeRedisSSEEvaler{state: state}).expireLease(key, "crashed-lease")
	// Busy tenant: one live lease remains, the crashed one is purged, and a new
	// stream still fits within max=2.
	if ok, err := podB.Acquire(ctx, 11, "fresh-lease"); err != nil || !ok {
		t.Fatalf("busy tenant acquire after cleanup = %v, %v", ok, err)
	}
	if len(state.scores[key]) != 2 {
		t.Fatalf("lease set = %v, want live+fresh", state.scores[key])
	}
}

func TestRedisGenerationSSELimiterPropagatesRedisErrors(t *testing.T) {
	state := &fakeRedisSSEState{scores: map[string]map[string]int64{}, keyTTL: map[string]int64{}, evalErr: errors.New("redis down")}
	limiter := newRedisGenerationSSELimiter(&fakeRedisSSEEvaler{state: state}, 2, GenerationSSELeaseTTL)
	if _, err := limiter.Acquire(context.Background(), 7, "lease-a"); err == nil {
		t.Fatal("redis error must surface so the resilient wrapper can degrade")
	}
}

type failingSSELimiter struct{ err error }

func (f failingSSELimiter) Acquire(context.Context, int64, string) (bool, error) {
	return false, f.err
}
func (f failingSSELimiter) Release(context.Context, int64, string) error { return f.err }
func (f failingSSELimiter) Renew(context.Context, int64, string) error   { return f.err }

type scriptedSSELimiter struct {
	mu           sync.Mutex
	allow        map[string]bool
	errLeases    map[string]bool
	renewErrs    map[string]error
	acquisitions []string
	releases     []string
	renews       []string
}

func (s *scriptedSSELimiter) Acquire(_ context.Context, _ int64, leaseID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errLeases[leaseID] {
		return false, errors.New("primary unavailable")
	}
	if !s.allow[leaseID] {
		return false, nil
	}
	s.acquisitions = append(s.acquisitions, leaseID)
	return true, nil
}

func (s *scriptedSSELimiter) Release(_ context.Context, _ int64, leaseID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releases = append(s.releases, leaseID)
	return nil
}

func (s *scriptedSSELimiter) Renew(_ context.Context, _ int64, leaseID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renews = append(s.renews, leaseID)
	if err := s.renewErrs[leaseID]; err != nil {
		return err
	}
	return nil
}

func TestResilientGenerationSSELimiterDegradesToFallback(t *testing.T) {
	fallback := &scriptedSSELimiter{allow: map[string]bool{"lease-a": true}}
	resilient := NewResilientGenerationSSELimiter(failingSSELimiter{err: errors.New("redis down")}, fallback, nil)
	ctx := context.Background()
	admitted, err := resilient.Acquire(ctx, 7, "lease-a")
	if err != nil || !admitted {
		t.Fatalf("fallback acquire = %v, %v", admitted, err)
	}
	if err := resilient.Renew(ctx, 7, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if err := resilient.Release(ctx, 7, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if len(fallback.releases) != 1 || len(fallback.renews) != 1 {
		t.Fatalf("fallback release/renew = %v/%v", fallback.releases, fallback.renews)
	}
}

func TestResilientGenerationSSELimiterMixedOriginsReleaseOnlyTheirSource(t *testing.T) {
	primary := &scriptedSSELimiter{
		allow:     map[string]bool{"primary-lease": true},
		errLeases: map[string]bool{"fallback-lease": true},
	}
	fallback := &scriptedSSELimiter{allow: map[string]bool{"fallback-lease": true}}
	resilient := NewResilientGenerationSSELimiter(primary, fallback, nil)
	ctx := context.Background()
	if ok, err := resilient.Acquire(ctx, 7, "primary-lease"); err != nil || !ok {
		t.Fatalf("primary lease acquire = %v, %v", ok, err)
	}
	if ok, err := resilient.Acquire(ctx, 7, "fallback-lease"); err != nil || !ok {
		t.Fatalf("fallback lease acquire = %v, %v", ok, err)
	}
	if err := resilient.Release(ctx, 7, "primary-lease"); err != nil {
		t.Fatal(err)
	}
	if err := resilient.Renew(ctx, 7, "fallback-lease"); err != nil {
		t.Fatal(err)
	}
	if err := resilient.Release(ctx, 7, "fallback-lease"); err != nil {
		t.Fatal(err)
	}
	if len(primary.releases) != 1 || primary.releases[0] != "primary-lease" {
		t.Fatalf("primary releases = %v, want only primary-lease", primary.releases)
	}
	if len(fallback.releases) != 1 || fallback.releases[0] != "fallback-lease" {
		t.Fatalf("fallback releases = %v, want only fallback-lease", fallback.releases)
	}
	if len(primary.renews) != 0 {
		t.Fatalf("primary renews = %v, want none", primary.renews)
	}
	if len(fallback.renews) != 1 || fallback.renews[0] != "fallback-lease" {
		t.Fatalf("fallback renews = %v, want fallback-lease", fallback.renews)
	}
}

// TestRedisGenerationSSELimiterRealRedis exercises the actual ZSET Lua scripts
// against a real Redis when GP_TEST_REDIS_ADDR is provided (rehearsal only).
func TestRedisGenerationSSELimiterRealRedis(t *testing.T) {
	addr := os.Getenv("GP_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("GP_TEST_REDIS_ADDR is not set; real Redis limiter check requires a scratch Redis")
	}
	firstClient := redis.NewClient(&redis.Options{Addr: addr})
	defer firstClient.Close()
	secondClient := redis.NewClient(&redis.Options{Addr: addr})
	defer secondClient.Close()
	first := NewRedisGenerationSSELimiter(firstClient, 2, GenerationSSELeaseTTL)
	second := NewRedisGenerationSSELimiter(secondClient, 2, GenerationSSELeaseTTL)
	exerciseRealRedisLimiter(t, firstClient, first, second)
}

func exerciseRealRedisLimiter(t *testing.T, client *redis.Client, first, second *RedisGenerationSSELimiter) {
	t.Helper()
	ctx := context.Background()
	const teamID = int64(990022)
	key := generationSSEKeyPrefix + "990022"
	client.Del(ctx, key)
	defer client.Del(ctx, key)
	assertRealRedisSharedLimit(ctx, t, client, first, second, teamID, key)
	assertRealRedisRenewAndLost(ctx, t, client, first, second, teamID, key)
}

func assertRealRedisSharedLimit(ctx context.Context, t *testing.T, client *redis.Client, first, second *RedisGenerationSSELimiter, teamID int64, key string) {
	t.Helper()
	if ok, err := first.Acquire(ctx, teamID, "lease-a"); err != nil || !ok {
		t.Fatalf("first acquire = %v, %v", ok, err)
	}
	if ok, err := second.Acquire(ctx, teamID, "lease-b"); err != nil || !ok {
		t.Fatalf("second replica acquire = %v, %v", ok, err)
	}
	if ok, err := first.Acquire(ctx, teamID, "lease-c"); err != nil || ok {
		t.Fatalf("shared limit did not reject third: %v, %v", ok, err)
	}
	// Simulate a crashed pod lease and confirm the next Acquire purges it.
	past := time.Now().Add(-time.Minute).UnixMilli()
	if err := client.ZAdd(ctx, key, redis.Z{Score: float64(past), Member: "crashed-lease"}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := second.Release(ctx, teamID, "lease-b"); err != nil {
		t.Fatal(err)
	}
	if ok, err := second.Acquire(ctx, teamID, "fresh-lease"); err != nil || !ok {
		t.Fatalf("acquire after crashed lease cleanup = %v, %v", ok, err)
	}
	card, err := client.ZCard(ctx, key).Result()
	if err != nil || card != 2 {
		t.Fatalf("zset cardinality = %d, %v; want 2 (lease-a + fresh-lease)", card, err)
	}
}

func assertRealRedisRenewAndLost(ctx context.Context, t *testing.T, client *redis.Client, first, second *RedisGenerationSSELimiter, teamID int64, key string) {
	t.Helper()
	if ttl, err := client.TTL(ctx, key).Result(); err != nil || ttl <= 0 || ttl > GenerationSSELeaseTTL {
		t.Fatalf("lease ttl = %v, %v", ttl, err)
	}
	if err := first.Renew(ctx, teamID, "lease-a"); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if err := first.Release(ctx, teamID, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if err := second.Release(ctx, teamID, "fresh-lease"); err != nil {
		t.Fatal(err)
	}
	if err := first.Renew(ctx, teamID, "lease-gone"); !errors.Is(err, domain.ErrGenerationSSELeaseLost) {
		t.Fatalf("renew of missing lease = %v, want ErrGenerationSSELeaseLost", err)
	}
	var _ domain.GenerationSSELimiter = first
}

func TestRedisGenerationSSELimiterRenewLostReturnsTypedError(t *testing.T) {
	state := &fakeRedisSSEState{scores: map[string]map[string]int64{}, keyTTL: map[string]int64{}}
	limiter := newRedisGenerationSSELimiter(&fakeRedisSSEEvaler{state: state}, 2, GenerationSSELeaseTTL)
	ctx := context.Background()
	if ok, err := limiter.Acquire(ctx, 7, "lease-a"); err != nil || !ok {
		t.Fatal("acquire failed")
	}
	if err := limiter.Release(ctx, 7, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if err := limiter.Renew(ctx, 7, "lease-a"); !errors.Is(err, domain.ErrGenerationSSELeaseLost) {
		t.Fatalf("renew missing lease error = %v, want ErrGenerationSSELeaseLost", err)
	}
}

func TestResilientGenerationSSELimiterRenewSwitchesToFallback(t *testing.T) {
	primary := &scriptedSSELimiter{
		allow:     map[string]bool{"lease-a": true},
		renewErrs: map[string]error{"lease-a": errors.New("redis down")},
	}
	fallback := &scriptedSSELimiter{allow: map[string]bool{"lease-a": true}}
	resilient := NewResilientGenerationSSELimiter(primary, fallback, nil)
	ctx := context.Background()
	if ok, err := resilient.Acquire(ctx, 7, "lease-a"); err != nil || !ok {
		t.Fatalf("primary acquire = %v, %v", ok, err)
	}
	if err := resilient.Renew(ctx, 7, "lease-a"); err != nil {
		t.Fatalf("renew should switch to fallback: %v", err)
	}
	if len(fallback.acquisitions) != 1 {
		t.Fatalf("fallback acquisitions = %v, want the lease re-admitted", fallback.acquisitions)
	}
	if len(primary.releases) != 1 {
		t.Fatalf("primary best-effort release = %v, want the lost lease released", primary.releases)
	}
	// Later Renew/Release must use the fallback origin only.
	if err := resilient.Renew(ctx, 7, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if err := resilient.Release(ctx, 7, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if len(primary.renews) != 1 {
		t.Fatalf("primary renews = %d, want only the failed attempt", len(primary.renews))
	}
	if len(fallback.renews) != 1 {
		t.Fatalf("fallback renews = %d, want 1 (the post-switch renewal)", len(fallback.renews))
	}
	if len(fallback.releases) != 1 {
		t.Fatalf("fallback releases = %d, want 1", len(fallback.releases))
	}
}

func TestResilientGenerationSSELimiterLostLeaseReacquiresPrimary(t *testing.T) {
	primary := &scriptedSSELimiter{
		allow:     map[string]bool{"lease-a": true},
		renewErrs: map[string]error{"lease-a": domain.ErrGenerationSSELeaseLost},
	}
	fallback := &scriptedSSELimiter{allow: map[string]bool{"lease-a": true}}
	resilient := NewResilientGenerationSSELimiter(primary, fallback, nil)
	ctx := context.Background()
	if ok, err := resilient.Acquire(ctx, 7, "lease-a"); err != nil || !ok {
		t.Fatalf("primary acquire = %v, %v", ok, err)
	}
	if err := resilient.Renew(ctx, 7, "lease-a"); err != nil {
		t.Fatalf("lost lease should be re-acquired on primary: %v", err)
	}
	if len(primary.acquisitions) != 2 {
		t.Fatalf("primary acquisitions = %v, want initial + re-acquire", primary.acquisitions)
	}
	if len(fallback.acquisitions) != 0 {
		t.Fatalf("fallback must not be used when primary re-acquire works: %v", fallback.acquisitions)
	}
	if err := resilient.Release(ctx, 7, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if len(primary.releases) != 1 || len(fallback.releases) != 0 {
		t.Fatalf("release primary/fallback = %v/%v", primary.releases, fallback.releases)
	}
}

func TestResilientGenerationSSELimiterLostLeaseFallsBack(t *testing.T) {
	primary := &scriptedSSELimiter{
		allow:     map[string]bool{"lease-a": true},
		renewErrs: map[string]error{"lease-a": domain.ErrGenerationSSELeaseLost},
	}
	fallback := &scriptedSSELimiter{allow: map[string]bool{"lease-a": true}}
	resilient := NewResilientGenerationSSELimiter(primary, fallback, nil)
	ctx := context.Background()
	if ok, err := resilient.Acquire(ctx, 7, "lease-a"); err != nil || !ok {
		t.Fatalf("primary acquire = %v, %v", ok, err)
	}
	primary.allow["lease-a"] = false // primary re-acquire rejected
	if err := resilient.Renew(ctx, 7, "lease-a"); err != nil {
		t.Fatalf("lost lease should fall back: %v", err)
	}
	if len(fallback.acquisitions) != 1 {
		t.Fatalf("fallback acquisitions = %v", fallback.acquisitions)
	}
	if err := resilient.Renew(ctx, 7, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if len(primary.renews) != 1 {
		t.Fatalf("primary renews = %d, want only the lost attempt", len(primary.renews))
	}
	if err := resilient.Release(ctx, 7, "lease-a"); err != nil {
		t.Fatal(err)
	}
	if len(fallback.releases) != 1 {
		t.Fatalf("fallback releases = %v", fallback.releases)
	}
}

func TestResilientGenerationSSELimiterRenewFailsWhenBothSourcesFail(t *testing.T) {
	primary := &scriptedSSELimiter{
		allow:     map[string]bool{"lease-a": true},
		renewErrs: map[string]error{"lease-a": domain.ErrGenerationSSELeaseLost},
	}
	fallback := &scriptedSSELimiter{allow: map[string]bool{"lease-a": false}}
	resilient := NewResilientGenerationSSELimiter(primary, fallback, nil)
	ctx := context.Background()
	if ok, err := resilient.Acquire(ctx, 7, "lease-a"); err != nil || !ok {
		t.Fatalf("primary acquire = %v, %v", ok, err)
	}
	primary.allow["lease-a"] = false // primary re-acquire rejected too
	if err := resilient.Renew(ctx, 7, "lease-a"); err == nil {
		t.Fatal("both sources losing the lease must return an error")
	} else if !errors.Is(err, domain.ErrGenerationSSELeaseLost) {
		t.Fatalf("error = %v, want wrapped ErrGenerationSSELeaseLost", err)
	}
	if len(fallback.acquisitions) != 0 {
		t.Fatalf("fallback should not admit: %v", fallback.acquisitions)
	}
}
