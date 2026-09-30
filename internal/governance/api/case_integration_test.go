//go:build integration

// GP1-02 集成测试：用例版本化（新增/更新/回退/历史 + RLS 隔离）。
// 连真实 PG 5433 透传；沙箱对进程内监听端口连接做 TCP 隔离，故用 ResponseRecorder 直调 handler。
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/gateway"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/infra"
	"github.com/openware-io/open-green-pass/pkg/id"
)

func newCaseHandler(t *testing.T) http.Handler {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDSN)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	gen, _ := id.New(1, nil)
	db := infra.NewDB(pool)
	caseStore := infra.NewCaseStore(db, gen)
	caseSvc := application.NewCaseService(caseStore, gen)
	return gateway.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(mux *http.ServeMux) { RegisterCases(mux, caseSvc) })
}

// TestCaseVersioning 用例版本化闭环：新增→更新→回退→历史，RLS 跨租户隔离。
func TestCaseVersioning(t *testing.T) {
	h := newCaseHandler(t)

	// 1. team 100 新建用例（归属 im-saas-gateway 节点 1003；随机 code 保证幂等）
	code, body := doReq(t, h, "POST", "/cases", 100,
		fmt.Sprintf(`{"target_id":1003,"code":"it-%d","title":"登录接口","kind":"api","script":{"method":"GET","path":"/login"}}`, time.Now().UnixNano()))
	if code != http.StatusCreated {
		t.Fatalf("create case want 201 got %d body=%s", code, body)
	}
	var created struct {
		ID             int64 `json:"id"`
		CurrentVersion int   `json:"current_version"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || created.ID == 0 || created.CurrentVersion != 1 {
		t.Fatalf("parse created: %v body=%s", err, body)
	}
	cid := created.ID

	// 2. 新增版本 v2（内容更新，来源 repo 2001/main）
	code, body = doReq(t, h, "POST", "/cases/"+itoa(cid)+"/versions", 100,
		`{"script":{"method":"POST","path":"/login","body":{}},"source_repo_id":2001,"source_branch":"main"}`)
	if code != http.StatusOK {
		t.Fatalf("add version want 200 got %d body=%s", code, body)
	}
	if !strings.Contains(body, `"current_version":2`) {
		t.Fatalf("after update current_version should be 2, body=%s", body)
	}

	// 3. 回退到 v1（current_version=1，新增 rollback 版本）
	code, body = doReq(t, h, "POST", "/cases/"+itoa(cid)+"/rollback", 100, `{"version":1}`)
	if code != http.StatusOK {
		t.Fatalf("rollback want 200 got %d body=%s", code, body)
	}
	if !strings.Contains(body, `"current_version":1`) {
		t.Fatalf("after rollback current_version should be 1, body=%s", body)
	}

	// 4. 历史应为 3 条（added/updated/rollback）
	code, body = doReq(t, h, "GET", "/cases/"+itoa(cid)+"/history", 100, "")
	if code != http.StatusOK {
		t.Fatalf("history want 200 got %d body=%s", code, body)
	}
	if !strings.Contains(body, `"ChangeType":"added"`) ||
		!strings.Contains(body, `"ChangeType":"updated"`) ||
		!strings.Contains(body, `"ChangeType":"rollback"`) {
		t.Fatalf("history missing change types, body=%s", body)
	}

	// 5. 列表按被测对象筛选应含该用例
	code, body = doReq(t, h, "GET", "/cases?target_id=1003", 100, "")
	if code != http.StatusOK {
		t.Fatalf("list by target want 200 got %d body=%s", code, body)
	}

	// 6. team 200 列表应空（RLS 隔离）
	code, body = doReq(t, h, "GET", "/cases", 200, "")
	if code != http.StatusOK {
		t.Fatalf("list team200 want 200 got %d body=%s", code, body)
	}
	if strings.Contains(body, "it-") {
		t.Fatalf("RLS leak: team 200 saw team 100 cases, body=%s", body)
	}
}
