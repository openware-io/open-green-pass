package pgpool

import (
	"context"
	"errors"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

// SelectorConfig controls pool liveness and reservation expiry.
type SelectorConfig struct {
	HeartbeatTTL time.Duration
	LeaseTTL     time.Duration
}

// Selector adapts the PostgreSQL registry to the execution domain port.
type Selector struct {
	registry *Registry
	config   SelectorConfig
	now      func() time.Time
}

// NewSelector validates and constructs a shared pool selector.
func NewSelector(registry *Registry, config SelectorConfig) (*Selector, error) {
	if registry == nil || config.HeartbeatTTL <= 0 || config.LeaseTTL <= 0 {
		return nil, errors.New("valid execution pool selector configuration is required")
	}
	return &Selector{registry: registry, config: config, now: time.Now}, nil
}
func (s *Selector) Reserve(ctx context.Context, request domain.ExecutionPoolRequest) (domain.ExecutionPoolLease, error) {
	lease, err := s.registry.Reserve(ctx, request.Resource, request.Units, s.now().UTC(), s.config.HeartbeatTTL, s.config.LeaseTTL)
	if err != nil {
		return domain.ExecutionPoolLease{}, err
	}
	return domain.ExecutionPoolLease{Token: lease.Token, PoolID: lease.PoolID, Units: lease.Units, FencingToken: lease.FencingToken}, nil
}
func (s *Selector) Release(ctx context.Context, lease domain.ExecutionPoolLease) error {
	return s.registry.Release(ctx, Lease{Token: lease.Token, PoolID: lease.PoolID, Units: lease.Units, FencingToken: lease.FencingToken})
}
