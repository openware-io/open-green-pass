package infra

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// GenerationSSELeaseTTL is the crash-recovery lease for one live SSE stream.
// Active streams renew it on heartbeat/snapshot ticks; if a server process dies
// without releasing, the per-lease ZSET score expires and the next Acquire on
// that tenant removes the stale member automatically.
const GenerationSSELeaseTTL = 90 * time.Second

// Each stream is one member of a per-tenant ZSET keyed by lease score (expiry in
// ms). Acquire first removes expired members, then enforces the limit, then adds
// the new lease and extends the key TTL. All three steps run atomically so a
// crashed replica can never leave a permanent count behind.
const acquireGenerationSSELua = `
local now = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
local current = redis.call('ZCARD', KEYS[1])
if current >= limit then
  return 0
end
redis.call('ZADD', KEYS[1], now + ttl, ARGV[4])
redis.call('PEXPIRE', KEYS[1], ttl)
return 1
`

const renewGenerationSSELua = `
local now = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
if redis.call('ZSCORE', KEYS[1], ARGV[3]) == false then
  return 0
end
redis.call('ZADD', KEYS[1], 'XX', now + ttl, ARGV[3])
redis.call('PEXPIRE', KEYS[1], ttl)
return 1
`

const releaseGenerationSSELua = `
local removed = redis.call('ZREM', KEYS[1], ARGV[1])
if redis.call('ZCARD', KEYS[1]) == 0 then
  redis.call('DEL', KEYS[1])
end
return removed
`

const generationSSEKeyPrefix = "gp:gen:sse:team:"

// redisEvaler is the narrow surface needed from go-redis; tests can fake it
// without a Redis server while production delegates to the shared gp-server
// Redis client. Lease scripts return integer verdicts only.
type redisEvaler interface {
	EvalInt(ctx context.Context, script string, keys []string, args ...any) (int64, error)
}

type goRedisEvaler struct{ client redis.Cmdable }

func (e goRedisEvaler) EvalInt(ctx context.Context, script string, keys []string, args ...any) (int64, error) {
	return e.client.Eval(ctx, script, keys, args...).Int64()
}

// RedisGenerationSSELimiter keeps one ZSET lease per live stream so multiple
// replicas share a tenant limit and crashed replicas' leases expire by score.
type RedisGenerationSSELimiter struct {
	eval      redisEvaler
	keyPrefix string
	max       int
	ttl       time.Duration
}

// NewRedisGenerationSSELimiter creates the limiter over the shared server Redis
// client (redis.Cmdable, e.g. *redis.Client).
func NewRedisGenerationSSELimiter(client redis.Cmdable, maxStreams int, ttl time.Duration) *RedisGenerationSSELimiter {
	return newRedisGenerationSSELimiter(goRedisEvaler{client: client}, maxStreams, ttl)
}

func newRedisGenerationSSELimiter(eval redisEvaler, maxStreams int, ttl time.Duration) *RedisGenerationSSELimiter {
	if maxStreams <= 0 {
		maxStreams = 8
	}
	if ttl <= 0 {
		ttl = GenerationSSELeaseTTL
	}
	return &RedisGenerationSSELimiter{eval: eval, keyPrefix: generationSSEKeyPrefix, max: maxStreams, ttl: ttl}
}

func (l *RedisGenerationSSELimiter) key(teamID int64) string {
	return l.keyPrefix + strconv.FormatInt(teamID, 10)
}

func (l *RedisGenerationSSELimiter) nowMillis() string {
	return strconv.FormatInt(time.Now().UnixMilli(), 10)
}

func (l *RedisGenerationSSELimiter) ttlMillis() string {
	return strconv.FormatInt(l.ttl.Milliseconds(), 10)
}

func (l *RedisGenerationSSELimiter) Acquire(ctx context.Context, teamID int64, leaseID string) (bool, error) {
	admitted, err := l.eval.EvalInt(ctx, acquireGenerationSSELua, []string{l.key(teamID)},
		l.nowMillis(), l.ttlMillis(), strconv.Itoa(l.max), leaseID)
	if err != nil {
		return false, err
	}
	return admitted == 1, nil
}

func (l *RedisGenerationSSELimiter) Release(ctx context.Context, teamID int64, leaseID string) error {
	_, err := l.eval.EvalInt(ctx, releaseGenerationSSELua, []string{l.key(teamID)}, leaseID)
	return err
}

func (l *RedisGenerationSSELimiter) Renew(ctx context.Context, teamID int64, leaseID string) error {
	renewed, err := l.eval.EvalInt(ctx, renewGenerationSSELua, []string{l.key(teamID)},
		l.nowMillis(), l.ttlMillis(), leaseID)
	if err != nil {
		return err
	}
	if renewed != 1 {
		// ZSCORE was empty: the lease score expired or the key was reclaimed.
		return domain.ErrGenerationSSELeaseLost
	}
	return nil
}

type leaseKey struct {
	teamID  int64
	leaseID string
}

type leaseOrigin int

const (
	leaseOriginUnknown leaseOrigin = iota
	leaseOriginPrimary
	leaseOriginFallback
)

// ResilientGenerationSSELimiter prefers Redis (shared across replicas) and
// degrades to the in-process limiter when Redis is unavailable. It records where
// each admitted lease came from so Release/Renew only touch that source; mixed
// primary/fallback streams must not release each other's slots.
type ResilientGenerationSSELimiter struct {
	primary  domain.GenerationSSELimiter
	fallback domain.GenerationSSELimiter
	log      *slog.Logger
	mu       sync.Mutex
	origins  map[leaseKey]leaseOrigin
}

func NewResilientGenerationSSELimiter(primary, fallback domain.GenerationSSELimiter, log *slog.Logger) *ResilientGenerationSSELimiter {
	if log == nil {
		log = slog.Default()
	}
	return &ResilientGenerationSSELimiter{
		primary: primary, fallback: fallback, log: log,
		origins: make(map[leaseKey]leaseOrigin),
	}
}

func (l *ResilientGenerationSSELimiter) Acquire(ctx context.Context, teamID int64, leaseID string) (bool, error) {
	if l.primary != nil {
		admitted, err := l.primary.Acquire(ctx, teamID, leaseID)
		if err == nil {
			if admitted {
				l.remember(teamID, leaseID, leaseOriginPrimary)
			}
			return admitted, nil
		}
		l.log.WarnContext(ctx, "generation sse redis limiter unavailable; using in-process fallback",
			"team_id", teamID, "lease_id", leaseID, "error", err)
	}
	admitted, err := l.fallback.Acquire(ctx, teamID, leaseID)
	if err == nil && admitted {
		l.remember(teamID, leaseID, leaseOriginFallback)
	}
	return admitted, err
}

func (l *ResilientGenerationSSELimiter) Release(ctx context.Context, teamID int64, leaseID string) error {
	origin := l.take(teamID, leaseID)
	switch origin {
	case leaseOriginPrimary:
		return l.primary.Release(ctx, teamID, leaseID)
	case leaseOriginFallback:
		return l.fallback.Release(ctx, teamID, leaseID)
	default:
		// Unknown origin (e.g. the wrapper restarted): releasing neither source
		// is safer than freeing another live stream's lease. The lease TTL
		// reclaims it.
		return nil
	}
}

func (l *ResilientGenerationSSELimiter) Renew(ctx context.Context, teamID int64, leaseID string) error {
	switch l.origin(teamID, leaseID) {
	case leaseOriginPrimary:
		err := l.primary.Renew(ctx, teamID, leaseID)
		if err == nil {
			return nil
		}
		if errors.Is(err, domain.ErrGenerationSSELeaseLost) {
			return l.reacquirePrimaryThenFallback(ctx, teamID, leaseID, err)
		}
		// Primary (Redis) is failing: move this live stream to the degraded
		// in-process limiter so the stream keeps being counted somewhere.
		return l.switchToFallback(ctx, teamID, leaseID, "renew_primary", err)
	case leaseOriginFallback:
		err := l.fallback.Renew(ctx, teamID, leaseID)
		if err == nil {
			return nil
		}
		// The fallback lease is lost/unrenewable: re-admit on fallback first, then
		// try the primary connection, and only fail if neither can hold it.
		if admitted, acquireErr := l.fallback.Acquire(ctx, teamID, leaseID); acquireErr == nil && admitted {
			l.remember(teamID, leaseID, leaseOriginFallback)
			return nil
		}
		if admitted, acquireErr := l.primary.Acquire(ctx, teamID, leaseID); acquireErr == nil && admitted {
			l.remember(teamID, leaseID, leaseOriginPrimary)
			return nil
		}
		l.log.WarnContext(ctx, "generation sse lease renewal failed on both sources",
			"team_id", teamID, "lease_id", leaseID, "origin", "fallback", "error", err)
		return fmt.Errorf("generation sse lease renewal failed: %w", err)
	default:
		// Unknown origin (e.g. wrapper restarted): do not silently assume a lease
		// still exists; the next Acquire/reconnect can re-admit it.
		return nil
	}
}

// reacquirePrimaryThenFallback handles a lost Redis lease: first try to get a
// fresh primary lease, then degrade to the in-process limiter. It never returns
// nil when neither source admitted the lease.
func (l *ResilientGenerationSSELimiter) reacquirePrimaryThenFallback(ctx context.Context, teamID int64, leaseID string, cause error) error {
	if admitted, err := l.primary.Acquire(ctx, teamID, leaseID); err == nil && admitted {
		l.remember(teamID, leaseID, leaseOriginPrimary)
		return nil
	} else if err != nil {
		l.log.WarnContext(ctx, "generation sse lease lost; primary re-acquire failed",
			"team_id", teamID, "lease_id", leaseID, "error", err)
	}
	return l.switchToFallback(ctx, teamID, leaseID, "lease_lost", cause)
}

// switchToFallback re-admits the lease on the in-process limiter, records the new
// origin and best-effort releases the primary membership (the lease is lost or
// the primary connection is failing, so errors there are not fatal).
func (l *ResilientGenerationSSELimiter) switchToFallback(ctx context.Context, teamID int64, leaseID, stage string, cause error) error {
	admitted, err := l.fallback.Acquire(ctx, teamID, leaseID)
	if err != nil || !admitted {
		l.log.WarnContext(ctx, "generation sse lease cannot be admitted on fallback",
			"team_id", teamID, "lease_id", leaseID, "stage", stage, "error", err, "cause", cause)
		if err != nil {
			return fmt.Errorf("generation sse lease renewal failed on fallback: %w", err)
		}
		return fmt.Errorf("generation sse lease renewal failed on fallback: %w", cause)
	}
	l.remember(teamID, leaseID, leaseOriginFallback)
	if l.primary != nil {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if releaseErr := l.primary.Release(releaseCtx, teamID, leaseID); releaseErr != nil {
			l.log.WarnContext(ctx, "generation sse primary lease best-effort release failed",
				"team_id", teamID, "lease_id", leaseID, "error", releaseErr)
		}
	}
	return nil
}

func (l *ResilientGenerationSSELimiter) origin(teamID int64, leaseID string) leaseOrigin {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.origins[leaseKey{teamID: teamID, leaseID: leaseID}]
}

func (l *ResilientGenerationSSELimiter) remember(teamID int64, leaseID string, origin leaseOrigin) {
	l.mu.Lock()
	l.origins[leaseKey{teamID: teamID, leaseID: leaseID}] = origin
	l.mu.Unlock()
}

func (l *ResilientGenerationSSELimiter) take(teamID int64, leaseID string) leaseOrigin {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := leaseKey{teamID: teamID, leaseID: leaseID}
	origin := l.origins[key]
	delete(l.origins, key)
	return origin
}
