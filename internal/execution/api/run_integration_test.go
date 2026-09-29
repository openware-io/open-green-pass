//go:build integration

// GP1-04 集成测试：运行编排闭环（创建→版本校验→执行→重跑 attempt 递增 + RLS 隔离）。
package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/execution/application"
	einfra "github.com/openware-io/open-green-pass/internal/execution/infra"
	"github.com/openware-io/open-green-pass/internal/gateway"
	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/pkg/id"
)

func newRunHandler(t *testing.T) http.Handler {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDSN)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	gen, _ := id.New(1, nil)
	dbb := db.NewDB(pool)
	envStore := einfra.NewEnvStore(dbb, gen)
	envSvc := application.NewEnvService(envStore, gen)
	runStore := einfra.NewRunStore(dbb, gen)
	caseReader := einfra.NewCaseReader(dbb)
	policyStore := einfra.NewPolicyStore(dbb, gen)
	policySvc := application.NewPolicyService(policyStore, gen)
	mockRunner := einfra.NewMockRunner(gen)
	runSvc := application.NewRunService(runStore, envSvc, caseReader, policySvc, mockRunner, gen)
	return gateway.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(mux *http.ServeMux) { Register(mux, envSvc, runSvc, policySvc) })
}

// TestRunExecution 运行闭环：创建→版本校验(match)→执行→重跑 attempt 递增→历史 + RLS。
func TestRunExecution(t *testing.T) {
	h := newRunHandler(t)
	const targetID = 1003 // im-saas-gateway（已有用例 im-saas-gw-001）

	// 1. 创建运行（勾选空→全量；该被测对象现有 1 条活跃用例）
	code, body := doReq(t, h, "POST", "/runs", 100,
		`{"scenario_id":1,"target_id":1003,"env":"test","target_version":"v1.0.0","target_branch":"main","run_mode":"manual"}`)
	if code != http.StatusCreated || !strings.Contains(body, `"State":"queued"`) {
		t.Fatalf("create run want 201/queued got %d body=%s", code, body)
	}
	runID := extractID(t, body)
	if !strings.Contains(body, "4001") {
		t.Fatalf("selected cases should include 4001, body=%s", body)
	}

	// 2. 版本校验（env test 运行 v1.0.0 == 目标 v1.0.0 → match → scheduled）
	code, body = doReq(t, h, "POST", "/runs/"+itoa(runID)+"/version-check", 100, "")
	if code != http.StatusOK || !strings.Contains(body, `"State":"scheduled"`) {
		t.Fatalf("version-check want scheduled got %d body=%s", code, body)
	}

	// 3. 执行 → done；case 4001 pass, attempt=1
	code, body = doReq(t, h, "POST", "/runs/"+itoa(runID)+"/execute", 100, "")
	if code != http.StatusOK || !strings.Contains(body, `"State":"done"`) || !strings.Contains(body, `"AttemptSeq":1`) {
		t.Fatalf("execute want done/attempt1 got %d body=%s", code, body)
	}

	// 4. 重跑 → attempt=2
	code, body = doReq(t, h, "POST", "/runs/"+itoa(runID)+"/execute", 100, "")
	if code != http.StatusOK || !strings.Contains(body, `"AttemptSeq":2`) {
		t.Fatalf("re-execute want attempt2 got %d body=%s", code, body)
	}

	// 5. 明细：2 条（attempt 1,2）
	code, body = doReq(t, h, "GET", "/runs/"+itoa(runID)+"/case-results", 100, "")
	if code != http.StatusOK || !strings.Contains(body, `"AttemptSeq":2`) || !strings.Contains(body, "pass") {
		t.Fatalf("case-results want 200 with pass/attempt got %d body=%s", code, body)
	}

	// 6. RLS：team 200 查该 run 应失败（空）
	code, body = doReq(t, h, "GET", "/runs/"+itoa(runID), 200, "")
	if code != http.StatusNotFound {
		t.Fatalf("team200 get run want 404 got %d body=%s", code, body)
	}
}

// extractID 从 JSON body 提取 "ID":N。
func extractID(t *testing.T, body string) int64 {
	t.Helper()
	i := strings.Index(body, `"ID":`)
	if i < 0 {
		t.Fatalf("no ID in body=%s", body)
	}
	rest := body[i+5:]
	j := strings.IndexAny(rest, ",}")
	if j < 0 {
		t.Fatalf("bad ID in body=%s", body)
	}
	var n int64
	for _, ch := range rest[:j] {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int64(ch-'0')
	}
	return n
}
