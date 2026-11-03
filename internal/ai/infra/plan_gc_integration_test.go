package infra

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformdb "github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/platform/metrics"
	"github.com/openware-io/open-green-pass/internal/platform/observability"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

// TestGenerationPlanSweepOnceWithRealDB verifies the GC visibility counters on a
// migrated scratch database: one expired draft plan is expired, the sweep reports
// tenant/expired/duration and no failures. Skipped unless GP_TEST_DB_DSN is set.
func TestGenerationPlanSweepOnceWithRealDB(t *testing.T) {
	dsn := os.Getenv("GP_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("GP_TEST_DB_DSN is not set; real plan GC sweep requires a migrated scratch database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	database := platformdb.NewDB(pool)
	base := time.Now().UTC().UnixNano() % 1_000_000_000_000
	teamID := 9_000_000_000_000 + base
	planID := 9_100_000_000_000 + base
	tenantCtx := rls.WithTenant(context.Background(), teamID)
	exec := func(statement string, args ...any) error {
		return database.WithTenant(tenantCtx, func(tx pgx.Tx) error {
			_, err := tx.Exec(tenantCtx, statement, args...)
			return err
		})
	}
	t.Cleanup(func() {
		_ = exec(`DELETE FROM gen_plan WHERE team_id=$1 AND id=$2`, teamID, planID)
		_ = exec(`DELETE FROM tnt_team WHERE team_id=$1 AND id=$2`, teamID, teamID)
	})
	if err := exec(`INSERT INTO tnt_team(id,team_id,name) VALUES ($1,$1,'gc-visibility-test')`, teamID); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if err := exec(`INSERT INTO gen_plan(id,team_id,project_id,scope_type,plan_hash,state,expires_at,created_by)
		VALUES ($1,$2,20,'project','sha256:gc-test','draft',now()-interval '1 hour',40)`, planID, teamID); err != nil {
		t.Fatalf("insert expired plan: %v", err)
	}
	store := NewGenerationPlanStore(database)
	recorder := observability.NewRecorder()
	store.SetMetricsRecorder(recorder)
	stats := store.SweepOnce(context.Background(), slog.Default())
	if stats.Tenants < 1 || stats.Expired < 1 {
		t.Fatalf("sweep stats = %+v, want at least one tenant/expired plan", stats)
	}
	if stats.FailedTenants != 0 || stats.Duration <= 0 {
		t.Fatalf("sweep stats = %+v, want no failures and positive duration", stats)
	}
	exposition := gcMetricExposition(t, recorder)
	for _, want := range []string{
		metrics.GenerationPlanGCSweeps + " 1",
		metrics.GenerationPlanGCDuration + "_count 1",
		metrics.GenerationPlanGCExpired,
	} {
		if !strings.Contains(exposition, want) {
			t.Fatalf("GC metrics missing %q:\n%s", want, exposition)
		}
	}
	if strings.Contains(exposition, "team_id") || strings.Contains(exposition, "tenant") {
		t.Fatalf("GC metrics must stay low-cardinality:\n%s", exposition)
	}
	var state string
	if err := database.WithTenant(tenantCtx, func(tx pgx.Tx) error {
		return tx.QueryRow(tenantCtx, `SELECT state FROM gen_plan WHERE team_id=$1 AND id=$2`, teamID, planID).Scan(&state)
	}); err != nil {
		t.Fatalf("read plan state: %v", err)
	}
	if state != "expired" {
		t.Fatalf("plan state = %q, want expired", state)
	}
}
