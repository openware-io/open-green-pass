// 可信域审计哈希链核心概念：aud_event append-only + 防篡改哈希链。
package domain

import (
	"context"
	"time"
)

// AuditEvent 审计事件（哈希链块；hash = sha256(prev_hash || op || asset || asset_id || payload || ts)）。
type AuditEvent struct {
	ID       int64          `json:"id"`
	TeamID   int64          `json:"team_id"`
	Actor    int64          `json:"actor"`
	Op       string         `json:"op"` // gate.evaluate / cost.insert / case.rollback / ...
	Asset    string         `json:"asset"`
	AssetID  int64          `json:"asset_id"`
	Payload  map[string]any `json:"payload"`
	PrevHash string         `json:"prev_hash"`
	Hash     string         `json:"hash"`
	Ts       time.Time      `json:"ts"`
}

// AuditRepository 审计仓储端口（append-only）。
type AuditRepository interface {
	// Append 追加审计事件：读当前租户最新 hash 为 prev，计算本事件 hash 后插入。
	Append(ctx context.Context, e *AuditEvent) error
	// LatestHash 读当前租户最新审计事件 hash（首事件为全零链头）。
	LatestHash(ctx context.Context, teamID int64) (string, error)
}

// AtomicAuditRepository serializes head selection and append in one database
// transaction. Implementations use a tenant-scoped advisory lock; callers
// must prefer this port whenever concurrent writers are possible.
type AtomicAuditRepository interface {
	AuditRepository
	AppendWithHead(ctx context.Context, teamID int64, build func(prevHash string) (*AuditEvent, error)) (*AuditEvent, error)
}
