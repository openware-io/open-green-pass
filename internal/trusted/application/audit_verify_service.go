package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/openware-io/open-green-pass/internal/trusted/domain"
)

var ErrAuditVerificationInput = errors.New("invalid audit verification input")

// AuditVerifier provides a bounded, read-only verification boundary for API,
// CLI and scheduled checks. It never requires trusted-writer credentials.
type AuditVerifier struct {
	MaxEvents int
}

func (v AuditVerifier) Verify(ctx context.Context, events []domain.AuditEvent, anchor *domain.AuditAnchor) (AuditVerification, error) {
	if ctx == nil || ctx.Err() != nil {
		return AuditVerification{}, ErrAuditVerificationInput
	}
	if len(events) == 0 || (v.MaxEvents > 0 && len(events) > v.MaxEvents) {
		return AuditVerification{}, fmt.Errorf("%w: event count", ErrAuditVerificationInput)
	}
	for _, event := range events {
		if event.ID <= 0 || event.TeamID <= 0 || event.Hash == "" || event.PrevHash == "" || event.Ts.IsZero() {
			return AuditVerification{}, fmt.Errorf("%w: malformed event", ErrAuditVerificationInput)
		}
	}
	return VerifyAuditChainFromAnchor(events, anchor), nil
}
