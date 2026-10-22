// TrustedAuditPort bridges governance findings into the Trusted hash-chain
// service. The trusted service owns tenant storage and hash calculation; this
// adapter only maps the bounded event contract.
package infra

import (
	"context"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	trusteddomain "github.com/openware-io/open-green-pass/internal/trusted/domain"
)

type trustedAuditAppender interface {
	Append(context.Context, int64, string, string, int64, map[string]any) (*trusteddomain.AuditEvent, error)
}

// TrustedAuditPort implements the governance application audit port.
type TrustedAuditPort struct{ service trustedAuditAppender }

// NewTrustedAuditPort creates the adapter.
func NewTrustedAuditPort(service *trustedapp.AuditService) *TrustedAuditPort {
	return &TrustedAuditPort{service: service}
}

// Append writes one finding audit event; actor comes from verified context.
func (p *TrustedAuditPort) Append(ctx context.Context, actor int64, op, asset string, assetID int64, payload map[string]any) error {
	if actor == 0 {
		actor, _ = rls.UserFrom(ctx)
	}
	_, err := p.service.Append(ctx, actor, op, asset, assetID, payload)
	return err
}
