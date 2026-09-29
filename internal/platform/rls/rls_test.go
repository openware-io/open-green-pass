package rls

import "testing"

func TestWithTenantRoundtrip(t *testing.T) {
	ctx := WithTenant(t.Context(), 42)
	id, ok := TenantFrom(ctx)
	if !ok {
		t.Fatal("TenantFrom 应返回 ok=true")
	}
	if id != 42 {
		t.Fatalf("TenantFrom 返回 %d, want 42", id)
	}
}

func TestTenantFromEmpty(t *testing.T) {
	if _, ok := TenantFrom(t.Context()); ok {
		t.Fatal("空 context 不应返回租户")
	}
}

func TestStmts(t *testing.T) {
	if SetTenantStmt == "" {
		t.Fatal("SetTenantStmt 不能为空")
	}
	if SetUserStmt == "" {
		t.Fatal("SetUserStmt 不能为空")
	}
}
