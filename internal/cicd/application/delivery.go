package application

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/openware-io/open-green-pass/internal/cicd/domain"
)

type MemoryInbox struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func NewMemoryInbox() *MemoryInbox { return &MemoryInbox{seen: map[string]struct{}{}} }
func (m *MemoryInbox) Accept(d domain.InboxDelivery) error {
	if err := d.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := d.Provider + ":" + d.DeliveryKey
	if _, ok := m.seen[key]; ok {
		return domain.ErrDuplicateDelivery
	}
	m.seen[key] = struct{}{}
	return nil
}

func PayloadHash(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

func (p RetryPolicy) Next(attempt int, now time.Time) (time.Time, bool) {
	if p.MaxAttempts <= 0 || attempt >= p.MaxAttempts {
		return time.Time{}, false
	}
	delay := p.BaseDelay
	if delay <= 0 {
		delay = time.Second
	}
	for i := 1; i < attempt; i++ {
		delay *= 2
	}
	return now.Add(delay), true
}
