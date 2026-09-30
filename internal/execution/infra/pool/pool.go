// Package pool models registered execution clusters. Scheduling can select a
// pool by resource capability without knowing whether it is K8s, Playwright,
// STF or an external provider.
package pool

import (
	"errors"
	"sort"
	"sync"
)

var ErrPoolNotFound = errors.New("execution pool not found")
var ErrPoolUnavailable = errors.New("execution pool unavailable")

type Registration struct {
	ID         string
	Cluster    string
	Resources  []string
	Endpoint   string
	Capacity   int
	Available  int
	Enabled    bool
}

type Registry struct {
	mu    sync.RWMutex
	pools map[string]Registration
}

func NewRegistry() *Registry { return &Registry{pools: make(map[string]Registration)} }

func (r *Registry) Upsert(p Registration) error {
	if p.ID == "" || p.Cluster == "" || p.Capacity <= 0 || p.Available < 0 || p.Available > p.Capacity { return ErrPoolUnavailable }
	p.Resources = append([]string(nil), p.Resources...)
	sort.Strings(p.Resources)
	r.mu.Lock(); defer r.mu.Unlock(); r.pools[p.ID] = p
	return nil
}

func (r *Registry) Get(id string) (Registration, error) { r.mu.RLock(); defer r.mu.RUnlock(); p, ok := r.pools[id]; if !ok { return Registration{}, ErrPoolNotFound }; return clone(p), nil }

func (r *Registry) Select(resource string) (Registration, error) {
	r.mu.RLock(); defer r.mu.RUnlock()
	var selected Registration
	for _, p := range r.pools { if !p.Enabled || p.Available == 0 || !contains(p.Resources, resource) { continue }; if selected.ID == "" || p.Available > selected.Available { selected = p } }
	if selected.ID == "" { return Registration{}, ErrPoolNotFound }
	return clone(selected), nil
}

func (r *Registry) Reserve(id string, units int) error {
	if units <= 0 { return ErrPoolUnavailable }
	r.mu.Lock(); defer r.mu.Unlock(); p, ok := r.pools[id]; if !ok { return ErrPoolNotFound }; if !p.Enabled || p.Available < units { return ErrPoolUnavailable }; p.Available -= units; r.pools[id] = p; return nil
}

func (r *Registry) Release(id string, units int) error {
	if units <= 0 { return ErrPoolUnavailable }
	r.mu.Lock(); defer r.mu.Unlock(); p, ok := r.pools[id]; if !ok { return ErrPoolNotFound }; p.Available += units; if p.Available > p.Capacity { p.Available = p.Capacity }; r.pools[id] = p; return nil
}

func contains(xs []string, want string) bool { for _, x := range xs { if x == want { return true } }; return false }
func clone(p Registration) Registration { p.Resources = append([]string(nil), p.Resources...); return p }
