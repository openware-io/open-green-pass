//go:build integration

// GP1-03 集成测试：测试环境版本校验（登记运行版本 / 比对 match-mismatch / 历史 + RLS 隔离）。
package api

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/execution/application"
	einfra "github.com/openware-io/open-green-pass/internal/execution/infra"
	"github.com/openware-io/open-green-pass/internal/gateway"
	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/pkg/id"
)

const testDSN = "postgres://gp:gp@127.0.0.1:5433/gp?sslmode=disable"

func newEnvHandler(t *testing.T) http.Handler {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDSN)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	gen, _ := id.New(1, nil)
	dbb := db.NewDB(pool)
	store := einfra.NewEnvStore(dbb, gen)
	svc := application.NewEnvService(store, gen)
	return gateway.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(mux *http.ServeMux) { Register(mux, svc, nil) })
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

// TestEnvVersionCheck 版本校验闭环：登记→match→mismatch→历史，RLS 隔离。
func TestEnvVersionCheck(t *testing.T) {
	h := newEnvHandler(t)
	const targetID = 1003 // im-saas-gateway

	// 1. 登记环境运行版本（env=test, v1.0.0）
	code, body := doReq(t, h, "POST", "/targets/"+itoa(targetID)+"/env/runtime", 100,
		`{"env":"test","version":"v1.0.0"}`)
	if code != http.StatusOK || !strings.Contains(body, `"RunningVersion":"v1.0.0"`) {
		t.Fatalf("register runtime want 200 got %d body=%s", code, body)
	}

	// 2. 目标 v1.0.0 == 环境 v1.0.0 → match
	code, body = doReq(t, h, "POST", "/targets/"+itoa(targetID)+"/version-check", 100,
		`{"env":"test","target_version":"v1.0.0"}`)
	if code != http.StatusOK || !strings.Contains(body, `"Result":"match"`) {
		t.Fatalf("check match want 200/match got %d body=%s", code, body)
	}

	// 3. 目标 v1.0.1 != 环境 v1.0.0 → mismatch
	code, body = doReq(t, h, "POST", "/targets/"+itoa(targetID)+"/version-check", 100,
		`{"env":"test","target_version":"v1.0.1"}`)
	if code != http.StatusOK || !strings.Contains(body, `"Result":"mismatch"`) {
		t.Fatalf("check mismatch want 200/mismatch got %d body=%s", code, body)
	}

	// 4. 历史应含 match + mismatch
	code, body = doReq(t, h, "GET", "/targets/"+itoa(targetID)+"/version-checks", 100, "")
	if code != http.StatusOK || !strings.Contains(body, "match") || !strings.Contains(body, "mismatch") {
		t.Fatalf("recent checks want 200 with match/mismatch got %d body=%s", code, body)
	}

	// 5. RLS：team 200 查不到校验记录（空数组）
	code, body = doReq(t, h, "GET", "/targets/"+itoa(targetID)+"/version-checks", 200, "")
	if code != http.StatusOK {
		t.Fatalf("team200 checks want 200 got %d body=%s", code, body)
	}
	if strings.Contains(body, "match") {
		t.Fatalf("RLS leak: team 200 saw env checks, body=%s", body)
	}
}
