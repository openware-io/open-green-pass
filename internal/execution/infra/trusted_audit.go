package infra

import (
	"context"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	trusteddomain "github.com/openware-io/open-green-pass/internal/trusted/domain"
)

// TrustedAuditPort bridges execution events into the trusted hash-chain
// service. The trusted service remains responsible for tenant-scoped storage
// and hash calculation; this adapter only maps the bounded event contract.
type trustedAuditAppender interface {
	Append(context.Context, int64, string, string, int64, map[string]any) (*trusteddomain.AuditEvent, error)
}

type TrustedAuditPort struct{ service trustedAuditAppender }

func NewTrustedAuditPort(service *trustedapp.AuditService) *TrustedAuditPort {
	return &TrustedAuditPort{service: service}
}

func (p *TrustedAuditPort) Append(ctx context.Context, event domain.ExecutionAuditEvent) error {
	actorID, _ := rls.UserFrom(ctx)
	_, err := p.service.Append(ctx, actorID, event.Op, event.Asset, event.AssetID, event.Payload)
	return err
}

var _ domain.AuditPort = (*TrustedAuditPort)(nil)
