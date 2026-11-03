package application

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	aiinfra "github.com/openware-io/open-green-pass/internal/ai/infra"
	platformdb "github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	trustedinfra "github.com/openware-io/open-green-pass/internal/trusted/infra"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// TestGenerationRunCostUsesBatchID verifies the migration-000065 contract on a
// migrated PostgreSQL scratch database:
//   - the DSH cost adapter writes cost_line_item.batch_id for new charges;
//   - run cost reads exact batch_id matches AND keeps the request-id fallback for
//     historical rows that predate the column.
//
// It is skipped unless GP_TEST_DB_DSN points at a scratch database with all
// migrations applied (same convention as the plan-cost integration test).
//
//nolint:gocyclo // Integration fixture covers new batch_id plus legacy fallback rows.
func TestGenerationRunCostUsesBatchID(t *testing.T) {
	dsn := os.Getenv("GP_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("GP_TEST_DB_DSN is not set; real batch-cost attribution requires a migrated scratch database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	database := platformdb.NewDB(pool)
	gen, err := id.New(6, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().UnixNano() % 1_000_000_000_000
	teamID := 8_000_000_000_000 + base
	projectID := 8_100_000_000_000 + base
	repoID := 8_200_000_000_000 + base
	serviceID := 8_300_000_000_000 + base
	batchID := 8_400_000_000_000 + base
	runID := 8_500_000_000_000 + base
	unitID := 8_600_000_000_000 + base
	priceID := 8_700_000_000_000 + base
	legacyCostID := 8_800_000_000_000 + base
	tenantCtx := rls.WithTenant(context.Background(), teamID)

	exec := func(sql string, args ...any) error {
		return database.WithTenant(tenantCtx, func(tx pgx.Tx) error {
			_, err := tx.Exec(tenantCtx, sql, args...)
			return err
		})
	}
	cleanup := func() {
		_ = exec(`DELETE FROM gen_run_unit WHERE team_id=$1 AND run_id=$2`, teamID, runID)
		_ = exec(`DELETE FROM gen_run WHERE team_id=$1 AND id=$2`, teamID, runID)
		_ = exec(`DELETE FROM gen_batch WHERE team_id=$1 AND id=$2`, teamID, batchID)
		_ = exec(`DELETE FROM cost_price WHERE team_id=$1 AND id=$2`, teamID, priceID)
		_ = exec(`DELETE FROM gen_run_unit WHERE team_id=$1 AND id=$2`, teamID, unitID)
	}
	t.Cleanup(cleanup)

	if err := exec(`INSERT INTO gen_run(id,team_id,project_id,scope_type,scope_id,state,total_units,completed_units,failed_units,waiting_review_units,idempotency_key,created_by)
		VALUES ($1,$2,$3,'project',0,'running',1,0,0,0,'cost-batch-run',40)`, runID, teamID, projectID); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := exec(`INSERT INTO gen_batch(id,team_id,target_id,repo_id,service_id,source_path,ref_type,branch,state,state_version,idempotency_key,created_by,tokens_in,tokens_out)
		VALUES ($1,$2,$3,$4,NULL,'services/svc','branch','main','approved',1,'cost-batch-batch',40,1000,2000)`,
		batchID, teamID, projectID, repoID); err != nil {
		t.Fatalf("insert batch: %v", err)
	}
	if err := exec(`INSERT INTO gen_run_unit(id,team_id,run_id,service_id,repo_id,batch_id,state,batch_state,stage)
		VALUES ($1,$2,$3,$4,$5,$6,'succeeded','approved','approved')`, unitID, teamID, runID, serviceID, repoID, batchID); err != nil {
		t.Fatalf("insert run unit: %v", err)
	}
	if err := exec(`INSERT INTO cost_price(id,team_id,model_key,input_per_1k,output_per_1k,cache_read_per_1k,currency,effective_from,status,created_by)
		VALUES ($1,$2,'dsh/test-model',0.01,0.02,0,'CNY',now()-interval '1 hour','active',40)`, priceID, teamID); err != nil {
		t.Fatalf("insert cost price: %v", err)
	}

	costStore := trustedinfra.NewCostStore(database, gen)
	auditSvc := trustedapp.NewAuditService(trustedinfra.NewAuditStore(database, gen), gen)
	costSvc := trustedapp.NewCostService(costStore, auditSvc, gen)
	priceSvc := trustedapp.NewCostPriceService(trustedinfra.NewCostPriceStore(database), auditSvc, gen)
	adapter := aiinfra.NewGenerationCostAdapter(costSvc, priceSvc)
	requestID := fmt.Sprintf("gen-%d-1", batchID)
	if err := adapter.RecordGenerationCost(tenantCtx, domain.GenerationCost{
		RequestID: requestID, TeamID: teamID, TargetID: projectID, BatchID: batchID,
		Model: "dsh/test-model", TokensIn: 1000, TokensOut: 2000,
	}); err != nil {
		t.Fatalf("record cost: %v", err)
	}
	var storedBatchID *int64
	if err := database.WithTenant(tenantCtx, func(tx pgx.Tx) error {
		return tx.QueryRow(tenantCtx, `SELECT batch_id FROM cost_line_item WHERE team_id=$1 AND request_id=$2`, teamID, requestID).Scan(&storedBatchID)
	}); err != nil {
		t.Fatalf("read stored cost: %v", err)
	}
	if storedBatchID == nil || *storedBatchID != batchID {
		t.Fatalf("cost_line_item.batch_id = %v, want %d", storedBatchID, batchID)
	}

	// Historical NULL row: COST-CLEANUP removed these; the trusted read must not
	// use a request_id LIKE fallback, so this row must be excluded from run cost.
	if err := exec(`INSERT INTO cost_line_item(id,request_id,idempotency_key,team_id,target_id,run_id,biz_category,biz_point,model,tokens_in,tokens_out,amount,occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,'generate','case_generate','dsh/test-model',0,0,0.02,now())`,
		legacyCostID, fmt.Sprintf("gen-%d-legacy", batchID), fmt.Sprintf("legacy-%d", batchID), teamID, projectID, runID); err != nil {
		t.Fatalf("insert legacy cost: %v", err)
	}

	runStore := aiinfra.NewGenerationRunStore(database)
	runStore.SetCostReadPort(aiinfra.NewGenerationCostReadAdapter(costSvc))
	run, err := runStore.Find(tenantCtx, teamID, runID)
	if err != nil {
		t.Fatalf("find run: %v", err)
	}
	// Exact batch_id matches only: 0.01*1 + 0.02*2 = 0.05; the NULL legacy row
	// (0.02) is intentionally excluded.
	if run.CostAmount < 0.049 || run.CostAmount > 0.051 {
		t.Fatalf("run cost = %v, want ≈0.05 (exact batch_id only; NULL legacy row excluded)", run.CostAmount)
	}
	if run.TokensIn != 1000 || run.TokensOut != 2000 {
		t.Fatalf("run tokens = %d/%d", run.TokensIn, run.TokensOut)
	}
}
