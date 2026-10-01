package infra

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	trusteddomain "github.com/openware-io/open-green-pass/internal/trusted/domain"
)

type capturedAudit struct {
	actor   int64
	op      string
	asset   string
	assetID int64
	payload map[string]any
}

func (c *capturedAudit) Append(_ context.Context, actor int64, op, asset string, assetID int64, payload map[string]any) (*trusteddomain.AuditEvent, error) {
	c.actor, c.op, c.asset, c.assetID, c.payload = actor, op, asset, assetID, payload
	return &trusteddomain.AuditEvent{}, nil
}

func TestTrustedAuditPortPreservesActorAndEvent(t *testing.T) {
	capture := &capturedAudit{}
	port := &TrustedAuditPort{service: capture}
	ctx := rls.WithUser(context.Background(), 99)
	err := port.Append(ctx, domain.ExecutionAuditEvent{
		Op: "execution.resource_conflict", Asset: "run", AssetID: 88,
		Payload: map[string]any{"target_id": int64(7)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if capture.actor != 99 || capture.op != "execution.resource_conflict" || capture.asset != "run" || capture.assetID != 88 {
		t.Fatalf("capture=%+v", capture)
	}
	if capture.payload["target_id"] != int64(7) {
		t.Fatalf("payload=%v", capture.payload)
	}
}
