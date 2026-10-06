package domain

import "time"

// AuditAnchor identifies a trusted, read-only checkpoint immediately before
// an exported audit range. It is an internal checkpoint only; it carries no
// external signature or claim of public notarization.
type AuditAnchor struct {
	TeamID    int64     `json:"team_id"`
	EventID   int64     `json:"event_id"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}
