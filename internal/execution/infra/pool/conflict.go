package pool

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

var (
	ErrInvalidClaim = errors.New("invalid resource claim")
	// ErrResourceConflict is kept as a compatibility alias for pool callers;
	// the source of truth lives in execution/domain.
	ErrResourceConflict = domain.ErrResourceConflict
	ErrClaimNotFound    = errors.New("resource claim not found")
)

type conflictKey struct {
	teamID       int64
	targetID     int64
	resourceType string
	poolID       string
}

type activeClaim struct {
	token string
	claim domain.ResourceClaim
}

// ConflictRegistry is an in-process reference implementation for resource
// mutual exclusion. Production deployments must replace its storage with a
// shared Redis/DB-backed implementation before enabling multiple schedulers.
type ConflictRegistry struct {
	mu     sync.Mutex
	claims map[conflictKey]activeClaim
}

func NewConflictRegistry() *ConflictRegistry {
	return &ConflictRegistry{claims: make(map[conflictKey]activeClaim)}
}

func (r *ConflictRegistry) Acquire(ctx context.Context, claim domain.ResourceClaim) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if claim.TeamID <= 0 || claim.TargetID <= 0 || claim.ResourceType == "" || claim.PoolID == "" {
		return "", ErrInvalidClaim
	}
	if !claim.Exclusive {
		return "", nil
	}
	key := conflictKey{teamID: claim.TeamID, targetID: claim.TargetID, resourceType: claim.ResourceType, poolID: claim.PoolID}
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, ok := r.claims[key]; ok {
		return "", fmt.Errorf("%w: pool=%s resource=%s owner=%d", ErrResourceConflict, current.claim.PoolID, current.claim.ResourceType, current.claim.OwnerID)
	}
	token, err := newClaimToken()
	if err != nil {
		return "", err
	}
	r.claims[key] = activeClaim{token: token, claim: claim}
	return token, nil
}

func (r *ConflictRegistry) Release(ctx context.Context, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if token == "" {
		// Empty token represents a non-exclusive claim, which does not reserve
		// registry state and therefore needs no release operation.
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, claim := range r.claims {
		if claim.token == token {
			delete(r.claims, key)
			return nil
		}
	}
	return ErrClaimNotFound
}

func newClaimToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

var _ domain.ConflictPort = (*ConflictRegistry)(nil)
