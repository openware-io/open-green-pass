package infra

import (
	"context"

	"github.com/openware-io/open-green-pass/internal/iam/application"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	trusteddomain "github.com/openware-io/open-green-pass/internal/trusted/domain"
)

type auditAppender interface {
	Append(context.Context, int64, string, string, int64, map[string]any) (*trusteddomain.AuditEvent, error)
}
type TrustedAuditPort struct{ service auditAppender }

func NewTrustedAuditPort(service *trustedapp.AuditService) *TrustedAuditPort {
	return &TrustedAuditPort{service: service}
}
func (p *TrustedAuditPort) Append(ctx context.Context, event application.AuditEvent) error {
	actor, _ := rls.UserFrom(ctx)
	_, err := p.service.Append(ctx, actor, event.Op, event.Asset, event.AssetID, event.Payload)
	return err
}

var _ application.AuditPort = (*TrustedAuditPort)(nil)
