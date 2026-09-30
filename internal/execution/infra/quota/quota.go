// Package quota contains the scheduling primitives used before a runner is
// dispatched. MemoryQuota is deterministic and useful for tests/dev; the
// QuotaPort contract is intentionally storage-neutral for Redis Lua later.
package quota

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

var ErrQuotaExceeded = errors.New("execution quota exceeded")
var ErrInvalidRequest = errors.New("invalid resource request")

type Limits struct {
	Global int
	Team   int
	Target int
	Owner  int
}

type resourceUsage struct {
	global  int
	teams   map[int64]int
	targets map[string]int
	owners  map[string]int
}

// MemoryQuota atomically enforces global→team→target→owner limits. Keys are
// scoped by resource type, preventing browsers from consuming device slots.
type MemoryQuota struct {
	mu     sync.Mutex
	limits map[string]Limits
	used   map[string]*resourceUsage
}

func NewMemoryQuota(limits map[string]Limits) *MemoryQuota {
	copyLimits := make(map[string]Limits, len(limits))
	for k, v := range limits {
		copyLimits[k] = v
	}
	return &MemoryQuota{limits: copyLimits, used: make(map[string]*resourceUsage)}
}

func (q *MemoryQuota) Acquire(ctx context.Context, req domain.ResourceRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if req.Type == "" || req.Units <= 0 || req.TeamID <= 0 || req.TargetID <= 0 || req.OwnerID <= 0 {
		return ErrInvalidRequest
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	lim, ok := q.limits[req.Type]
	if !ok {
		return fmt.Errorf("%w: resource=%s", ErrQuotaExceeded, req.Type)
	}
	if req.Units > lim.Global {
		return ErrQuotaExceeded
	}
	u := q.used[req.Type]
	if u == nil {
		u = &resourceUsage{teams: make(map[int64]int), targets: make(map[string]int), owners: make(map[string]int)}
		q.used[req.Type] = u
	}
	tk, ok := u.teams[req.TeamID]
	if !ok {
		tk = 0
	}
	tr := u.targets[targetKey(req)]
	ow := u.owners[ownerKey(req)]
	if exceeds(u.global, req.Units, lim.Global) || exceeds(tk, req.Units, lim.Team) || exceeds(tr, req.Units, lim.Target) || exceeds(ow, req.Units, lim.Owner) {
		return ErrQuotaExceeded
	}
	u.global += req.Units
	u.teams[req.TeamID] = tk + req.Units
	u.targets[targetKey(req)] = tr + req.Units
	u.owners[ownerKey(req)] = ow + req.Units
	return nil
}

func (q *MemoryQuota) Release(ctx context.Context, req domain.ResourceRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if req.Type == "" || req.Units <= 0 || req.TeamID <= 0 || req.TargetID <= 0 || req.OwnerID <= 0 {
		return ErrInvalidRequest
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	u := q.used[req.Type]
	if u == nil {
		return nil
	}
	tk := max0(u.teams[req.TeamID] - req.Units)
	trk := targetKey(req)
	tr := max0(u.targets[trk] - req.Units)
	owk := ownerKey(req)
	ow := max0(u.owners[owk] - req.Units)
	u.global = max0(u.global - req.Units)
	if tk == 0 {
		delete(u.teams, req.TeamID)
	} else {
		u.teams[req.TeamID] = tk
	}
	if tr == 0 {
		delete(u.targets, trk)
	} else {
		u.targets[trk] = tr
	}
	if ow == 0 {
		delete(u.owners, owk)
	} else {
		u.owners[owk] = ow
	}
	if u.global == 0 {
		delete(q.used, req.Type)
	}
	return nil
}

func targetKey(r domain.ResourceRequest) string { return fmt.Sprintf("%d/%d", r.TeamID, r.TargetID) }
func ownerKey(r domain.ResourceRequest) string  { return fmt.Sprintf("%d/%d", r.TeamID, r.OwnerID) }
func exceeds(current, add, limit int) bool      { return limit > 0 && current+add > limit }
func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

var _ domain.QuotaPort = (*MemoryQuota)(nil)
