package pgpool

import (
	"testing"
	"time"
)

func TestNewSelectorRejectsInvalidConfiguration(t *testing.T) {
	if _, err := NewSelector(nil, SelectorConfig{HeartbeatTTL: time.Second, LeaseTTL: time.Second}); err == nil {
		t.Fatal("expected nil registry to be rejected")
	}
	registry := &Registry{}
	if _, err := NewSelector(registry, SelectorConfig{}); err == nil {
		t.Fatal("expected zero TTLs to be rejected")
	}
}
