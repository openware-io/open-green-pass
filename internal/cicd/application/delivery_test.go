package application

import (
	"github.com/openware-io/open-green-pass/internal/cicd/domain"
	"testing"
	"time"
)

func TestMemoryInboxDeduplicatesProviderDelivery(t *testing.T) {
	m := NewMemoryInbox()
	d := domain.InboxDelivery{TeamID: 1, Provider: "fake", DeliveryKey: "d1", PayloadHash: PayloadHash([]byte("x"))}
	if err := m.Accept(d); err != nil {
		t.Fatal(err)
	}
	if err := m.Accept(d); err != domain.ErrDuplicateDelivery {
		t.Fatalf("err=%v", err)
	}
}
func TestRetryPolicyExponentialBackoff(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 3, BaseDelay: time.Second}
	now := time.Unix(0, 0)
	if next, ok := p.Next(2, now); !ok || next.Sub(now) != 2*time.Second {
		t.Fatalf("next=%v ok=%v", next, ok)
	}
	if _, ok := p.Next(3, now); ok {
		t.Fatal("max attempts exceeded")
	}
}
