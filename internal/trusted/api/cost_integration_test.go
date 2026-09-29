//go:build integration

// GP1-06 集成测试：成本明细（计量幂等 / 总览=Σ明细 / 明细按大类 / 历史对比 + RLS）。
package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestCostAccounting 成本闭环：生成/执行各计量 → 幂等防重复 → 总览=Σ → 明细按大类 → 历史对比 + RLS。
func TestCostAccounting(t *testing.T) {
	h := newTrustedHandler(t)
	pool, _ := pgxpool.New(context.Background(), testDSN)
	t.Cleanup(pool.Close)
	ctx := context.Background()

	// 清理测试成本数据
	tx, _ := pool.Begin(ctx)
	_, _ = tx.Exec(ctx, `SELECT set_config('gp.team_id','100',false)`)
	_, _ = tx.Exec(ctx, `DELETE FROM cost_line_item WHERE team_id=100`)
	_ = tx.Commit(ctx)

	// 1. 生成成本（biz_point=case_generate，case 4001）
	code, body := gdoReq(t, h, "POST", "/cost/record", 100,
		`{"request_id":"req-gen-1","biz_point":"case_generate","target_id":1003,"case_id":4001,"model":"mock-llm","tokens_in":1000,"tokens_out":500,"unit_price":0.02}`)
	if code != http.StatusOK || !strings.Contains(body, `"inserted":true`) {
		t.Fatalf("record generate want inserted got %d body=%s", code, body)
	}

	// 2. 执行成本（biz_point=case_execute，case 4001，run 10001）
	code, body = gdoReq(t, h, "POST", "/cost/record", 100,
		`{"request_id":"req-exe-1","biz_point":"case_execute","target_id":1003,"run_id":10001,"case_id":4001,"model":"mock-llm","tokens_in":800,"tokens_out":200,"unit_price":0.02}`)
	if code != http.StatusOK || !strings.Contains(body, `"inserted":true`) {
		t.Fatalf("record execute want inserted got %d body=%s", code, body)
	}

	// 3. 同 request_id+biz_point 重放 → 幂等不重复（inserted=false）
	code, body = gdoReq(t, h, "POST", "/cost/record", 100,
		`{"request_id":"req-exe-1","biz_point":"case_execute","target_id":1003,"run_id":10001,"case_id":4001,"model":"mock-llm","tokens_in":800,"tokens_out":200,"unit_price":0.02}`)
	if code != http.StatusOK || !strings.Contains(body, `"inserted":false`) {
		t.Fatalf("idempotent replay want inserted=false got %d body=%s", code, body)
	}

	// 4. 总览：TotalAmount = (1500/1000*0.02=0.03 generate) + (1000/1000*0.02=0.02 execute) = 0.05
	code, body = gdoReq(t, h, "GET", "/cost/overview", 100, "")
	if code != http.StatusOK || !strings.Contains(body, `"generate":0.03`) || !strings.Contains(body, `"execute":0.02`) {
		t.Fatalf("overview want generate/execute split got %d body=%s", code, body)
	}

	// 5. 明细按大类（execute）
	code, body = gdoReq(t, h, "GET", "/cost/items?category=execute", 100, "")
	if code != http.StatusOK || !strings.Contains(body, "req-exe-1") || strings.Contains(body, "req-gen-1") {
		t.Fatalf("items execute want only execute got %d body=%s", code, body)
	}

	// 6. 历史对比（case 4001 两条）
	code, body = gdoReq(t, h, "GET", "/cost/compare/4001", 100, "")
	if code != http.StatusOK || !strings.Contains(body, "case_generate") || !strings.Contains(body, "case_execute") {
		t.Fatalf("compare history want 2 kinds got %d body=%s", code, body)
	}

	// 7. RLS：team 200 总览应为空（0）
	code, body = gdoReq(t, h, "GET", "/cost/overview", 200, "")
	if code != http.StatusOK || strings.Contains(body, "req-exe-1") {
		t.Fatalf("team200 overview should be empty got %d body=%s", code, body)
	}

	// 清理
	tx, _ = pool.Begin(ctx)
	_, _ = tx.Exec(ctx, `SELECT set_config('gp.team_id','100',false)`)
	_, _ = tx.Exec(ctx, `DELETE FROM cost_line_item WHERE team_id=100`)
	_ = tx.Commit(ctx)
}
