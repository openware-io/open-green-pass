package application

import (
	"fmt"
	"strings"

	"github.com/openware-io/open-green-pass/internal/trusted/domain"
)

// AuditVerification is the result of validating a complete, ordered audit
// chain. It is deliberately a query result: verification must never append an
// event to the chain it is checking.
type AuditVerification struct {
	Valid               bool   `json:"valid"`
	CheckedCount        int    `json:"checked_count"`
	FirstInvalidEventID int64  `json:"first_invalid_event_id,omitempty"`
	Reason              string `json:"reason,omitempty"`
	LastHash            string `json:"last_hash,omitempty"`
}

// VerifyAuditChain recomputes the current aud_event hash contract for a
// complete chain ordered from its zero hash head to its tail. Callers that
// verify an exported range must include the preceding event (or an anchor
// verifier) so the first event's prev_hash can be verified.
//
// This verifier intentionally does not claim to verify an external anchor.
// Anchor manifests/signatures are a separate GP3-04 integration because they
// require a trusted external reference and read-only credential boundary.
func VerifyAuditChain(events []domain.AuditEvent) AuditVerification {
	return VerifyAuditChainFromAnchor(events, nil)
}

// VerifyAuditChainFromAnchor verifies an exported range against an internal
// read-only checkpoint. The anchor is not written or refreshed by verification.
// A nil anchor requires the range to begin at the zero chain head.
func VerifyAuditChainFromAnchor(events []domain.AuditEvent, anchor *domain.AuditAnchor) AuditVerification {
	result := AuditVerification{Valid: true}
	if len(events) == 0 {
		return result
	}
	teamID := events[0].TeamID
	if teamID == 0 {
		return invalidAuditVerification(result, events[0].ID, "team_id is required")
	}
	expectedPrev, eventID, reason := auditChainHead(anchor, teamID, events[0].ID)
	if reason != "" {
		return invalidAuditVerification(result, eventID, reason)
	}
	seenIDs := make(map[int64]struct{}, len(events))
	for _, event := range events {
		result.CheckedCount++
		next, reason := verifyAuditEvent(event, expectedPrev, teamID, anchor, seenIDs)
		if reason != "" {
			return invalidAuditVerification(result, event.ID, reason)
		}
		expectedPrev = next
	}
	result.LastHash = expectedPrev
	return result
}

// auditChainHead resolves the expected prev_hash before the first event. It
// returns the event ID to report when validation fails (0 for anchor defects).
func auditChainHead(anchor *domain.AuditAnchor, teamID, firstEventID int64) (prev string, eventID int64, reason string) {
	if anchor == nil {
		return zeroChainHead, 0, ""
	}
	if anchor.TeamID == 0 || anchor.EventID == 0 || !isSHA256Hex(anchor.Hash) || anchor.CreatedAt.IsZero() {
		return "", 0, "invalid audit anchor"
	}
	if anchor.TeamID != teamID {
		return "", firstEventID, "anchor team_id does not match export"
	}
	return anchor.Hash, 0, ""
}

// verifyAuditEvent validates one event against its expected predecessor and the
// accumulated chain state, returning the next expected prev_hash or a reason.
func verifyAuditEvent(event domain.AuditEvent, expectedPrev string, teamID int64, anchor *domain.AuditAnchor, seenIDs map[int64]struct{}) (string, string) {
	if event.ID == 0 {
		return "", "event id is required"
	}
	if anchor != nil && event.ID <= anchor.EventID {
		return "", "event precedes or duplicates audit anchor"
	}
	if _, duplicate := seenIDs[event.ID]; duplicate {
		return "", "duplicate event id"
	}
	seenIDs[event.ID] = struct{}{}
	if event.TeamID != teamID {
		return "", "mixed team_id values"
	}
	if event.Ts.IsZero() {
		return "", "timestamp is required"
	}
	if !isSHA256Hex(event.PrevHash) {
		return "", "prev_hash is not a lowercase sha256 digest"
	}
	if !isSHA256Hex(event.Hash) {
		return "", "hash is not a lowercase sha256 digest"
	}
	if event.PrevHash != expectedPrev {
		return "", "prev_hash does not match the preceding event"
	}
	expectedHash := ComputeAuditHash(event.PrevHash, event.Op, event.Asset, event.AssetID, event.Payload, event.Ts)
	if event.Hash != expectedHash {
		return "", "hash does not match canonical event content"
	}
	return event.Hash, ""
}

func invalidAuditVerification(result AuditVerification, eventID int64, reason string) AuditVerification {
	result.Valid = false
	result.FirstInvalidEventID = eventID
	result.Reason = reason
	return result
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	return strings.IndexFunc(value, func(r rune) bool {
		return (r < '0' || r > '9') && (r < 'a' || r > 'f')
	}) == -1
}

// FormatAuditVerification returns a stable, operator-facing failure summary.
// It keeps the verifier usable by a CLI without exposing event payloads.
func FormatAuditVerification(result AuditVerification) string {
	if result.Valid {
		return fmt.Sprintf("audit chain valid: checked=%d last_hash=%s", result.CheckedCount, result.LastHash)
	}
	return fmt.Sprintf("audit chain invalid: checked=%d event_id=%d reason=%s", result.CheckedCount, result.FirstInvalidEventID, result.Reason)
}
