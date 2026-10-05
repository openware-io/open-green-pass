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
	result := AuditVerification{Valid: true}
	if len(events) == 0 {
		return result
	}

	teamID := events[0].TeamID
	if teamID == 0 {
		return invalidAuditVerification(result, events[0].ID, "team_id is required")
	}

	expectedPrev := zeroChainHead
	seenIDs := make(map[int64]struct{}, len(events))
	for _, event := range events {
		result.CheckedCount++
		if event.ID == 0 {
			return invalidAuditVerification(result, event.ID, "event id is required")
		}
		if _, duplicate := seenIDs[event.ID]; duplicate {
			return invalidAuditVerification(result, event.ID, "duplicate event id")
		}
		seenIDs[event.ID] = struct{}{}
		if event.TeamID != teamID {
			return invalidAuditVerification(result, event.ID, "mixed team_id values")
		}
		if event.Ts.IsZero() {
			return invalidAuditVerification(result, event.ID, "timestamp is required")
		}
		if !isSHA256Hex(event.PrevHash) {
			return invalidAuditVerification(result, event.ID, "prev_hash is not a lowercase sha256 digest")
		}
		if !isSHA256Hex(event.Hash) {
			return invalidAuditVerification(result, event.ID, "hash is not a lowercase sha256 digest")
		}
		if event.PrevHash != expectedPrev {
			return invalidAuditVerification(result, event.ID, "prev_hash does not match the preceding event")
		}
		expectedHash := ComputeAuditHash(event.PrevHash, event.Op, event.Asset, event.AssetID, event.Payload, event.Ts)
		if event.Hash != expectedHash {
			return invalidAuditVerification(result, event.ID, "hash does not match canonical event content")
		}
		expectedPrev = event.Hash
	}
	result.LastHash = expectedPrev
	return result
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
		return !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f'))
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
