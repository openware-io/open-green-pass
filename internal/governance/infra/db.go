// Package infra 治理域基础设施：数据库访问基座（RLS 租户注入）复用 platform/db + 被测对象树/用例仓储。
// 依赖 pgx 连接池；RLS 采用事务级 set_config（事务结束自动失效，连接归还无租户污染）。
package infra

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/platform/db"
)

// DB 治理域数据库访问基座（别名至 platform/db；WithTenant 语义见 platform/db）。
type DB = db.DB

// NewDB 创建 DB 基座。
func NewDB(pool *pgxpool.Pool) *DB { return db.NewDB(pool) }
