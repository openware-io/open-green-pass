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

// RepositoryModelPolicy is the persistent whitelist boundary. It resolves a
// binding against the tenant's catalogue and only permits explicitly approved
// entries; provider adapters are deliberately outside this policy.
type RepositoryModelPolicy struct {
	repo domain.ModelGovernanceRepository
}

func NewRepositoryModelPolicy(repo domain.ModelGovernanceRepository) *RepositoryModelPolicy {
	return &RepositoryModelPolicy{repo: repo}
}

func (p *RepositoryModelPolicy) ValidateBinding(ctx context.Context, teamID, _ int64, binding domain.ModelBinding) error {
	if p == nil || p.repo == nil {
		return fmt.Errorf("%w: model governance repository unavailable", ErrModelNotApproved)
	}
	if err := binding.Validate(); err != nil {
		return err
	}
	models, err := p.repo.ListModels(ctx, teamID)
	if err != nil {
		return err
	}
	for _, model := range models {
		if model != nil && model.Provider == binding.Provider && model.ModelKey == binding.Model && model.Status == domain.ModelApproved {
			return nil
		}
	}
	return fmt.Errorf("%w: %s:%s", ErrModelNotApproved, binding.Provider, binding.Model)
}

var _ domain.ModelBindingPolicy = (*RepositoryModelPolicy)(nil)

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
