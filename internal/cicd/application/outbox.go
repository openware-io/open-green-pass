package application

import (
	"sync"
	"time"

	"github.com/openware-io/open-green-pass/internal/cicd/domain"
)

// MemoryOutbox is a deterministic contract-test adapter. Production workers
// must provide a durable implementation with the same claim/ack semantics.
type MemoryOutbox struct {
	mu    sync.Mutex
	items map[string]domain.OutboxDelivery
}

func NewMemoryOutbox() *MemoryOutbox {
	return &MemoryOutbox{items: make(map[string]domain.OutboxDelivery)}
}

func (o *MemoryOutbox) Enqueue(d domain.OutboxDelivery) error {
	if d.Status == "" {
		d.Status = domain.DeliveryPending
	}
	if err := d.Validate(); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.items[d.DeliveryKey]; ok {
		return domain.ErrDuplicateDelivery
	}
	o.items[d.DeliveryKey] = d
	return nil
}

func (o *MemoryOutbox) Claim(now time.Time) (*domain.OutboxDelivery, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for key, item := range o.items {
		if item.Status != domain.DeliveryPending || item.NextAttemptAt.After(now) {
			continue
		}
		item.Attempt++
		o.items[key] = item
		copy := item
		return &copy, nil
	}
	return nil, nil
}

func (o *MemoryOutbox) MarkDelivered(key string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	item, ok := o.items[key]
	if !ok {
		return domain.ErrInvalidDelivery
	}
	item.Status = domain.DeliveryDelivered
	o.items[key] = item
	return nil
}

func (o *MemoryOutbox) MarkFailed(key, reason string, next time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	item, ok := o.items[key]
	if !ok {
		return domain.ErrInvalidDelivery
	}
	if next.IsZero() {
		item.Status = domain.DeliveryFailed
	} else {
		item.Status = domain.DeliveryPending
	}
	item.LastError = reason
	item.NextAttemptAt = next
	o.items[key] = item
	return nil
}
