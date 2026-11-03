package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// GenerationRunEventType is the closed set of append-only run events. The set
// must stay in sync with the gen_run_event CHECK constraint; SSE clients use the
// type to explain what changed without re-deriving it from payloads.
type GenerationRunEventType string

const (
	GenerationRunEventRunCreated       GenerationRunEventType = "run.created"
	GenerationRunEventRunStarted       GenerationRunEventType = "run.started"
	GenerationRunEventRunProgress      GenerationRunEventType = "run.progress"
	GenerationRunEventRunWaitingReview GenerationRunEventType = "run.waiting_review"
	GenerationRunEventRunCompleted     GenerationRunEventType = "run.completed"
	GenerationRunEventRunPartialFailed GenerationRunEventType = "run.partial_failed"
	GenerationRunEventRunFailed        GenerationRunEventType = "run.failed"
	GenerationRunEventRunCancelled     GenerationRunEventType = "run.cancelled"
	GenerationRunEventRunReconciled    GenerationRunEventType = "run.reconciled"

	GenerationRunEventUnitQueued         GenerationRunEventType = "unit.queued"
	GenerationRunEventUnitStageChanged   GenerationRunEventType = "unit.stage_changed"
	GenerationRunEventUnitReviewRequired GenerationRunEventType = "unit.review_required"
	GenerationRunEventUnitApproved       GenerationRunEventType = "unit.approved"
	GenerationRunEventUnitRejected       GenerationRunEventType = "unit.rejected"
	GenerationRunEventUnitFailed         GenerationRunEventType = "unit.failed"
	GenerationRunEventUnitCancelled      GenerationRunEventType = "unit.cancelled"
)

// MaxGenerationRunEventPayloadBytes bounds the append-only payload. Events are
// change signals, not snapshots; the run detail remains the authoritative read.
const MaxGenerationRunEventPayloadBytes = 4096

// ErrInvalidGenerationRunEvent is returned for unknown event types or oversized
// payloads so bad events fail before they reach the database.
var ErrInvalidGenerationRunEvent = errors.New("invalid generation run event")

var generationRunEventTypes = map[GenerationRunEventType]struct{}{
	GenerationRunEventRunCreated: {}, GenerationRunEventRunStarted: {}, GenerationRunEventRunProgress: {},
	GenerationRunEventRunWaitingReview: {}, GenerationRunEventRunCompleted: {}, GenerationRunEventRunPartialFailed: {},
	GenerationRunEventRunFailed: {}, GenerationRunEventRunCancelled: {}, GenerationRunEventRunReconciled: {},
	GenerationRunEventUnitQueued: {}, GenerationRunEventUnitStageChanged: {}, GenerationRunEventUnitReviewRequired: {},
	GenerationRunEventUnitApproved: {}, GenerationRunEventUnitRejected: {}, GenerationRunEventUnitFailed: {},
	GenerationRunEventUnitCancelled: {},
}

// GenerationRunEvent is one append-only change signal. Seq is assigned by the
// database identity column and is the resumable cursor for SSE (`after_seq`).
type GenerationRunEvent struct {
	ID        int64                  `json:"id"`
	TeamID    int64                  `json:"team_id"`
	RunID     int64                  `json:"run_id"`
	Seq       int64                  `json:"seq"`
	Type      GenerationRunEventType `json:"type"`
	UnitID    *int64                 `json:"unit_id,omitempty"`
	EventKey  string                 `json:"event_key,omitempty"`
	Payload   map[string]any         `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
}

// Validate enforces the closed event type set and bounded payload size before a
// row is appended. A malformed event is a programming error, not a user error.
func (e *GenerationRunEvent) Validate() error {
	if e == nil {
		return fmt.Errorf("%w: nil event", ErrInvalidGenerationRunEvent)
	}
	if e.RunID <= 0 || e.TeamID <= 0 {
		return fmt.Errorf("%w: tenant and run are required", ErrInvalidGenerationRunEvent)
	}
	if _, ok := generationRunEventTypes[e.Type]; !ok {
		return fmt.Errorf("%w: unknown type %q", ErrInvalidGenerationRunEvent, e.Type)
	}
	if len(e.Payload) == 0 {
		e.Payload = map[string]any{}
	}
	encoded, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("%w: encode payload: %v", ErrInvalidGenerationRunEvent, err)
	}
	if len(encoded) > MaxGenerationRunEventPayloadBytes {
		return fmt.Errorf("%w: payload exceeds %d bytes", ErrInvalidGenerationRunEvent, MaxGenerationRunEventPayloadBytes)
	}
	return nil
}

// GenerationRunEventRepository is the append-only event log. ListAfter is the
// replay/cursor read used by SSE and by diagnosis.
type GenerationRunEventRepository interface {
	Append(ctx context.Context, event *GenerationRunEvent) error
	ListAfter(ctx context.Context, teamID, runID, afterSeq int64, limit int) ([]*GenerationRunEvent, error)
	LastSeq(ctx context.Context, teamID, runID int64) (int64, error)
}

// ErrGenerationRunEventNotifierUnavailable means the cross-process wake-up hint
// channel is not ready (for example during a database reconnect). SSE callers
// must fall back to the low-frequency database poll instead of failing.
var ErrGenerationRunEventNotifierUnavailable = errors.New("generation run event notifier unavailable")

// GenerationRunEventNotifier delivers a coalesced "new events for this run"
// hint across processes. Hints are best effort: consumers always keep a
// low-frequency DB fallback because a NOTIFY can be lost during reconnect.
// Subscribe must never block the caller; the returned cancel function is
// idempotent and must be called when the stream ends.
type GenerationRunEventNotifier interface {
	Subscribe(ctx context.Context, runID int64) (<-chan struct{}, func(), error)
}

// RunStateEventType maps a persisted run state to the event emitted when the
// aggregate changes into that state.
func RunStateEventType(state GenerationRunState) GenerationRunEventType {
	switch state {
	case GenerationRunRunning:
		return GenerationRunEventRunProgress
	case GenerationRunWaitingReview:
		return GenerationRunEventRunWaitingReview
	case GenerationRunSucceeded:
		return GenerationRunEventRunCompleted
	case GenerationRunPartialFailed:
		return GenerationRunEventRunPartialFailed
	case GenerationRunFailed:
		return GenerationRunEventRunFailed
	case GenerationRunCancelled:
		return GenerationRunEventRunCancelled
	default:
		return GenerationRunEventRunStarted
	}
}

// UnitStateEventType maps a persisted unit state to the event emitted when the
// unit changes into that state.
func UnitStateEventType(state GenerationUnitState) GenerationRunEventType {
	switch state {
	case GenerationUnitRunning:
		return GenerationRunEventUnitStageChanged
	case GenerationUnitWaitingReview:
		return GenerationRunEventUnitReviewRequired
	case GenerationUnitSucceeded:
		return GenerationRunEventUnitApproved
	case GenerationUnitRejected:
		return GenerationRunEventUnitRejected
	case GenerationUnitFailed:
		return GenerationRunEventUnitFailed
	case GenerationUnitCancelled:
		return GenerationRunEventUnitCancelled
	default:
		return GenerationRunEventUnitQueued
	}
}
