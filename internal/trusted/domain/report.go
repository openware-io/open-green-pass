// 可信域测试报告核心概念：工程/服务/用例分级报告，evidence 哈希标注，预留跨场景合编（P2）。
package domain

import (
	"context"
	"errors"
	"time"
)

// ErrReportNotFound 报告不存在或租户不可见。
var ErrReportNotFound = errors.New("report not found")

// Report 测试报告（kind 分 project/service/case）。
type Report struct {
	ID        int64
	TeamID    int64
	RunID     int64
	TargetID  int64
	Kind      string
	Title     string
	Version   string
	Branch    string
	Scenario  string
	Status    string // pass/fail/blocked
	Summary   map[string]any
	Evidence  map[string]any
	HTML      string
	CreatedAt time.Time
}

// ReportRepository 报告仓储端口。
type ReportRepository interface {
	Save(ctx context.Context, r *Report) error
	Find(ctx context.Context, teamID, id int64) (*Report, error)
}

// ReportBundlePort supplies already-authorized reports for cross-scenario
// aggregation. Implementations must apply tenant isolation in the repository.
type ReportBundlePort interface {
	FindMany(ctx context.Context, teamID int64, ids []int64) ([]*Report, error)
}
