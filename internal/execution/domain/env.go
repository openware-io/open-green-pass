// Package domain 执行域核心概念：测试环境版本校验（"测了没白测"）。
// 目标版本 vs 环境运行版本比对；mismatch 阻断执行。零三方依赖，见 ENGINEERING-SPEC §8。
package domain

import (
	"context"
	"time"
)

// EnvCheckResult 版本校验结果。
type EnvCheckResult string

const (
	EnvMatch    EnvCheckResult = "match"    // 目标版本 == 环境运行版本
	EnvMismatch EnvCheckResult = "mismatch" // 版本不一致，阻断执行
	EnvUnknown  EnvCheckResult = "unknown"  // 无环境运行版本记录，不阻断但留痕
)

// EnvRuntime 测试环境运行版本（校验依据）。
type EnvRuntime struct {
	ID             int64
	TeamID         int64 // RLS 租户列
	TargetID       int64 // 归属被测对象节点（服务/工程）
	Env            string
	RunningVersion string
	CheckedAt      time.Time
	CreatedBy      int64
}

// EnvCheck 版本校验记录（目标 vs 环境运行版本）。
type EnvCheck struct {
	ID            int64
	TeamID        int64
	RunID         *int64 // 关联运行（可空：独立校验）
	TargetID      int64
	TargetVersion string
	EnvVersion    string
	Result        EnvCheckResult
	CheckedAt     time.Time
}

// EnvRepository 环境版本仓储端口（实现位于 infra，RLS 由实现注入租户）。
type EnvRepository interface {
	UpsertRuntime(ctx context.Context, r *EnvRuntime) error
	FindRuntime(ctx context.Context, teamID, targetID int64, env string) (*EnvRuntime, error)
	SaveCheck(ctx context.Context, c *EnvCheck) error
	RecentChecks(ctx context.Context, teamID, targetID int64, limit int) ([]*EnvCheck, error)
}

// ErrNoEnvRuntime 被测对象在指定环境无运行版本记录。
type ErrNoEnvRuntime struct{ TargetID int64; Env string }

func (e *ErrNoEnvRuntime) Error() string {
	return "no env runtime recorded"
}
