//go:build integration

// P0 集成测试骨架：验证 GP0-03 部署的真实依赖（PG/Redis/Temporal）经端口透传可达。
// 依赖未就绪时由 SkipIfUnavailable 跳过，CI 无集群环境不失败。
package setup

import (
	"net"
	"strings"
	"testing"
	"time"
)

// hostPort 从地址串提取 host:port（DB DSN 单独处理）。
func hostPort(addr string) string {
	addr = strings.TrimPrefix(addr, "postgres://")
	if i := strings.Index(addr, "/"); i >= 0 {
		addr = addr[:i]
	}
	if i := strings.LastIndex(addr, "@"); i >= 0 {
		addr = addr[i+1:]
	}
	return addr
}

// TestDepsReachable 探活真实依赖 TCP 端口。
func TestDepsReachable(t *testing.T) {
	d := LoadDeps()
	SkipIfUnavailable(t, d)

	targets := map[string]string{
		"postgres": hostPort(d.DBDSN),
		"redis":    d.RedisAddr,
		"temporal": d.TemporalAddr,
	}
	for name, addr := range targets {
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			t.Fatalf("%s 依赖不可达 %s: %v", name, addr, err)
		}
		conn.Close()
		t.Logf("%s 可达: %s", name, addr)
	}
}
