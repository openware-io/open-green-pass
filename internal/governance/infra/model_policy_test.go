package infra

import (
	"context"
	"errors"
	"testing"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
)

func TestStaticModelPolicy(t *testing.T) {
	policy := NewStaticModelPolicy("openai:gpt-4o")
	if err := policy.ValidateBinding(context.Background(), 100, 1, domain.ModelBinding{Provider: "openai", Model: "gpt-4o"}); err != nil {
		t.Fatal(err)
	}
	if err := policy.ValidateBinding(context.Background(), 100, 1, domain.ModelBinding{Provider: "openai", Model: "other"}); !errors.Is(err, ErrModelNotApproved) {
		t.Fatalf("err=%v, want model not approved", err)
	}
}
