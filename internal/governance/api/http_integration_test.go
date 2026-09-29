//go:build integration

// GP1-01 集成测试：被测对象树 + 仓库绑定（连真实 PG 5433 透传，RLS 跨租户隔离验证）。
// 注意：本环境沙箱对"进程内新监听端口"的连接做 TCP 隔离，故用 ResponseRecorder 直接调用
// handler（绕开网络层），DB 经 5433 透传真实查询。
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/gateway"
	"github.com/openware-io/open-green-pass/internal/governance/application"
	"github.com/openware-io/open-green-pass/internal/governance/infra"
	"github.com/openware-io/open-green-pass/pkg/id"
)

const testDSN = "postgres://gp:gp@127.0.0.1:5433/gp?sslmode=disable"

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatalf("id: %v", err)
	}
	db := infra.NewDB(pool)
	store := infra.NewTargetStore(db, gen)
	svc := application.NewTargetTreeService(store, gen)
	return gateway.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(mux *http.ServeMux) { Register(mux, svc) })
}

func doReq(t *testing.T, h http.Handler, method, path string, team int64, body string) (int, string) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-gp-team-id", itoa(team))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func itoa(n int64) string {
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

// TestTargetTree_CreateRepoList 建工程→绑仓库→列表，并验证 RLS 跨租户隔离。
func TestTargetTree_CreateRepoList(t *testing.T) {
	h := newTestHandler(t)

	// 1. team 100 建工程（随机名保证幂等，避免重复跑 unique 冲突）
	body := `{"name":"it-%d","type":"project","kind":"project"}`
	code, body := doReq(t, h, "POST", "/targets", 100, fmt.Sprintf(body, time.Now().UnixNano()))
	if code != http.StatusCreated {
		t.Fatalf("create project want 201 got %d body=%s", code, body)
	}
	var created struct {
		Node struct {
			ID int64 `json:"ID"`
		} `json:"Node"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || created.Node.ID == 0 {
		t.Fatalf("parse created: %v body=%s", err, body)
	}
	targetID := created.Node.ID

	// 2. team 100 绑仓库/分支/版本
	code, body = doReq(t, h, "POST", "/targets/"+itoa(targetID)+"/repo", 100,
		`{"url":"git@github.com:openware-io/im-saas.git","default_branch":"main","branch":"main","version":"v1.0.0"}`)
	if code != http.StatusOK {
		t.Fatalf("attach repo want 200 got %d body=%s", code, body)
	}

	// 3. team 100 列表应含工程
	code, body = doReq(t, h, "GET", "/targets?level=0", 100, "")
	if code != http.StatusOK {
		t.Fatalf("list want 200 got %d body=%s", code, body)
	}
	var list []map[string]any
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("parse list: %v body=%s", err, body)
	}
	if len(list) == 0 {
		t.Fatalf("list empty for team 100")
	}

	// 4. team 200 列表应空（RLS 跨租户隔离）
	code, body = doReq(t, h, "GET", "/targets?level=0", 200, "")
	if code != http.StatusOK {
		t.Fatalf("list team200 want 200 got %d body=%s", code, body)
	}
	var list2 []map[string]any
	if err := json.Unmarshal([]byte(body), &list2); err != nil {
		t.Fatalf("parse list2: %v body=%s", err, body)
	}
	if len(list2) != 0 {
		t.Fatalf("RLS leak: team 200 saw %d targets", len(list2))
	}
}
