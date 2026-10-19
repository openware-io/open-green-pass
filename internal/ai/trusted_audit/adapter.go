package trusted_audit

import (
	"context"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	trusteddomain "github.com/openware-io/open-green-pass/internal/trusted/domain"
)

type auditAppender interface {
	Append(context.Context, int64, string, string, int64, map[string]any) (*trusteddomain.AuditEvent, error)
}

// Port writes generation decisions to the append-only trusted audit hash chain.
type Port struct{ service auditAppender }

func New(service *trustedapp.AuditService) *Port { return &Port{service: service} }

func (p *Port) AppendGenerationAudit(ctx context.Context, event application.GenerationAuditEvent) error {
	actor := event.ActorID
	if actor <= 0 {
		actor, _ = rls.UserFrom(ctx)
	}
	_, err := p.service.Append(ctx, actor, event.Action, "generation_batch", event.BatchID, map[string]any{
		"batch_id":      event.BatchID,
		"state_version": event.StateVersion,
		"request_id":    event.RequestID,
		"occurred_at":   event.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	})
	return err
}

var _ application.GenerationAuditPort = (*Port)(nil)
