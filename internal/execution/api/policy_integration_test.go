//go:build integration

// GP2-05 集成测试：服务级截图开关策略（PRD R-TEST-13）——下发→执行→证据形态变化（截图 vs 仅日志+哈希）。
package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestScreenshotPolicy 截图开关：关闭→执行证据=日志+hash；开启→执行证据=截图。
func TestScreenshotPolicy(t *testing.T) {
	h := newRunHandler(t)
	pool, _ := pgxpool.New(context.Background(), testDSN)
	t.Cleanup(pool.Close)
	ctx := context.Background()

	// 清理策略
	tx, _ := pool.Begin(ctx)
	_, _ = tx.Exec(ctx, `SELECT set_config('gp.team_id','100',false)`)
	_, _ = tx.Exec(ctx, `DELETE FROM svc_policy WHERE team_id=100`)
	_, _ = tx.Exec(ctx, `DELETE FROM run_run WHERE team_id=100`)
	_, _ = tx.Exec(ctx, `DELETE FROM run_case_result WHERE team_id=100`)
	_ = tx.Commit(ctx)

	// 1. 下发关闭截图
	code, body := doReq(t, h, "PUT", "/targets/1003/screenshot-policy", 100,
		`{"scenario_id":1,"screenshot_enabled":false}`)
	if code != http.StatusOK || !strings.Contains(body, `"ScreenshotEnabled":false`) {
		t.Fatalf("set screenshot=false want false got %d body=%s", code, body)
	}

	// 2. 读取确认
	code, body = doReq(t, h, "GET", "/targets/1003/screenshot-policy?scenario_id=1", 100, "")
	if code != http.StatusOK || !strings.Contains(body, `"ScreenshotEnabled":false`) {
		t.Fatalf("get screenshot want false got %d body=%s", code, body)
	}

	// 3. 执行运行 → 证据应仅日志（无截图）
	code, body = doReq(t, h, "POST", "/runs", 100,
		`{"scenario_id":1,"target_id":1003,"env":"test","target_version":"v1.0.0","target_branch":"main","run_mode":"manual"}`)
	if code != http.StatusCreated {
		t.Fatalf("create run want 201 got %d body=%s", code, body)
	}
	runID := extractID(t, body)
	code, _ = doReq(t, h, "POST", "/runs/"+itoa(runID)+"/version-check", 100, "")
	if code != http.StatusOK {
		t.Fatalf("version-check got %d", code)
	}
	code, body = doReq(t, h, "POST", "/runs/"+itoa(runID)+"/execute", 100, "")
	if code != http.StatusOK || !strings.Contains(body, "mock:log://") {
		t.Fatalf("execute off-shot want log evidence got %d body=%s", code, body)
	}

	// 4. 开启截图 → 证据含截图
	code, body = doReq(t, h, "PUT", "/targets/1003/screenshot-policy", 100,
		`{"scenario_id":1,"screenshot_enabled":true}`)
	if code != http.StatusOK {
		t.Fatalf("set screenshot=true got %d body=%s", code, body)
	}
	code, body = doReq(t, h, "POST", "/runs/"+itoa(runID)+"/execute", 100, "")
	if code != http.StatusOK || !strings.Contains(body, "mock:screenshot://") {
		t.Fatalf("execute on-shot want screenshot evidence got %d body=%s", code, body)
	}

	// 清理
	tx, _ = pool.Begin(ctx)
	_, _ = tx.Exec(ctx, `SELECT set_config('gp.team_id','100',false)`)
	_, _ = tx.Exec(ctx, `DELETE FROM svc_policy WHERE team_id=100`)
	_, _ = tx.Exec(ctx, `DELETE FROM run_run WHERE team_id=100`)
	_, _ = tx.Exec(ctx, `DELETE FROM run_case_result WHERE team_id=100`)
	_ = tx.Commit(ctx)
}
