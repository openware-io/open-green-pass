package domain

import (
	"testing"
	"time"
)

func TestModelConfigValidate(t *testing.T) {
	if err := (ModelConfig{Name: "GPT", Provider: "openai", ModelKey: "gpt-4o", Status: ModelApproved}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (ModelConfig{Provider: "openai", ModelKey: "gpt-4o", Status: ModelApproved}).Validate(); err == nil {
		t.Fatal("expected required name error")
	}
}

func TestPriceSnapshotValidate(t *testing.T) {
	now := time.Now()
	p := PriceSnapshot{TeamID: 1, ModelID: 2, Currency: "USD", EffectiveFrom: now, InputPer1K: 0.1, OutputPer1K: 0.2}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	end := now
	if err := (PriceSnapshot{TeamID: 1, ModelID: 2, Currency: "USD", EffectiveFrom: now, EffectiveTo: &end}).Validate(); err == nil {
		t.Fatal("expected invalid effective range")
	}
	if err := (PriceSnapshot{ModelID: 2, Currency: "USD", EffectiveFrom: now}).Validate(); err == nil {
		t.Fatal("expected team required")
	}
}
