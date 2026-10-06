// Package application 可信域应用服务：审计哈希链（append-only，防篡改）。
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// ErrTenantRequired 请求缺少租户（x-gp-team-id）。
var ErrTenantRequired = &TenantErr{}

// TenantErr 租户缺失错误。
type TenantErr struct{}

func (e *TenantErr) Error() string { return "tenant required: missing x-gp-team-id header" }

func errTenantRequired() error { return ErrTenantRequired }

// AuditService 审计服务（审计哈希链写入）。
type AuditService struct {
	repo domain.AuditRepository
	gen  *id.Generator
}

// NewAuditService 创建审计服务。
func NewAuditService(repo domain.AuditRepository, gen *id.Generator) *AuditService {
	return &AuditService{repo: repo, gen: gen}
}

// zeroChainHead 哈希链头（首事件 prev = 全零）。
const zeroChainHead = "0000000000000000000000000000000000000000000000000000000000000000"

// Append 追加审计事件（计算哈希链）。
func (s *AuditService) Append(ctx context.Context, actor int64, op, asset string, assetID int64, payload map[string]any) (*domain.AuditEvent, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	if atomicRepo, ok := s.repo.(domain.AtomicAuditRepository); ok {
		return atomicRepo.AppendWithHead(ctx, teamID, func(prev string) (*domain.AuditEvent, error) {
			return s.buildEvent(teamID, actor, op, asset, assetID, payload, prev), nil
		})
	}
	return s.appendLegacy(ctx, teamID, actor, op, asset, assetID, payload)
}

func (s *AuditService) appendLegacy(ctx context.Context, teamID, actor int64, op, asset string, assetID int64, payload map[string]any) (*domain.AuditEvent, error) {
	prev := zeroChainHead
	if h, err := s.repo.LatestHash(ctx, teamID); err == nil {
		prev = h
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	e := s.buildEvent(teamID, actor, op, asset, assetID, payload, prev)
	if err := s.repo.Append(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *AuditService) buildEvent(teamID, actor int64, op, asset string, assetID int64, payload map[string]any, prev string) *domain.AuditEvent {
	ts := time.Now().UTC()
	hash := ComputeAuditHash(prev, op, asset, assetID, payload, ts)
	e := &domain.AuditEvent{
		ID: s.gen.Next(), TeamID: teamID, Actor: actor, Op: op, Asset: asset, AssetID: assetID,
		Payload: payload, PrevHash: prev, Hash: hash, Ts: ts,
	}
	return e
}

// ComputeAuditHash 计算审计哈希链块 hash = sha256(prev || op || asset || asset_id || payload || ts)。
func ComputeAuditHash(prev, op, asset string, assetID int64, payload map[string]any, ts time.Time) string {
	pj, _ := json.Marshal(payload)
	h := sha256.New()
	h.Write([]byte(prev))
	h.Write([]byte(op))
	h.Write([]byte(asset))
	h.Write([]byte(int64String(assetID)))
	h.Write(pj)
	h.Write([]byte(ts.Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))
}

func int64String(n int64) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
