package infra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openware-io/open-green-pass/internal/cicd/domain"
	"github.com/openware-io/open-green-pass/internal/platform/db"
)

// OutboxStore persists CI delivery state and claims due items with row locks.
type OutboxStore struct{ db *db.DB }

func NewOutboxStore(database *db.DB) *OutboxStore { return &OutboxStore{db: database} }

func (s *OutboxStore) Enqueue(ctx context.Context, d domain.OutboxDelivery) error {
	if d.Status == "" {
		d.Status = domain.DeliveryPending
	}
	if err := d.Validate(); err != nil {
		return err
	}
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO cicd_outbox(team_id,event_id,delivery_key,provider,attempt,next_attempt_at,status,last_error,payload)
VALUES($1,$2,$3,$4,$5,COALESCE(NULLIF($6,'epoch'::timestamptz),now()),$7,$8,$9)
ON CONFLICT(team_id,delivery_key) DO NOTHING`, d.TeamID, d.EventID, d.DeliveryKey, d.Provider, d.Attempt, d.NextAttemptAt, d.Status, d.LastError, d.Payload)
		return err
	})
}

func (s *OutboxStore) Claim(ctx context.Context, now time.Time) (*domain.OutboxDelivery, error) {
	var delivery *domain.OutboxDelivery
	// Claim is intentionally a worker-only cross-tenant operation. It invokes the
	// migration-owned, narrowly scoped SECURITY DEFINER function; end-user paths
	// use the RLS-scoped Enqueue/Mark methods instead.
	row := s.db.Pool().QueryRow(ctx, `SELECT team_id,event_id,delivery_key,provider,attempt,next_attempt_at,status,last_error,payload FROM gp.claim_cicd_outbox($1)`, now.UTC())
	item := &domain.OutboxDelivery{}
	if err := row.Scan(&item.TeamID, &item.EventID, &item.DeliveryKey, &item.Provider, &item.Attempt, &item.NextAttemptAt, &item.Status, &item.LastError, &item.Payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("claim cicd outbox: %w", err)
	}
	delivery = item
	return delivery, nil
}

func (s *OutboxStore) MarkDelivered(ctx context.Context, key string) error {
	return s.update(ctx, key, domain.DeliveryDelivered, "", time.Time{})
}

func (s *OutboxStore) MarkFailed(ctx context.Context, key, reason string, next time.Time) error {
	status := domain.DeliveryPending
	if next.IsZero() {
		status = domain.DeliveryFailed
	}
	return s.update(ctx, key, status, reason, next)
}

// ReclaimStale makes a crashed worker's dispatching rows eligible for retry.
func (s *OutboxStore) ReclaimStale(ctx context.Context, now time.Time, timeout time.Duration) error {
	if timeout <= 0 {
		return domain.ErrInvalidDelivery
	}
	_, err := s.db.Pool().Exec(ctx, `UPDATE cicd_outbox SET status='pending', next_attempt_at=$1, updated_at=now()
WHERE status='dispatching' AND claimed_at IS NOT NULL AND claimed_at <= $2`, now.UTC(), now.Add(-timeout).UTC())
	return err
}

func (s *OutboxStore) update(ctx context.Context, key string, status domain.DeliveryStatus, reason string, next time.Time) error {
	if key == "" {
		return domain.ErrInvalidDelivery
	}
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE cicd_outbox SET status=$2,last_error=$3,
next_attempt_at=CASE WHEN $4='epoch'::timestamptz THEN next_attempt_at ELSE $4 END,updated_at=now()
WHERE delivery_key=$1 AND status='dispatching'`, key, status, reason, next)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return domain.ErrInvalidDelivery
		}
		return nil
	})
}

var _ domain.OutboxPort = (*OutboxStore)(nil)
