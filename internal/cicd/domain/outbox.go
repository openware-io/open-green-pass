package domain

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidDelivery = errors.New("cicd: invalid delivery")
var ErrDuplicateDelivery = errors.New("cicd: duplicate delivery")

type DeliveryStatus string

const (
	DeliveryPending     DeliveryStatus = "pending"
	DeliveryDispatching DeliveryStatus = "dispatching"
	DeliveryDelivered   DeliveryStatus = "delivered"
	DeliveryFailed      DeliveryStatus = "failed"
)

type InboxDelivery struct {
	TeamID      int64
	Provider    string
	ConnectorID int64
	DeliveryKey string
	PayloadHash string
	ReceivedAt  time.Time
}

type OutboxDelivery struct {
	TeamID        int64
	EventID       string
	DeliveryKey   string
	Provider      string
	Attempt       int
	NextAttemptAt time.Time
	Status        DeliveryStatus
	LastError     string
}

func (d InboxDelivery) Validate() error {
	if d.TeamID <= 0 || d.Provider == "" || d.DeliveryKey == "" || d.PayloadHash == "" {
		return ErrInvalidDelivery
	}
	return nil
}
func (d OutboxDelivery) Validate() error {
	if d.TeamID <= 0 || d.EventID == "" || d.DeliveryKey == "" || d.Provider == "" || d.Attempt < 0 {
		return ErrInvalidDelivery
	}
	switch d.Status {
	case DeliveryPending, DeliveryDispatching, DeliveryDelivered, DeliveryFailed:
		return nil
	}
	return ErrInvalidDelivery
}

type InboxPort interface{ Accept(InboxDelivery) error }
type OutboxPort interface {
	Enqueue(context.Context, OutboxDelivery) error
	Claim(context.Context, time.Time) (*OutboxDelivery, error)
	MarkDelivered(context.Context, string) error
	MarkFailed(context.Context, string, string, time.Time) error
	ReclaimStale(context.Context, time.Time, time.Duration) error
}

// SignatureVerifier authenticates the raw provider payload before decoding it.
// Implementations must not log the payload or credential.
type SignatureVerifier interface {
	Verify(payload []byte, signature string, timestamp time.Time, now time.Time) error
}
