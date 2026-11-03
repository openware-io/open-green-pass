package infra

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// GenerationRunEventChannel is the PostgreSQL notification channel fed by the
// gen_run_event INSERT trigger (migration 000064).
const GenerationRunEventChannel = "gp_gen_run_event"

// RunEventNotifier owns one LISTEN connection per server process and fans the
// run-id hints out to local SSE subscribers. If the connection drops, it
// reconnects with bounded backoff; subscribers fall back to their own
// low-frequency DB poll until the listener is ready again.
type RunEventNotifier struct {
	pool    *pgxpool.Pool
	log     *slog.Logger
	mu      sync.RWMutex
	subs    map[int64]map[chan struct{}]struct{}
	ready   atomic.Bool
	backoff time.Duration // first reconnect delay (tests override)
}

// NewRunEventNotifier creates the notifier. Start must run in its own goroutine.
func NewRunEventNotifier(pool *pgxpool.Pool, log *slog.Logger) *RunEventNotifier {
	if log == nil {
		log = slog.Default()
	}
	return &RunEventNotifier{
		pool: pool, log: log, subs: map[int64]map[chan struct{}]struct{}{},
		backoff: time.Second,
	}
}

// Ready reports whether the LISTEN connection is established. Subscribe returns
// ErrGenerationRunEventNotifierUnavailable while false.
func (n *RunEventNotifier) Ready() bool { return n != nil && n.ready.Load() }

// Start blocks until ctx is cancelled, maintaining the LISTEN connection.
func (n *RunEventNotifier) Start(ctx context.Context) {
	if n == nil || n.pool == nil {
		return
	}
	delay := n.backoff
	if delay <= 0 {
		delay = time.Second
	}
	for {
		if ctx.Err() != nil {
			n.ready.Store(false)
			return
		}
		conn, err := n.pool.Acquire(ctx)
		if err != nil {
			n.sleep(ctx, delay)
			delay = nextBackoff(delay)
			continue
		}
		if _, err := conn.Exec(ctx, "LISTEN "+GenerationRunEventChannel); err != nil {
			n.log.WarnContext(ctx, "generation run event listener failed", "error", err)
			conn.Release()
			n.sleep(ctx, delay)
			delay = nextBackoff(delay)
			continue
		}
		n.ready.Store(true)
		delay = n.backoff
		if delay <= 0 {
			delay = time.Second
		}
		n.log.InfoContext(ctx, "generation run event listener started", "channel", GenerationRunEventChannel)
		listenErr := n.listen(ctx, conn)
		n.ready.Store(false)
		conn.Release()
		if ctx.Err() != nil {
			return
		}
		n.log.WarnContext(ctx, "generation run event listener stopped; reconnecting", "error", listenErr, "retry_in", delay)
		n.sleep(ctx, delay)
		delay = nextBackoff(delay)
	}
}

func (n *RunEventNotifier) listen(ctx context.Context, conn *pgxpool.Conn) error {
	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		runID, err := strconv.ParseInt(notification.Payload, 10, 64)
		if err != nil || runID <= 0 {
			continue
		}
		n.dispatch(runID)
	}
}

func (n *RunEventNotifier) sleep(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func nextBackoff(delay time.Duration) time.Duration {
	const maxDelay = 30 * time.Second
	if delay <= 0 {
		return time.Second
	}
	delay *= 2
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

// Subscribe registers one coalesced hint channel for a run.
func (n *RunEventNotifier) Subscribe(_ context.Context, runID int64) (<-chan struct{}, func(), error) {
	if !n.Ready() {
		return nil, nil, domain.ErrGenerationRunEventNotifierUnavailable
	}
	channel := make(chan struct{}, 1)
	n.mu.Lock()
	if n.subs[runID] == nil {
		n.subs[runID] = map[chan struct{}]struct{}{}
	}
	n.subs[runID][channel] = struct{}{}
	n.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			n.mu.Lock()
			defer n.mu.Unlock()
			subscribers := n.subs[runID]
			delete(subscribers, channel)
			if len(subscribers) == 0 {
				delete(n.subs, runID)
			}
		})
	}
	return channel, cancel, nil
}

// dispatch wakes every local subscriber of one run. It coalesces bursts with a
// non-blocking send: one pending hint is enough because consumers re-read the
// authoritative event rows with after_seq.
func (n *RunEventNotifier) dispatch(runID int64) {
	n.mu.RLock()
	subscribers := make([]chan struct{}, 0, len(n.subs[runID]))
	for channel := range n.subs[runID] {
		subscribers = append(subscribers, channel)
	}
	n.mu.RUnlock()
	for _, channel := range subscribers {
		select {
		case channel <- struct{}{}:
		default:
		}
	}
}
