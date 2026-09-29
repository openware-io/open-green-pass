//go:build integration

// GP1-07 集成测试：测试报告生成 + HTML 导出 + RLS。
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestReportGenerate 报告闭环：seed run → 生成报告 → 导出 HTML → RLS 隔离。
func TestReportGenerate(t *testing.T) {
	h := newTrustedHandler(t)
	pool, _ := pgxpool.New(context.Background(), testDSN)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	runID := time.Now().UnixNano()

	// 清理 + seed 一条 pass run（用已验证的 seedRun）
	tx, _ := pool.Begin(ctx)
	_, _ = tx.Exec(ctx, `SELECT set_config('gp.team_id','100',false)`)
	_, _ = tx.Exec(ctx, `DELETE FROM rpt_report WHERE team_id=100`)
	_, _ = tx.Exec(ctx, `DELETE FROM run_case_result WHERE run_id=$1`, runID)
	_, _ = tx.Exec(ctx, `DELETE FROM run_run WHERE id=$1`, runID)
	_ = tx.Commit(ctx)
	seedRun(t, pool, runID, "pass")

	// 1. 生成服务级报告
	code, body := gdoReq(t, h, "POST", "/reports/generate", 100,
		fmt.Sprintf(`{"run_id":%d,"kind":"service"}`, runID))
	if code != http.StatusOK || !strings.Contains(body, "软件工程测试治理平台") {
		t.Fatalf("generate want 200 got %d body=%s", code, body)
	}
	// 提取报告 id
	idx := strings.Index(body, `"ID":`)
	if idx < 0 {
		t.Fatalf("no report id in body=%s", body)
	}
	end := idx + len(`"ID":`)
	for end < len(body) && body[end] >= '0' && body[end] <= '9' {
		end++
	}
	reportID := body[idx+len(`"ID":`) : end]

	// 2. 导出 HTML
	code, body = gdoReq(t, h, "GET", "/reports/"+reportID+"/export?fmt=html", 100, "")
	if code != http.StatusOK || !strings.Contains(body, "<html") || !strings.Contains(body, "软件工程测试治理平台") {
		t.Fatalf("export want html got %d body(head)=%s", code, truncate(body, 120))
	}

	// 3. RLS：team200 导出 → 404
	code, _ = gdoReq(t, h, "GET", "/reports/"+reportID+"/export?fmt=html", 200, "")
	if code != http.StatusNotFound {
		t.Fatalf("team200 export want 404 got %d", code)
	}

	// 清理
	tx, _ = pool.Begin(ctx)
	_, _ = tx.Exec(ctx, `SELECT set_config('gp.team_id','100',false)`)
	_, _ = tx.Exec(ctx, `DELETE FROM rpt_report WHERE team_id=100`)
	_, _ = tx.Exec(ctx, `DELETE FROM run_case_result WHERE run_id=$1`, runID)
	_, _ = tx.Exec(ctx, `DELETE FROM run_run WHERE id=$1`, runID)
	_ = tx.Commit(ctx)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
