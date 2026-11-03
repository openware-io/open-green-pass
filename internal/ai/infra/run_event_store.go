package infra

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/platform/db"
)

// GenerationRunEventStore appends change signals to gen_run_event. The table is
// append-only at the database layer (trigger + revoked UPDATE/DELETE), so this
// store intentionally exposes no update or delete path.
type GenerationRunEventStore struct {
	db *db.DB
}

func NewGenerationRunEventStore(database *db.DB) *GenerationRunEventStore {
	return &GenerationRunEventStore{db: database}
}

var _ domain.GenerationRunEventRepository = (*GenerationRunEventStore)(nil)

// Append inserts one event. A repeated event_key for the same run is a no-op,
// which keeps retried review/cancel commands from duplicating history.
func (s *GenerationRunEventStore) Append(ctx context.Context, event *domain.GenerationRunEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	eventKey := nullString(event.EventKey)
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var seq int64
		err := tx.QueryRow(ctx, `
			INSERT INTO gen_run_event(id,team_id,run_id,type,unit_id,event_key,payload)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (run_id,event_key) WHERE event_key IS NOT NULL DO NOTHING
			RETURNING seq`,
			event.ID, event.TeamID, event.RunID, string(event.Type), event.UnitID, eventKey, payload).Scan(&seq)
		if errors.Is(err, pgx.ErrNoRows) {
			// Another writer already appended this event; that is success.
			return nil
		}
		if err != nil {
			return err
		}
		event.Seq = seq
		return nil
	})
}

func (s *GenerationRunEventStore) ListAfter(ctx context.Context, teamID, runID, afterSeq int64, limit int) ([]*domain.GenerationRunEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var out []*domain.GenerationRunEvent
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id,team_id,run_id,seq,type,unit_id,COALESCE(event_key,''),payload,created_at
			FROM gen_run_event
			WHERE team_id=$1 AND run_id=$2 AND seq > $3
			ORDER BY seq LIMIT $4`, teamID, runID, afterSeq, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var event domain.GenerationRunEvent
			var eventType string
			var payload []byte
			if err := rows.Scan(&event.ID, &event.TeamID, &event.RunID, &event.Seq, &eventType,
				&event.UnitID, &event.EventKey, &payload, &event.CreatedAt); err != nil {
				return err
			}
			event.Type = domain.GenerationRunEventType(eventType)
			if err := json.Unmarshal(payload, &event.Payload); err != nil {
				return err
			}
			out = append(out, &event)
		}
		return rows.Err()
	})
	return out, err
}

func (s *GenerationRunEventStore) LastSeq(ctx context.Context, teamID, runID int64) (int64, error) {
	var last int64
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COALESCE(MAX(seq),0) FROM gen_run_event WHERE team_id=$1 AND run_id=$2`, teamID, runID).Scan(&last)
	})
	return last, err
}
