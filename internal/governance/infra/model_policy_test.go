package infra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
)

type modelPolicyRepo struct{ models []*domain.ModelConfig }

func (r modelPolicyRepo) CreateModel(context.Context, *domain.ModelConfig) error { return nil }
func (r modelPolicyRepo) FindModel(context.Context, int64, int64) (*domain.ModelConfig, error) {
	return nil, nil
}
func (r modelPolicyRepo) ListModels(_ context.Context, teamID int64) ([]*domain.ModelConfig, error) {
	var out []*domain.ModelConfig
	for _, model := range r.models {
		if model != nil && model.TeamID == teamID {
			out = append(out, model)
		}
	}
	return out, nil
}
func (r modelPolicyRepo) AddPrice(context.Context, *domain.PriceSnapshot) error { return nil }
func (r modelPolicyRepo) ResolvePrice(context.Context, int64, int64, time.Time) (*domain.PriceSnapshot, error) {
	return nil, nil
}

func TestStaticModelPolicy(t *testing.T) {
	policy := NewStaticModelPolicy("openai:gpt-4o")
	if err := policy.ValidateBinding(context.Background(), 100, 1, domain.ModelBinding{Provider: "openai", Model: "gpt-4o"}); err != nil {
		t.Fatal(err)
	}
	if err := policy.ValidateBinding(context.Background(), 100, 1, domain.ModelBinding{Provider: "openai", Model: "other"}); !errors.Is(err, ErrModelNotApproved) {
		t.Fatalf("err=%v, want model not approved", err)
	}
}

func TestRepositoryModelPolicyRequiresApprovedTenantModel(t *testing.T) {
	policy := NewRepositoryModelPolicy(modelPolicyRepo{models: []*domain.ModelConfig{
		{TeamID: 100, Provider: "openai", ModelKey: "gpt-4o", Status: domain.ModelPending},
		{TeamID: 100, Provider: "deepseek", ModelKey: "deepseek-chat", Status: domain.ModelApproved},
	}})
	if err := policy.ValidateBinding(context.Background(), 100, 1, domain.ModelBinding{Provider: "deepseek", Model: "deepseek-chat"}); err != nil {
		t.Fatal(err)
	}
	if err := policy.ValidateBinding(context.Background(), 100, 1, domain.ModelBinding{Provider: "openai", Model: "gpt-4o"}); !errors.Is(err, ErrModelNotApproved) {
		t.Fatalf("err=%v, want pending model rejected", err)
	}
	if err := policy.ValidateBinding(context.Background(), 200, 1, domain.ModelBinding{Provider: "deepseek", Model: "deepseek-chat"}); !errors.Is(err, ErrModelNotApproved) {
		t.Fatalf("err=%v, want cross-tenant model rejected", err)
	}
}
