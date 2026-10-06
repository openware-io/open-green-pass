package application

import (
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/trusted/domain"
)

func TestVerifyAuditChainAcceptsUntamperedChain(t *testing.T) {
	events := verificationEvents(t)

	result := VerifyAuditChain(events)
	if !result.Valid {
		t.Fatalf("verification failed: %+v", result)
	}
	if result.CheckedCount != 2 || result.LastHash != events[1].Hash {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestVerifyAuditChainDetectsPayloadTampering(t *testing.T) {
	events := verificationEvents(t)
	events[1].Payload["decision"] = "pass"

	result := VerifyAuditChain(events)
	if result.Valid || result.FirstInvalidEventID != events[1].ID {
		t.Fatalf("expected payload tampering to be detected: %+v", result)
	}
}

func TestVerifyAuditChainDetectsBrokenLink(t *testing.T) {
	events := verificationEvents(t)
	events[1].PrevHash = zeroChainHead

	result := VerifyAuditChain(events)
	if result.Valid || result.FirstInvalidEventID != events[1].ID {
		t.Fatalf("expected broken link to be detected: %+v", result)
	}
}

func TestVerifyAuditChainRejectsMixedTeamExport(t *testing.T) {
	events := verificationEvents(t)
	events[1].TeamID = 99

	result := VerifyAuditChain(events)
	if result.Valid || result.FirstInvalidEventID != events[1].ID {
		t.Fatalf("expected mixed team export to be rejected: %+v", result)
	}
}

func TestVerifyAuditChainFromAnchorAcceptsRange(t *testing.T) {
	events := verificationEvents(t)
	anchor := &domain.AuditAnchor{TeamID: 7, EventID: 1, Hash: events[0].Hash, CreatedAt: time.Now().UTC()}
	rangeEvents := []domain.AuditEvent{events[1]}
	result := VerifyAuditChainFromAnchor(rangeEvents, anchor)
	if !result.Valid || result.LastHash != events[1].Hash {
		t.Fatalf("anchored verification failed: %+v", result)
	}
}

func TestVerifyAuditChainFromAnchorRejectsTamperedAnchor(t *testing.T) {
	events := verificationEvents(t)
	anchor := &domain.AuditAnchor{TeamID: 7, EventID: 1, Hash: zeroChainHead, CreatedAt: time.Now().UTC()}
	result := VerifyAuditChainFromAnchor([]domain.AuditEvent{events[1]}, anchor)
	if result.Valid || result.FirstInvalidEventID != events[1].ID {
		t.Fatalf("expected tampered anchor to break range: %+v", result)
	}
}

func verificationEvents(t *testing.T) []domain.AuditEvent {
	t.Helper()
	ts := time.Date(2026, time.October, 6, 1, 2, 3, 0, time.UTC)
	first := domain.AuditEvent{
		ID: 1, TeamID: 7, Actor: 3, Op: "run.created", Asset: "run", AssetID: 10,
		Payload: map[string]any{"mode": "smoke"}, PrevHash: zeroChainHead, Ts: ts,
	}
	first.Hash = ComputeAuditHash(first.PrevHash, first.Op, first.Asset, first.AssetID, first.Payload, first.Ts)
	second := domain.AuditEvent{
		ID: 2, TeamID: 7, Actor: 3, Op: "gate.evaluate", Asset: "run", AssetID: 10,
		Payload: map[string]any{"decision": "fail"}, PrevHash: first.Hash, Ts: ts.Add(time.Second),
	}
	second.Hash = ComputeAuditHash(second.PrevHash, second.Op, second.Asset, second.AssetID, second.Payload, second.Ts)
	return []domain.AuditEvent{first, second}
}
