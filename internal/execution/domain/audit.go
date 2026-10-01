package domain

import "context"

// ExecutionAuditEvent records an execution-domain event through the trusted
// audit boundary. It deliberately carries only primitives so the trusted
// domain remains the authority for hash-chain construction and persistence.
type ExecutionAuditEvent struct {
	Op      string
	Asset   string
	AssetID int64
	Payload map[string]any
}

// AuditPort is the outbound execution-to-trusted audit boundary.
type AuditPort interface {
	Append(context.Context, ExecutionAuditEvent) error
}

// NoopAuditPort supports local/P1 setups that have not wired trusted storage.
// Production composition must inject a trusted implementation.
type NoopAuditPort struct{}

func (NoopAuditPort) Append(context.Context, ExecutionAuditEvent) error { return nil }

var _ AuditPort = NoopAuditPort{}
