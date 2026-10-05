package application

import "context"

// AuditEvent is the bounded IAM-to-trusted event contract. It intentionally
// contains no credentials or provider claims.
type AuditEvent struct {
	Op      string
	Asset   string
	AssetID int64
	Payload map[string]any
}

type AuditPort interface {
	Append(context.Context, AuditEvent) error
}
type NoopAuditPort struct{}

func (NoopAuditPort) Append(context.Context, AuditEvent) error { return nil }
