//go:build integration

// GP1-05 集成测试：门禁判定（策略装载 → 全 pass→pass / 引入 fail→fail / 判定结果 + RLS）与审计哈希链。
package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/gateway"
	"github.com/openware-io/open-green-pass/internal/platform/db"
	tapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	tinfra "github.com/openware-io/open-green-pass/internal/trusted/infra"
	"github.com/openware-io/open-green-pass/pkg/id"
)

const testDSN = "postgres://gp:gp@127.0.0.1:5433/gp?sslmode=disable"

func newTrustedHandler(t *testing.T) http.Handler {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDSN)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	gen, _ := id.New(1, nil)
	dbb := db.NewDB(pool)
	gateStore := tinfra.NewGateStore(dbb, gen)
	auditStore := tinfra.NewAuditStore(dbb, gen)
	auditSvc := tapp.NewAuditService(auditStore, gen)
	statReader := tinfra.NewRunStatReader(dbb)
	gateSvc := tapp.NewGateService(gateStore, auditSvc, statReader, gen)
	costStore := tinfra.NewCostStore(dbb, gen)
	costSvc := tapp.NewCostService(costStore, auditSvc, gen)
	reportStore := tinfra.NewReportStore(dbb, gen)
	reportSvc, _ := tapp.NewReportService(statReader, costStore, gateStore, reportStore, gen)
	return gateway.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(mux *http.ServeMux) { Register(mux, gateSvc, auditSvc, costSvc, reportSvc) })
}

// seedRun 直接造一条 run + 一条用例结果（幂等：随机 run_id；同一事务 set tenant 供触发器/RLS）。
func seedRun(t *testing.T, pool *pgxpool.Pool, runID int64, status string) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('gp.team_id','100',false)`); err != nil {
		t.Fatalf("set tenant: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO run_run(id, team_id, scenario_id, target_id, env, target_version, target_branch, run_mode, state, selected_cases, created_by)
		 VALUES($1,100,1,1003,'test','v1.0.0','main','manual','done','[]',1)`, runID); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO run_case_result(id, team_id, run_id, case_id, case_version, status, attempt_seq, started_at)
		 VALUES($1,100,$2,4001,1,$3,1,now())`, runID*10, runID, status); err != nil {
		t.Fatalf("seed result: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func gdoReq(t *testing.T, h http.Handler, method, path string, team int64, body string) (int, string) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-gp-team-id", itoa2(team))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func itoa2(n int64) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestGateEvaluate 门禁闭环：装载策略→全 pass 判定 pass→引入 fail 判定 fail→判定入审计链 + RLS。
func TestGateEvaluate(t *testing.T) {
	h := newTrustedHandler(t)
	pool, _ := pgxpool.New(context.Background(), testDSN)
	t.Cleanup(pool.Close)

	runID := time.Now().UnixNano() // 随机 run，幂等

	// 0. 清理既有门禁规则（幂等；RLS 需事务 set tenant）
	ctx := context.Background()
	tx, _ := pool.Begin(ctx)
	_, _ = tx.Exec(ctx, `SELECT set_config('gp.team_id','100',false)`)
	_, _ = tx.Exec(ctx, `DELETE FROM gate_rule WHERE team_id=100 AND target_id=1003 AND scenario_id=1`)
	_, _ = tx.Exec(ctx, `DELETE FROM gate_result WHERE team_id=100`)
	_ = tx.Commit(ctx)

	// 1. 装载门禁策略（target 1003 / scenario 1：max_failed=0, pass_rate=1.0）
	policy := `{"min_pass_rate":1.0,"max_failed":0,"min_coverage":0.8}`
	code, body := gdoReq(t, h, "PUT", "/gates/rules", 100,
		fmt.Sprintf(`{"target_id":1003,"scenario_id":1,"rego":%q}`, policy))
	if code != http.StatusOK || !strings.Contains(body, `"version":1`) {
		t.Fatalf("upsert rule want 200/v1 got %d body=%s", code, body)
	}

	// 1. 造一条全 pass 的 run，判定 → pass
	seedRun(t, pool, runID, "pass")
	code, body = gdoReq(t, h, "POST", "/gates/evaluate", 100,
		fmt.Sprintf(`{"run_id":%d}`, runID))
	if code != http.StatusOK || !strings.Contains(body, `"result":"pass"`) {
		t.Fatalf("evaluate pass-run want pass got %d body=%s", code, body)
	}

	// 2. 把该用例改为 fail，重新判定 → fail
	if _, err := pool.Exec(context.Background(),
		`UPDATE run_case_result SET status='fail' WHERE run_id=$1`, runID); err != nil {
		t.Fatalf("flip to fail: %v", err)
	}
	code, body = gdoReq(t, h, "POST", "/gates/evaluate", 100,
		fmt.Sprintf(`{"run_id":%d}`, runID))
	if code != http.StatusOK || !strings.Contains(body, `"result":"fail"`) {
		t.Fatalf("evaluate fail-run want fail got %d body=%s", code, body)
	}

	// 3. 判定结果入审计链（aud_event 应有 gate.evaluate）
	var cnt int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM aud_event WHERE team_id=100 AND op='gate.evaluate' AND asset_id=$1`, runID).Scan(&cnt); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("want 2 audit events got %d", cnt)
	}

	// 4. RLS：team 200 判定该 run → 404（run 不可见）
	code, body = gdoReq(t, h, "POST", "/gates/evaluate", 200,
		fmt.Sprintf(`{"run_id":%d}`, runID))
	if code != http.StatusNotFound {
		t.Fatalf("team200 evaluate want 404 got %d body=%s", code, body)
	}

	// 清理
	_, _ = pool.Exec(context.Background(), `DELETE FROM run_case_result WHERE run_id=$1`, runID)
	_, _ = pool.Exec(context.Background(), `DELETE FROM run_run WHERE id=$1`, runID)
}
