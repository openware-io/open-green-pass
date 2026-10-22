// 执行 Fixtures：每 run 独立数据命名空间 + 环境租约。
package domain

import (
	"context"
	"time"
)

// RunFixture 本次 run 的 fixture 命名空间（run_fixture）。
type RunFixture struct {
	ID           int64
	TeamID       int64
	RunID        int64
	Namespace    string
	Manifest     map[string]any
	State        string
	ExpiresAt    *time.Time
	CleanupState string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// EnvLease 环境独占租约（env_lease）。
type EnvLease struct {
	ID         int64
	TeamID     int64
	EnvID      int64
	RunID      int64
	Owner      string
	LeaseUntil time.Time
	CreatedAt  time.Time
}

// FixtureRepository fixture/lease 仓储端口。
type FixtureRepository interface {
	Save(ctx context.Context, f *RunFixture) error
	List(ctx context.Context, teamID, runID int64) ([]*RunFixture, error)
	Cleanup(ctx context.Context, teamID, runID int64, namespace string) error
	Lease(ctx context.Context, lease *EnvLease) error
	Release(ctx context.Context, teamID, envID int64) error
}
