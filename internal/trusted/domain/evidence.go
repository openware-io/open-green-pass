// 可信域测试证据元数据（evd_evidence）；二进制在对象存储。
package domain

import (
	"context"
	"time"
)

// Evidence 一条缺陷证据的元数据（不可变）。
type Evidence struct {
	ID           int64
	TeamID       int64
	FindingID    int64
	Kind         string
	StorageURI   string
	MimeType     string
	SizeBytes    int64
	Width        *int
	Height       *int
	DurationMS   *int
	SHA256       string
	ThumbnailURI *string
	Redacted     bool
	CreatedBy    int64
	CreatedAt    time.Time
}

// EvidenceRepository 证据元数据仓储端口。
type EvidenceRepository interface {
	Create(ctx context.Context, e *Evidence) error
	List(ctx context.Context, teamID, findingID int64) ([]*Evidence, error)
	Find(ctx context.Context, teamID, findingID, id int64) (*Evidence, error)
}
