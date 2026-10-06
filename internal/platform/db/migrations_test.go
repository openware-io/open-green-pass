package db_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// GP3-07A 的迁移契约测试：不依赖数据库凭据，防止权限边界在后续迁移中被遗漏。
// 真实角色/RLS 行为仍由带 testcontainer PostgreSQL 的集成门禁验收。
func TestTrustedPermissionsMigrationContract(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller unavailable")
	}
	migDir := filepath.Join(filepath.Dir(file), "../../../migrations")
	up := readMigration(t, filepath.Join(migDir, "000013_trusted_permissions.up.sql"))
	down := readMigration(t, filepath.Join(migDir, "000013_trusted_permissions.down.sql"))

	for _, role := range []string{"gp_trusted_writer", "gp_server", "gp_worker", "gp_verifier", "gp_report_reader"} {
		if !strings.Contains(up, "rolname = '"+role+"'") {
			t.Errorf("up migration does not provision role %s", role)
		}
	}
	if strings.Contains(strings.ToUpper(up), "PASSWORD") {
		t.Fatal("trusted permissions migration must never contain password material")
	}

	for _, table := range []string{"aud_event", "cost_line_item", "gate_result"} {
		if !strings.Contains(up, "BEFORE UPDATE OR DELETE ON "+table) {
			t.Errorf("%s missing append-only trigger", table)
		}
		if !strings.Contains(up, "ALTER TABLE "+table+" FORCE ROW LEVEL SECURITY") {
			t.Errorf("%s missing FORCE ROW LEVEL SECURITY", table)
		}
	}
	if !strings.Contains(up, "GRANT SELECT, INSERT ON TABLE aud_event, cost_line_item, gate_result") {
		t.Error("trusted writer grant is not append-only minimal grant")
	}
	if !strings.Contains(up, "GRANT SELECT ON TABLE aud_event, cost_line_item, gate_result, gate_rule") {
		t.Error("reader grant is missing")
	}
	if !strings.Contains(up, "REVOKE ALL ON TABLE aud_event, cost_line_item, gate_result, gate_rule") {
		t.Error("default/table role revoke is missing")
	}

	for _, trigger := range []string{"trg_aud_event_append_only", "trg_cost_line_item_append_only", "trg_gate_result_append_only"} {
		if !strings.Contains(down, "DROP TRIGGER IF EXISTS "+trigger) {
			t.Errorf("down migration does not drop %s", trigger)
		}
	}
}

func readMigration(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", path, err)
	}
	return string(b)
}
