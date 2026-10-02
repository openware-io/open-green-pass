package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
)

var ErrModelNotApproved = errors.New("model is not approved for target")

// StaticModelPolicy is a deterministic reference policy for tests and local
// deployments. Production should replace it with a tenant-scoped repository
// backed by approval records and audit events.
type StaticModelPolicy struct {
	allowed map[string]struct{}
}

func NewStaticModelPolicy(models ...string) *StaticModelPolicy {
	allowed := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" {
			allowed[model] = struct{}{}
		}
	}
	return &StaticModelPolicy{allowed: allowed}
}

func (p *StaticModelPolicy) ValidateBinding(_ context.Context, _ int64, _ int64, binding domain.ModelBinding) error {
	if _, ok := p.allowed[binding.Provider+":"+binding.Model]; !ok {
		return fmt.Errorf("%w: %s:%s", ErrModelNotApproved, binding.Provider, binding.Model)
	}
	return nil
}

var _ domain.ModelBindingPolicy = (*StaticModelPolicy)(nil)
