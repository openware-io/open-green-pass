// CaseSpec 用例执行规格（脚本/目标/环境），由 RunService 从用例库装配后交 RunnerPort 执行。
package domain

import "context"

// CaseSpec 单条用例执行规格。
type CaseSpec struct {
	CaseID      int64
	CaseVersion int
	TargetID    int64
	Env         string
	// Scenario selects the execution adapter (api, web_e2e, mobile_e2e,
	// performance). Empty values are treated as api for backwards compatibility.
	Scenario string
	Script   any // 执行脚本定义（API 场景为 HTTP 定义；其他场景各异）
	// ScreenshotEnabled 服务级截图开关（PRD R-TEST-13）：false=证据仅日志+哈希，避免高并发截图开销。
	ScreenshotEnabled bool
}

// RunnerPort 执行器端口（P1 提供 MockRunner 本地闭环；K8s Job Runner 接入真实沙箱执行）。
type RunnerPort interface {
	Execute(ctx context.Context, spec []*CaseSpec) ([]*CaseResult, error)
}

// CasePort 用例库端口（execution 只读治理域用例：按被测对象列出/取脚本）。
type CasePort interface {
	ListIDsByTarget(ctx context.Context, teamID, targetID int64) ([]int64, error)
	FetchScript(ctx context.Context, teamID, caseID int64) (version int, script any, err error)
}
