package application

import (
	"errors"
	"fmt"
	"time"

	"github.com/openware-io/open-green-pass/internal/cicd/domain"
)

var ErrConnectorUnavailable = errors.New("cicd: connector unavailable")

// Connector is the provider-neutral delivery boundary. Implementations must
// use DeliveryKey as their idempotency key and must not mutate the outbox.
type Connector interface {
	Deliver(domain.OutboxDelivery) error
}

// Dispatcher claims one due outbox item, delivers it, and records the durable
// result. It is safe to call from a worker loop because claim and status writes
// remain owned by the OutboxPort.
type Dispatcher struct {
	Outbox     domain.OutboxPort
	Connectors map[string]Connector
	Retry      RetryPolicy
	Now        func() time.Time
}

func (d *Dispatcher) DispatchOne() (bool, error) {
	if d == nil || d.Outbox == nil {
		return false, ErrConnectorUnavailable
	}
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	delivery, err := d.Outbox.Claim(now)
	if err != nil {
		return false, fmt.Errorf("claim outbox delivery: %w", err)
	}
	if delivery == nil {
		return false, nil
	}
	connector := d.Connectors[delivery.Provider]
	if connector == nil {
		return true, d.fail(*delivery, ErrConnectorUnavailable.Error(), now)
	}
	if err := connector.Deliver(*delivery); err != nil {
		return true, d.fail(*delivery, err.Error(), now)
	}
	if err := d.Outbox.MarkDelivered(delivery.DeliveryKey); err != nil {
		return true, fmt.Errorf("mark delivery %s delivered: %w", delivery.DeliveryKey, err)
	}
	return true, nil
}

func (d *Dispatcher) fail(delivery domain.OutboxDelivery, reason string, now time.Time) error {
	updated, retry := d.Retry.ApplyFailure(delivery, reason, now)
	if retry {
		return d.Outbox.MarkFailed(updated.DeliveryKey, updated.LastError, updated.NextAttemptAt)
	}
	// OutboxPort has one failure method; a zero retry time is the durable
	// terminal marker and implementations must persist status=failed.
	return d.Outbox.MarkFailed(updated.DeliveryKey, updated.LastError, time.Time{})
}
