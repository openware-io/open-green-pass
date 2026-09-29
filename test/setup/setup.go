//go:build integration

// Package setup 提供集成测试基座：从环境读取真实依赖地址（对应 GP0-03/.env 端口契约），
// 依赖未就绪时跳过，避免 CI 环境无集群时集成测试失败。
package setup

import (
	"os"
	"testing"
)

// Deps 集成测试依赖地址。
type Deps struct {
	DBDSN         string
	RedisAddr     string
	TemporalAddr  string
	MinIOEndpoint string
}

// LoadDeps 从 env 读取依赖地址，缺省用本地调试默认值。
func LoadDeps() *Deps {
	return &Deps{
		DBDSN:         str("GP_DB_DSN", "postgres://gp:gp@127.0.0.1:5433/gp?sslmode=disable"),
		RedisAddr:     str("GP_REDIS_ADDR", "127.0.0.1:6380"),
		TemporalAddr:  str("GP_TEMPORAL_ADDR", "127.0.0.1:7233"),
		MinIOEndpoint: str("GP_MINIO_ENDPOINT", "127.0.0.1:9100"),
	}
}

// SkipIfUnavailable 若显式跳过或关键依赖未配置则 t.Skip。
// 约定：环境变量 GP_SKIP_INTEGRATION=1 时跳过；或 DB DSN 为空时跳过。
func SkipIfUnavailable(t *testing.T, d *Deps) {
	t.Helper()
	if os.Getenv("GP_SKIP_INTEGRATION") == "1" {
		t.Skip("GP_SKIP_INTEGRATION=1，跳过集成测试")
	}
	if d == nil || d.DBDSN == "" {
		t.Skip("DB DSN 未配置，跳过集成测试")
	}
}

func str(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
