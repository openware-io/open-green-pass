// Package rls 提供多租户 Row-Level Security 的 Go 侧辅助：
// context 租户注入 + 生成在每个 DB 会话上设置 gp.team_id 的语句（RLS 生效前提）。
package rls

import "context"

type (
	tenantKey struct{}
	userKey   struct{}
)

// WithTenant 把 team_id 注入 context（供 gateway tenant 中间件使用）。
func WithTenant(ctx context.Context, teamID int64) context.Context {
	return context.WithValue(ctx, tenantKey{}, teamID)
}

// TenantFrom 从 context 读取 team_id；不存在返回 false。
func TenantFrom(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(tenantKey{}).(int64)
	return v, ok
}

// WithUser 把 user_id 注入 context（供 gateway auth 中间件/审计使用）。
func WithUser(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userKey{}, userID)
}

// UserFrom 从 context 读取 user_id；不存在返回 false。
func UserFrom(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(userKey{}).(int64)
	return v, ok
}

// SetTenantStmt 在单个 DB 会话上设置租户的预编译语句文本。
// 参数 1: team_id (int64)。同一连接被复用前必须先执行，RLS 才按该租户生效。
// 对应迁移 000001 中 gp.set_tenant() / policy 使用的 current_setting('gp.team_id')。
const SetTenantStmt = "SELECT set_config('gp.team_id', $1::text, false)"

// SetUserStmt 在单个 DB 会话上设置操作人（审计 user_id）的语句。参数 1: user_id (int64)。
const SetUserStmt = "SELECT set_config('gp.user_id', $1::text, false)"
