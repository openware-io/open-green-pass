// Package config 提供应用配置加载：环境变量优先，带本地调试默认值。
// 端口契约与 GP0-03/.env 一致（宿主机避让 im-saas）。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 应用级配置。
type Config struct {
	Addr               string // HTTP 监听地址
	DBDSN              string // PostgreSQL DSN
	TrustedDBDSN       string // 独立可信写者 PostgreSQL DSN；生产环境必填
	RedisAddr          string
	TemporalAddr       string
	MinIOEndpoint      string // S3 兼容对象存储端点（本地 SeaweedFS）
	MinIOKey           string
	MinIOSecret        string
	CORSAllowedOrigins string // 逗号分隔的前端来源白名单
	Env                string // dev / test / prod
}

// Load 从环境变量读取配置（缺省用本地默认值）。
func Load() *Config {
	return &Config{
		Addr:               str("GP_ADDR", ":8080"),
		DBDSN:              str("GP_DB_DSN", "postgres://gp:gp@127.0.0.1:5433/gp?sslmode=disable"),
		TrustedDBDSN:       os.Getenv("GP_TRUSTED_DB_DSN"),
		RedisAddr:          str("GP_REDIS_ADDR", "127.0.0.1:6380"),
		TemporalAddr:       str("GP_TEMPORAL_ADDR", "127.0.0.1:7233"),
		MinIOEndpoint:      str("GP_MINIO_ENDPOINT", "127.0.0.1:9100"),
		MinIOKey:           str("GP_MINIO_KEY", "gpminio"),
		MinIOSecret:        str("GP_MINIO_SECRET", "gpminio-secret"),
		CORSAllowedOrigins: str("GP_CORS_ALLOWED_ORIGINS", "http://127.0.0.1:5173,http://localhost:5173"),
		Env:                str("GP_ENV", "dev"),
	}
}

// TrustedDSN returns the dedicated trusted-writer connection. Development
// may omit it for unit tests and local exploration; production must not share
// the primary application connection with trusted writes.
func (c *Config) TrustedDSN() (string, error) {
	if c.TrustedDBDSN != "" {
		if c.Env == "prod" && strings.TrimSpace(c.TrustedDBDSN) == strings.TrimSpace(c.DBDSN) {
			return "", fmt.Errorf("GP_TRUSTED_DB_DSN must use a dedicated production credential")
		}
		return c.TrustedDBDSN, nil
	}
	if c.Env == "prod" {
		return "", fmt.Errorf("GP_TRUSTED_DB_DSN is required in prod")
	}
	return c.DBDSN, nil
}

// MustInt 读取整数环境变量（缺省/非法用默认）。
func MustInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func str(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
