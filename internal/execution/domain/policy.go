// 执行域服务级截图开关策略：PRD R-TEST-13（防截图证据在高并发下的性能开销）。
// 关闭时证据=日志+哈希；开启时才要求截图证据。
package domain

import "context"

// ScreenshotPolicy 服务级截图开关策略（按 被测对象 × 场景）。
type ScreenshotPolicy struct {
	TargetID          int64
	ScenarioID        int64
	ScreenshotEnabled bool
}

// PolicyPort 截图策略只读端口（执行域读，装配 CaseSpec）。
type PolicyPort interface {
	Get(ctx context.Context, teamID, targetID, scenarioID int64) (*ScreenshotPolicy, error)
}

// PolicyWriter 截图策略写端口（应用服务下发）。
type PolicyWriter interface {
	Set(ctx context.Context, p *ScreenshotPolicy) (*ScreenshotPolicy, error)
}

// PolicyRepository 读写组合端口（应用服务聚合读写）。
type PolicyRepository interface {
	PolicyPort
	PolicyWriter
}
