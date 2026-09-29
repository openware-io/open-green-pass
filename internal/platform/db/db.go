// Package db 平台级数据库访问基座：pgx 连接池 + RLS 租户/操作人注入。
// 供各业务域 infra 复用（governance/execution/trusted 等），RLS 采用事务级 set_config（事务结束自动失效，连接归还无租户污染）。
package db

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

// DB 统一数据库访问基座：连接池 + RLS 租户/操作人注入。
type DB struct {
	pool *pgxpool.Pool
}

// NewDB 创建 DB 基座。
func NewDB(pool *pgxpool.Pool) *DB { return &DB{pool: pool} }

// Pool 暴露连接池（供需直接连接的操作）。
func (d *DB) Pool() *pgxpool.Pool { return d.pool }

// WithTenant 在事务内注入租户/操作人并执行 fn（RLS 生效；事务提交后作用域失效）。
// 读操作也走事务：RLS policy 依赖 current_setting，需在同一事务内设置才能按租户过滤。
func (d *DB) WithTenant(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if teamID, ok := rls.TenantFrom(ctx); ok {
		if _, err := tx.Exec(ctx, rls.SetTenantTxStmt, strconv.FormatInt(teamID, 10)); err != nil {
			return err
		}
	}
	if userID, ok := rls.UserFrom(ctx); ok {
		if _, err := tx.Exec(ctx, rls.SetUserTxStmt, strconv.FormatInt(userID, 10)); err != nil {
			return err
		}
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
